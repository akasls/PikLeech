package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/apikey"
	"pikpak-manager/internal/audit"
	"pikpak-manager/internal/auth"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/dashboard"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/fileagg"
	"pikpak-manager/internal/offline"
	"pikpak-manager/internal/pikpak"
	"pikpak-manager/internal/scheduler"
)

func TestAPI_ServerAndV1(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tempDir, err := os.MkdirTemp("", "pikpak_test_api_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-api-testing!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	accSvc := account.NewService(database, cfg.AppSecret)
	sched := scheduler.NewAccountScheduler(accSvc)
	offSvc := offline.NewService(database, accSvc, sched)
	fileSvc := fileagg.NewService(database, accSvc)
	authSvc := auth.NewService(database, cfg.AppSecret)
	apiKeySvc := apikey.NewService(database)
	auditSvc := audit.NewService(database)
	dashSvc := dashboard.NewService(database)

	// Mock task creation on scheduler
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return &pikpak.OfflineTask{
			ID:       "task_api_123",
			Name:     "Video.mp4",
			Phase:    "PHASE_TYPE_RUNNING",
			Progress: 5,
		}, nil
	})

	server := NewServer(accSvc, fileSvc, offSvc, authSvc, apiKeySvc, auditSvc, dashSvc, nil)

	// 1. Test /health
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected /health 200, got %d", w.Code)
	}

	// 2. Test unauthenticated request to /api/accounts
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for /api/accounts, got %d", w.Code)
	}

	// 3. Test Admin Login
	loginBody, _ := json.Marshal(map[string]string{
		"username": "admin",
		"password": "adminpassword",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected login 200, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	if !loginResp.Success || loginResp.Token == "" {
		t.Fatalf("Login failed, response: %s", w.Body.String())
	}

	// 4. Test Authenticated access to /api/accounts with Token
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 with session token, got %d", w.Code)
	}

	// 5. Create an Account
	ctx := context.Background()
	_, _ = accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "PikPakAcc1", IsEnabled: true})

	// 6. Test External REST API /api/v1/offline
	// Generate API Key first
	key, err := apiKeySvc.GenerateAPIKey("TestKey", "offline:create,offline:read")
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	// Submit via POST /api/v1/offline
	offlineBody, _ := json.Marshal(map[string]string{
		"url": "magnet:?xt=urn:btih:apiresttest",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/offline", bytes.NewReader(offlineBody))
	req.Header.Set("Authorization", "Bearer "+key.FullKey)
	req.Header.Set("Content-Type", "application/json")
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected /api/v1/offline 200, got %d: %s", w.Code, w.Body.String())
	}

	var taskResp struct {
		Success bool   `json:"success"`
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &taskResp)
	if !taskResp.Success || taskResp.TaskID == "" {
		t.Fatalf("Expected successful task creation via REST API, got: %s", w.Body.String())
	}

	// Query via GET /api/v1/offline/:id
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/offline/"+taskResp.TaskID, nil)
	req.Header.Set("Authorization", "Bearer "+key.FullKey)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected /api/v1/offline/:id 200, got %d: %s", w.Code, w.Body.String())
	}

	// Check /api/v1/accounts/status does NOT leak password or token
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/accounts/status", nil)
	req.Header.Set("Authorization", "Bearer "+key.FullKey)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected /api/v1/accounts/status 200, got %d", w.Code)
	}
	bodyStr := w.Body.String()
	if bytes.Contains(w.Body.Bytes(), []byte("password")) || bytes.Contains(w.Body.Bytes(), []byte("refresh_token")) {
		t.Fatalf("Security violation: accounts/status leaked credential fields: %s", bodyStr)
	}

	// 7. Test User Management: Admin creates a regular user
	createUserBody, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "alicepassword123",
		"role":     "user",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(createUserBody))
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)
	req.Header.Set("Content-Type", "application/json")
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected create user 200, got %d: %s", w.Code, w.Body.String())
	}

	// 8. Normal user login
	aliceLoginBody, _ := json.Marshal(map[string]string{
		"username": "alice",
		"password": "alicepassword123",
	})
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(aliceLoginBody))
	req.Header.Set("Content-Type", "application/json")
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected alice login 200, got %d: %s", w.Code, w.Body.String())
	}
	var aliceLoginResp struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
		Role    string `json:"role"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &aliceLoginResp)
	if aliceLoginResp.Role != "user" {
		t.Errorf("Expected alice role 'user', got '%s'", aliceLoginResp.Role)
	}

	// 9. Normal user attempts to access Admin route /api/accounts -> must be 403 Forbidden!
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	req.Header.Set("Authorization", "Bearer "+aliceLoginResp.Token)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for normal user accessing /api/accounts, got %d: %s", w.Code, w.Body.String())
	}

	// 10. Normal user attempts to access /api/settings -> must be 403 Forbidden!
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	req.Header.Set("Authorization", "Bearer "+aliceLoginResp.Token)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for normal user accessing /api/settings, got %d: %s", w.Code, w.Body.String())
	}

	// 11. Normal user accesses /api/files -> allowed 200 OK
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/files", nil)
	req.Header.Set("Authorization", "Bearer "+aliceLoginResp.Token)
	server.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for normal user accessing /api/files, got %d", w.Code)
	}
}
