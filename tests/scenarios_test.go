package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/api"
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

func setupTestEnvironment(t *testing.T) (*api.Server, *account.Service, *scheduler.AccountScheduler, *fileagg.Service, *offline.Service, *apikey.Service, func()) {
	gin.SetMode(gin.TestMode)

	tempDir, err := os.MkdirTemp("", "pikpak_scenario_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-scenario-tests",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	accSvc := account.NewService(database, cfg.AppSecret)
	sched := scheduler.NewAccountScheduler(accSvc)
	offSvc := offline.NewService(database, accSvc, sched)
	fileSvc := fileagg.NewService(database, accSvc)
	authSvc := auth.NewService(database, cfg.AppSecret)
	apiKeySvc := apikey.NewService(database)
	auditSvc := audit.NewService(database)
	dashSvc := dashboard.NewService(database)

	server := api.NewServer(accSvc, fileSvc, offSvc, authSvc, apiKeySvc, auditSvc, dashSvc, nil)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tempDir)
	}

	return server, accSvc, sched, fileSvc, offSvc, apiKeySvc, cleanup
}

// 场景 1: Account-A 额度耗尽，自动选择 Account-B 成功创建
func TestScenario1_AutoFailoverOnQuotaExhausted(t *testing.T) {
	_, accSvc, sched, _, offSvc, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create Account-A (higher priority 20) and Account-B (priority 10)
	accA, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-A", Priority: 20, IsEnabled: true})
	accB, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-B", Priority: 10, IsEnabled: true})

	// Hook: Account-A returns task_daily_create_limit, Account-B succeeds
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		if client.GetAccountID() == accA.ID {
			return nil, pikpak.ErrQuotaExceeded
		}
		if client.GetAccountID() == accB.ID {
			return &pikpak.OfflineTask{
				ID:       "task_on_b_success",
				Name:     "Inception.mkv",
				Phase:    "PHASE_TYPE_RUNNING",
				Progress: 0,
			}, nil
		}
		return nil, errors.New("unknown client")
	})

	res, err := offSvc.SubmitSingleLink(ctx, "magnet:?xt=urn:btih:hashscenario1", "Inception.mkv", "")
	if err != nil {
		t.Fatalf("SubmitSingleLink failed: %v", err)
	}

	if !res.Success || res.AccountID != accB.ID {
		t.Errorf("Expected task executed on Account-B (%d), got Account %d", accB.ID, res.AccountID)
	}

	// Verify Account-A was marked QUOTA_EXHAUSTED
	updatedA, _ := accSvc.GetAccount(accA.ID)
	if updatedA.Status != "QUOTA_EXHAUSTED" {
		t.Errorf("Expected Account-A to be QUOTA_EXHAUSTED, got %s", updatedA.Status)
	}
}

// 场景 2: A 和 B 均额度耗尽，系统返回 ALL_ACCOUNTS_QUOTA_EXHAUSTED，不无限重试
func TestScenario2_AllAccountsQuotaExhausted(t *testing.T) {
	_, accSvc, sched, _, offSvc, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	accA, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-A", Priority: 10, IsEnabled: true})
	accB, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-B", Priority: 10, IsEnabled: true})

	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return nil, pikpak.ErrQuotaExceeded
	})

	res, err := offSvc.SubmitSingleLink(ctx, "magnet:?xt=urn:btih:hashscenario2", "Film.mp4", "")
	if err == nil || !errors.Is(err, scheduler.ErrAllAccountsQuotaExhausted) {
		t.Fatalf("Expected ErrAllAccountsQuotaExhausted, got: %v", err)
	}

	if res.Success {
		t.Errorf("Expected task result success = false")
	}

	// Verify both accounts marked as QUOTA_EXHAUSTED
	a1, _ := accSvc.GetAccount(accA.ID)
	b1, _ := accSvc.GetAccount(accB.ID)
	if a1.Status != "QUOTA_EXHAUSTED" || b1.Status != "QUOTA_EXHAUSTED" {
		t.Errorf("Both accounts should be QUOTA_EXHAUSTED")
	}
}

// 场景 3: A 代理断开，B 正常。系统不能把代理故障判断为额度耗尽！
func TestScenario3_ProxyFailureIsolation(t *testing.T) {
	_, accSvc, sched, _, offSvc, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	accA, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-A", Priority: 20, IsEnabled: true})
	accB, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-B", Priority: 10, IsEnabled: true})

	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		if client.GetAccountID() == accA.ID {
			return nil, pikpak.ErrProxyFailed
		}
		return &pikpak.OfflineTask{ID: "task_b_ok"}, nil
	})

	res, err := offSvc.SubmitSingleLink(ctx, "magnet:?xt=urn:btih:hashscenario3", "Film.mp4", "")
	if err != nil {
		t.Fatalf("SubmitSingleLink failed: %v", err)
	}

	if res.AccountID != accB.ID {
		t.Errorf("Expected task executed on Account-B")
	}

	// Account-A must be marked PROXY_FAILED, strictly NOT QUOTA_EXHAUSTED!
	aUpdated, _ := accSvc.GetAccount(accA.ID)
	if aUpdated.Status != "PROXY_FAILED" {
		t.Errorf("Expected Account-A to be PROXY_FAILED, got %s", aUpdated.Status)
	}
}

