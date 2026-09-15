package api

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/apikey"
	"pikpak-manager/internal/audit"
	"pikpak-manager/internal/auth"
	"pikpak-manager/internal/dashboard"
	"pikpak-manager/internal/fileagg"
	"pikpak-manager/internal/offline"
	"pikpak-manager/internal/settings"
	"pikpak-manager/internal/stream"
)

type Server struct {
	Engine          *gin.Engine
	AccountService  *account.Service
	FileService     *fileagg.Service
	OfflineService  *offline.Service
	AuthService     *auth.Service
	ApiKeyService   *apikey.Service
	AuditService    *audit.Service
	DashService     *dashboard.Service
	SettingsService *settings.Service
	StreamHandler   *stream.Handler
	FrontendFS      *embed.FS
}

func NewServer(
	accSvc *account.Service,
	fileSvc *fileagg.Service,
	offSvc *offline.Service,
	authSvc *auth.Service,
	apiKeySvc *apikey.Service,
	auditSvc *audit.Service,
	dashSvc *dashboard.Service,
	frontendFS *embed.FS,
	optSettings ...*settings.Service,
) *Server {
	engine := gin.Default()

	var setSvc *settings.Service
	if len(optSettings) > 0 {
		setSvc = optSettings[0]
	}

	s := &Server{
		Engine:          engine,
		AccountService:  accSvc,
		FileService:     fileSvc,
		OfflineService:  offSvc,
		AuthService:     authSvc,
		ApiKeyService:   apiKeySvc,
		AuditService:    auditSvc,
		DashService:     dashSvc,
		SettingsService: setSvc,
		StreamHandler:   stream.NewHandler(fileSvc),
		FrontendFS:      frontendFS,
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := s.Engine

	// 1. Healthcheck for Docker
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "system": "pikpak-manager"})
	})

	// 2. Auth routes
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/login", s.AuthService.HandleLogin)
		authGroup.POST("/logout", s.AuthService.HandleLogout)

		protectedAuth := authGroup.Group("")
		protectedAuth.Use(s.AuthService.AuthMiddleware())
		{
			protectedAuth.GET("/me", s.AuthService.HandleMe)
			protectedAuth.POST("/change-password", s.AuthService.HandleChangePassword)
			protectedAuth.POST("/profile", s.AuthService.HandleUpdateProfile)
		}
	}

	// 3. Authenticated API routes
	api := r.Group("/api")
	api.Use(s.AuthService.AuthMiddleware())
	{
		// Admin-only management endpoints
		admin := api.Group("")
		admin.Use(s.AuthService.RequireAdmin())
		{
			// Accounts
			admin.GET("/accounts", s.handleListAccounts)
			admin.POST("/accounts", s.handleCreateAccount)
			admin.PUT("/accounts/:id", s.handleUpdateAccount)
			admin.DELETE("/accounts/:id", s.handleDeleteAccount)
			admin.POST("/accounts/:id/test", s.handleTestAccount)
			admin.POST("/accounts/:id/reset-quota", s.handleResetQuota)
			admin.POST("/accounts/:id/initialize", s.handleInitializeAccount)
			admin.POST("/accounts/test-proxy", s.handleTestProxy)

			// User Management
			admin.GET("/users", s.AuthService.HandleListUsers)
			admin.POST("/users", s.AuthService.HandleCreateUser)
			admin.DELETE("/users/:id", s.AuthService.HandleDeleteUser)
			admin.POST("/users/:id/reset-password", s.AuthService.HandleAdminResetPassword)

			// API Keys
			admin.GET("/apikeys", s.handleListApiKeys)
			admin.POST("/apikeys", s.handleCreateApiKey)
			admin.DELETE("/apikeys/:id", s.handleDeleteApiKey)
			admin.POST("/apikeys/:id/toggle", s.handleToggleApiKey)

			// Audit Logs
			admin.GET("/audit/logs", s.handleListAuditLogs)

			// Dashboard
			admin.GET("/dashboard/stats", s.handleGetDashboardStats)

			// Settings & Automation Policies
			admin.GET("/settings", s.handleGetSettings)
			admin.POST("/settings", s.handleUpdateSettings)
			admin.POST("/settings/cleanup/trigger", s.handleTriggerCleanup)
		}

		// Operations accessible to all authenticated users (isolated per user)
		// Files
		api.GET("/files", s.handleListFiles)
		api.GET("/files/search", s.handleSearchFiles)
		api.POST("/files/batch-delete", s.handleBatchDelete)
		api.POST("/files/rename", s.handleRenameFile)

		// Media / Video Stream
		api.GET("/media/info/:virtual_id", s.StreamHandler.GetPlaybackInfo)
		api.GET("/media/stream/:virtual_id", s.StreamHandler.ProxyStream)

		// Offline Tasks
		api.GET("/offline/tasks", s.handleListOfflineTasks)
		api.GET("/offline/tasks/:id", s.handleGetOfflineTask)
		api.POST("/offline/tasks", s.handleSubmitOfflineTask)
		api.DELETE("/offline/tasks/:id", s.handleDeleteOfflineTask)
		api.POST("/offline/tasks/:id/cancel", s.handleCancelOfflineTask)
		api.POST("/offline/tasks/:id/retry", s.handleRetryOfflineTask)
	}

	// 4. Public REST API v1 (Authenticated via Bearer API Key)
	v1 := r.Group("/api/v1")
	{
		// Offline submission with Idempotency-Key
		v1.POST("/offline", s.ApiKeyService.KeyAuthMiddleware("offline:create"), s.handleV1SubmitOffline)
		v1.GET("/offline/:id", s.ApiKeyService.KeyAuthMiddleware("offline:read"), s.handleV1GetOffline)
		v1.GET("/offline", s.ApiKeyService.KeyAuthMiddleware("offline:read"), s.handleV1ListOffline)
		v1.GET("/accounts/status", s.ApiKeyService.KeyAuthMiddleware("offline:read"), s.handleV1AccountsStatus)
	}

	// 5. Frontend Static Files Serving (Embedded SPA)
	if s.FrontendFS != nil {
		distFS, err := fs.Sub(s.FrontendFS, "dist")
		if err == nil {
			fileServer := http.FileServer(http.FS(distFS))
			r.NoRoute(func(c *gin.Context) {
				path := c.Request.URL.Path
				if strings.HasPrefix(path, "/api/") {
					c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
					return
				}

				f, err := distFS.Open(strings.TrimPrefix(path, "/"))
				if err == nil {
					_ = f.Close()
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}

				// Fallback to index.html for SPA routing
				indexContent, err := fs.ReadFile(distFS, "index.html")
				if err == nil {
					c.Data(http.StatusOK, "text/html; charset=utf-8", indexContent)
					return
				}
				c.String(http.StatusNotFound, "frontend assets not found")
			})
		}
	}
}