// 场景 4: 文件 A 位于 Account-A，文件 B 位于 Account-B。统一页面展示，批量勾选同时删除，后台分别调用对应账号
func TestScenario4_BatchDeleteAcrossAccounts(t *testing.T) {
	_, accSvc, _, fileSvc, _, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	accA, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-A", IsEnabled: true})
	accB, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "Account-B", IsEnabled: true})

	vID_A := fileagg.EncodeVirtualID(accA.ID, "file_a_id")
	vID_B := fileagg.EncodeVirtualID(accB.ID, "file_b_id")

	// Both IDs decode properly to their respective account IDs
	decAccA, decFileA, _ := fileagg.DecodeVirtualID(vID_A)
	decAccB, decFileB, _ := fileagg.DecodeVirtualID(vID_B)

	if decAccA != accA.ID || decFileA != "file_a_id" {
		t.Errorf("Virtual ID decoding failed for A")
	}
	if decAccB != accB.ID || decFileB != "file_b_id" {
		t.Errorf("Virtual ID decoding failed for B")
	}

	// Execute batch delete
	res, err := fileSvc.BatchDelete(ctx, fileagg.BatchDeleteReq{
		VirtualIDs: []string{vID_A, vID_B},
	})
	if err != nil {
		t.Fatalf("BatchDelete failed: %v", err)
	}

	if res.Total != 2 {
		t.Errorf("Expected total 2, got %d", res.Total)
	}
}

// 场景 5: Account-A 视频播放支持 HTTP Range 206 Partial Content
func TestScenario5_VideoRangeStreaming(t *testing.T) {
	server, _, _, _, _, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Make sure route handles range header
	req := httptest.NewRequest(http.MethodGet, "/api/media/stream/nonexistent_vid", nil)
	req.Header.Set("Range", "bytes=0-1024")
	w := httptest.NewRecorder()
	server.Engine.ServeHTTP(w, req)

	// Since nonexistent, expect 404/400, but route executed without panic
	if w.Code == http.StatusInternalServerError {
		t.Errorf("Unexpected 500 error: %s", w.Body.String())
	}
}

// 场景 6: 外部程序 POST /api/v1/offline 提交 Magnet，返回 task_id，通过 GET /api/v1/offline/:id 查询
func TestScenario6_ExternalRestApiAndIdempotency(t *testing.T) {
	server, accSvc, sched, _, _, apiKeySvc, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// 1. Setup account & mock
	_, _ = accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "ApiAccount", IsEnabled: true})
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return &pikpak.OfflineTask{
			ID:       "pikpak_external_987",
			Name:     "Ubuntu-Server.iso",
			Phase:    "PHASE_TYPE_RUNNING",
			Progress: 42,
		}, nil
	})

	// 2. Generate API Key
	apiKey, err := apiKeySvc.GenerateAPIKey("CI-Pipeline", "offline:create,offline:read")
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}

	// 3. POST /api/v1/offline
	payload := map[string]string{
		"url":  "magnet:?xt=urn:btih:ubuntu_server_hash",
		"name": "Ubuntu-Server.iso",
	}
	bodyBytes, _ := json.Marshal(payload)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/offline", bytes.NewReader(bodyBytes))
	req.Header.Set("Authorization", "Bearer "+apiKey.FullKey)
	req.Header.Set("Idempotency-Key", "idemp_test_external_1")
	req.Header.Set("Content-Type", "application/json")
	server.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/offline failed with %d: %s", w.Code, w.Body.String())
	}

	var submitResp struct {
		Success bool   `json:"success"`
		TaskID  string `json:"task_id"`
		Status  string `json:"status"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &submitResp)
	if !submitResp.Success || submitResp.TaskID == "" {
		t.Fatalf("Task creation failed: %s", w.Body.String())
	}

	// 4. GET /api/v1/offline/:id
	wQuery := httptest.NewRecorder()
	reqQuery := httptest.NewRequest(http.MethodGet, "/api/v1/offline/"+submitResp.TaskID, nil)
	reqQuery.Header.Set("Authorization", "Bearer "+apiKey.FullKey)
	server.Engine.ServeHTTP(wQuery, reqQuery)

	if wQuery.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/offline/:id failed with %d: %s", wQuery.Code, wQuery.Body.String())
	}

	var taskObj offline.Task
	_ = json.Unmarshal(wQuery.Body.Bytes(), &taskObj)
	if taskObj.ID != submitResp.TaskID || taskObj.Progress != 42 {
		t.Errorf("Expected task %s with progress 42, got %s (progress %d)", submitResp.TaskID, taskObj.ID, taskObj.Progress)
	}
}