// Account Handlers
func (s *Server) handleListAccounts(c *gin.Context) {
	list, err := s.AccountService.ListAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (s *Server) handleCreateAccount(c *gin.Context) {
	var req account.CreateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acc, err := s.AccountService.CreateAccount(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("CREATE_ACCOUNT", acc.Name, "Account added with proxy: "+acc.ProxyURL, "SUCCESS", username.(string))
	c.JSON(http.StatusOK, acc)
}

func (s *Server) handleUpdateAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req account.UpdateAccountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acc, err := s.AccountService.UpdateAccount(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("UPDATE_ACCOUNT", acc.Name, "Account updated", "SUCCESS", username.(string))
	c.JSON(http.StatusOK, acc)
}

func (s *Server) handleDeleteAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	err := s.AccountService.DeleteAccount(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("DELETE_ACCOUNT", strconv.FormatInt(id, 10), "Account deleted", "SUCCESS", username.(string))
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleTestAccount(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	res, err := s.AccountService.TestAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (s *Server) handleResetQuota(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	err := s.AccountService.ResetQuota(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("RESET_QUOTA", strconv.FormatInt(id, 10), "Quota status reset to HEALTHY", "SUCCESS", username.(string))
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleInitializeAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account id"})
		return
	}

	err = s.AccountService.InitializeAccount(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("INITIALIZE_ACCOUNT", strconv.FormatInt(id, 10), "Account initialized: remote files and offline tasks cleared", "SUCCESS", username.(string))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "account initialized successfully"})
}

func (s *Server) handleTestProxy(c *gin.Context) {
	var req struct {
		ProxyURL string `json:"proxy_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := s.AccountService.TestProxy(c.Request.Context(), req.ProxyURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func getUserContext(c *gin.Context) (int64, string, string) {
	var uid int64 = 1
	var username string = "admin"
	var role string = "admin"

	if v, exists := c.Get("user_id"); exists {
		switch id := v.(type) {
		case int64:
			uid = id
		case string:
			uid, _ = strconv.ParseInt(id, 10, 64)
		}
	}
	if v, exists := c.Get("username"); exists {
		if u, ok := v.(string); ok && u != "" {
			username = u
		}
	}
	if v, exists := c.Get("role"); exists {
		if r, ok := v.(string); ok && r != "" {
			role = r
		}
	}

	return uid, username, role
}

// File Handlers
func (s *Server) handleListFiles(c *gin.Context) {
	parentID := c.Query("parent_id")
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")

	uid, username, role := getUserContext(c)
	files, err := s.FileService.ListFiles(c.Request.Context(), parentID, uid, username, role, sortBy, sortOrder)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func (s *Server) handleSearchFiles(c *gin.Context) {
	keyword := c.Query("keyword")
	accID, _ := strconv.ParseInt(c.Query("account_id"), 10, 64)

	uid, _, _ := getUserContext(c)
	files, err := s.FileService.SearchFiles(c.Request.Context(), keyword, uid, accID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func (s *Server) handleBatchDelete(c *gin.Context) {
	var req fileagg.BatchDeleteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, username, role := getUserContext(c)
	res, err := s.FileService.BatchDelete(c.Request.Context(), req, uid, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.AuditService.Record("BATCH_DELETE", strconv.Itoa(len(req.VirtualIDs)), fmt.Sprintf("Deleted %d files (failed %d)", res.Success, res.Failed), "SUCCESS", username)
	c.JSON(http.StatusOK, res)
}

func (s *Server) handleRenameFile(c *gin.Context) {
	var req struct {
		VirtualID string `json:"virtual_id" binding:"required"`
		Name      string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, username, role := getUserContext(c)
	if err := s.FileService.RenameFile(c.Request.Context(), req.VirtualID, req.Name, uid, role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	s.AuditService.Record("RENAME_FILE", req.VirtualID, "Renamed file to: "+req.Name, "SUCCESS", username)
	c.JSON(http.StatusOK, gin.H{"success": true, "name": req.Name})
}

// Offline Task Handlers
func (s *Server) handleListOfflineTasks(c *gin.Context) {
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	uid, _, _ := getUserContext(c)
	tasks, total, err := s.OfflineService.ListTasks(c.Request.Context(), status, uid, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":  tasks,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (s *Server) handleGetOfflineTask(c *gin.Context) {
	id := c.Param("id")
	task, err := s.OfflineService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) handleSubmitOfflineTask(c *gin.Context) {
	var req offline.SubmitTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, username, _ := getUserContext(c)
	idempotencyKey := c.GetHeader("Idempotency-Key")
	res, err := s.OfflineService.SubmitBatchLinks(c.Request.Context(), req, idempotencyKey, uid, username)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (s *Server) handleDeleteOfflineTask(c *gin.Context) {
	id := c.Param("id")
	if err := s.OfflineService.DeleteTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleCancelOfflineTask(c *gin.Context) {
	id := c.Param("id")
	if err := s.OfflineService.CancelTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleRetryOfflineTask(c *gin.Context) {
	id := c.Param("id")
	uid, username, _ := getUserContext(c)
	res, err := s.OfflineService.RetryTask(c.Request.Context(), id, uid, username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

// API Key Handlers
func (s *Server) handleListApiKeys(c *gin.Context) {
	keys, err := s.ApiKeyService.ListKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (s *Server) handleCreateApiKey(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Permissions string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	k, err := s.ApiKeyService.GenerateAPIKey(req.Name, req.Permissions)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	username, _ := c.Get("username")
	s.AuditService.Record("CREATE_API_KEY", k.Name, "Created API key prefix "+k.KeyPrefix, "SUCCESS", username.(string))
	c.JSON(http.StatusOK, k)
}

func (s *Server) handleDeleteApiKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := s.ApiKeyService.DeleteKey(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleToggleApiKey(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.ApiKeyService.ToggleKey(id, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Audit & Dashboard
func (s *Server) handleListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := s.AuditService.ListLogs(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"logs": logs, "total": total})
}

func (s *Server) handleGetDashboardStats(c *gin.Context) {
	stats, err := s.DashService.GetStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// Public v1 REST API Handlers
func (s *Server) handleV1SubmitOffline(c *gin.Context) {
	var req offline.SubmitTaskReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	reqBytes := req.FileSize
	if reqBytes == 0 && req.FileSizeGB > 0 {
		reqBytes = int64(req.FileSizeGB * 1024 * 1024 * 1024)
	}

	// If single URL provided
	if req.URL != "" && len(req.URLs) == 0 && !strings.Contains(req.URL, "\n") {
		res, err := s.OfflineService.SubmitSingleLink(c.Request.Context(), req.URL, req.Name, idempotencyKey, 1, "admin", reqBytes)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"task_id": res.ID,
			"status":  res.Status,
			"account": res.Account,
		})
		return
	}

	// Batch submission
	batchResp, err := s.OfflineService.SubmitBatchLinks(c.Request.Context(), req, idempotencyKey, 1, "admin")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, batchResp)
}

func (s *Server) handleV1GetOffline(c *gin.Context) {
	id := c.Param("id")
	task, err := s.OfflineService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) handleV1ListOffline(c *gin.Context) {
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	tasks, total, err := s.OfflineService.ListTasks(c.Request.Context(), status, 0, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tasks": tasks, "total": total})
}

func (s *Server) handleV1AccountsStatus(c *gin.Context) {
	accounts, err := s.AccountService.ListAccounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type AccountStatusItem struct {
		ID             int64  `json:"id"`
		Name           string `json:"name"`
		Status         string `json:"status"`
		Priority       int    `json:"priority"`
		IsEnabled      bool   `json:"is_enabled"`
		DailyTaskCount int    `json:"daily_task_count"`
		UsedSpace      int64  `json:"used_space"`
		TotalSpace     int64  `json:"total_space"`
	}

	var res []AccountStatusItem
	for _, a := range accounts {
		res = append(res, AccountStatusItem{
			ID:             a.ID,
			Name:           a.Name,
			Status:         a.Status,
			Priority:       a.Priority,
			IsEnabled:      a.IsEnabled,
			DailyTaskCount: a.DailyTaskCount,
			UsedSpace:      a.UsedSpace,
			TotalSpace:     a.TotalSpace,
		})
	}

	c.JSON(http.StatusOK, res)
}

// Settings & Automation Policies Handlers
func (s *Server) handleGetSettings(c *gin.Context) {
	if s.SettingsService == nil {
		c.JSON(http.StatusOK, gin.H{
			"storage_balancing_enabled": true,
			"storage_min_free_gb":        10,
			"auto_cleanup_enabled":      false,
			"auto_cleanup_days":         7,
		})
		return
	}
	cfg, err := s.SettingsService.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	if s.SettingsService == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "settings service not initialized"})
		return
	}
	var req settings.SystemSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.SettingsService.UpdateSettings(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	username := c.GetString("username")
	s.AuditService.Record("UPDATE_SETTINGS", "SystemSettings", fmt.Sprintf("Balancing=%v, MinFree=%dGB, Cleanup=%v, Days=%d", req.StorageBalancingEnabled, req.StorageMinFreeGB, req.AutoCleanupEnabled, req.AutoCleanupDays), "SUCCESS", username)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "settings updated successfully", "settings": req})
}

func (s *Server) handleTriggerCleanup(c *gin.Context) {
	days := 7
	if s.SettingsService != nil {
		if set, err := s.SettingsService.GetSettings(c.Request.Context()); err == nil && set.AutoCleanupDays > 0 {
			days = set.AutoCleanupDays
		}
	}
	if qDays := c.Query("days"); qDays != "" {
		if n, err := strconv.Atoi(qDays); err == nil && n > 0 {
			days = n
		}
	}

	res, err := s.OfflineService.CleanExpiredTasks(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	username := c.GetString("username")
	s.AuditService.Record("MANUAL_CLEANUP", "ExpiredTasks", fmt.Sprintf("Manually triggered cleanup for tasks older than %d days. Cleaned %d tasks.", days, res.CleanedTasksCount), "SUCCESS", username)

	c.JSON(http.StatusOK, gin.H{
		"success":             true,
		"cleaned_tasks_count": res.CleanedTasksCount,
		"retention_days":      days,
		"message":             fmt.Sprintf("已成功清理 %d 个超过 %d 天的过期离线任务，远端文件与回收站已清空", res.CleanedTasksCount, days),
	})
}

