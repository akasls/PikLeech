package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

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
	"pikpak-manager/internal/scheduler"
	"pikpak-manager/internal/settings"
)

func setupSecurityAuditEnvironment(t *testing.T) (*api.Server, *sql.DB, *auth.Service, string, string, string) {
	tempDB := fmt.Sprintf("%s/security_test_%d.db", os.TempDir(), time.Now().UnixNano())
	cfg := &config.Config{
		DBPath:        tempDB,
		AppSecret:     "security-audit-secret-key-32-bytes!!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword123",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		_ = os.Remove(tempDB)
		_ = os.Remove(tempDB + "-wal")
		_ = os.Remove(tempDB + "-shm")
	})

	authSvc := auth.NewService(database, cfg.AppSecret)
	accSvc := account.NewService(database, cfg.AppSecret)
	sched := scheduler.NewAccountScheduler(accSvc)
	offSvc := offline.NewService(database, accSvc, sched)
	fileSvc := fileagg.NewService(database, accSvc)
	keySvc := apikey.NewService(database)
	auditSvc := audit.NewService(database)
	dashSvc := dashboard.NewService(database)
	setSvc := settings.NewService(database)

	server := api.NewServer(
		accSvc,
		fileSvc,
		offSvc,
		authSvc,
		keySvc,
		auditSvc,
		dashSvc,
		nil,
		setSvc,
	)

	// Create Normal User A and Normal User B
	userA, err := authSvc.CreateUser(context.Background(), "user_a", "userpass123", "user")
	if err != nil {
		t.Fatalf("Failed to create user A: %v", err)
	}
	uidA := userA.ID

	userB, err := authSvc.CreateUser(context.Background(), "user_b", "userpass123", "user")
	if err != nil {
		t.Fatalf("Failed to create user B: %v", err)
	}
	uidB := userB.ID

	adminToken, _ := authSvc.GenerateSessionToken(1, "admin", "admin")
	tokenA, _ := authSvc.GenerateSessionToken(uidA, "user_a", "user")
	tokenB, _ := authSvc.GenerateSessionToken(uidB, "user_b", "user")

	return server, database, authSvc, adminToken, tokenA, tokenB
}

// 1. Test Security Headers
func TestSecurity_Headers(t *testing.T) {
	server, _, _, _, _, _ := setupSecurityAuditEnvironment(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	server.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"X-XSS-Protection":       "1; mode=block",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for k, expected := range headers {
		actual := w.Header().Get(k)
		if actual != expected {
			t.Errorf("Security header %s mismatch: expected '%s', got '%s'", k, expected, actual)
		}
	}
}

// 2. Test RBAC Isolation: Regular Users Cannot Access Admin Endpoints
func TestSecurity_RBAC_AdminEndpoints(t *testing.T) {
	server, _, _, _, tokenA, _ := setupSecurityAuditEnvironment(t)

	adminEndpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/accounts"},
		{http.MethodPost, "/api/accounts"},
		{http.MethodGet, "/api/users"},
		{http.MethodPost, "/api/users"},
		{http.MethodGet, "/api/apikeys"},
		{http.MethodGet, "/api/audit/logs"},
		{http.MethodGet, "/api/dashboard/stats"},
		{http.MethodGet, "/api/settings"},
		{http.MethodPost, "/api/settings"},
	}

	for _, ep := range adminEndpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			req.Header.Set("Authorization", "Bearer "+tokenA)
			w := httptest.NewRecorder()
			server.Engine.ServeHTTP(w, req)

			if w.Code != http.StatusForbidden {
				t.Errorf("Expected 403 Forbidden for non-admin on %s %s, got HTTP %d", ep.method, ep.path, w.Code)
			}
		})
	}
}

// 3. Test IDOR Protection on Offline Tasks
func TestSecurity_IDOR_OfflineTasks(t *testing.T) {
	server, database, authSvc, adminToken, tokenA, tokenB := setupSecurityAuditEnvironment(t)

	// Decode token A to get uidA
	claimsA, err := authSvc.ValidateSessionToken(tokenA)
	if err != nil {
		t.Fatalf("ValidateSessionToken failed: %v", err)
	}

	// Insert dummy pikpak account
	resAcc, err := database.Exec(`
		INSERT INTO pikpak_accounts (
			name, username, password_enc, device_id, priority, is_enabled,
			status, daily_task_count, total_files, used_space, total_space,
			created_at, updated_at
		) VALUES ('SecAcc', 'sec@pikpak.com', 'pwd', 'dev1', 10, 1, 'HEALTHY', 0, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("Failed to insert account: %v", err)
	}
	accID, _ := resAcc.LastInsertId()

	// Insert an offline task belonging to user A
	taskIDA := "task-user-a-001"
	_, err = database.Exec(`
		INSERT INTO offline_tasks (
			id, source_url, file_name, account_id, status, progress, user_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'PENDING', 0, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, taskIDA, "magnet:?xt=urn:btih:dummy", "dummy.mp4", accID, claimsA.UserID)
	if err != nil {
		t.Fatalf("Failed to insert user A offline task: %v", err)
	}

	// User B attempts to access User A's task
	t.Run("User B cannot view User A task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/offline/tasks/"+taskIDA, nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound && w.Code != http.StatusForbidden {
			t.Errorf("Expected 404 or 403 when User B accesses User A task, got %d", w.Code)
		}
	})

	t.Run("User B cannot cancel User A task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/offline/tasks/"+taskIDA+"/cancel", nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Errorf("Expected 403 or 404 when User B cancels User A task, got %d", w.Code)
		}
	})

	t.Run("User B cannot delete User A task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/offline/tasks/"+taskIDA, nil)
		req.Header.Set("Authorization", "Bearer "+tokenB)
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden && w.Code != http.StatusNotFound {
			t.Errorf("Expected 403 or 404 when User B deletes User A task, got %d", w.Code)
		}
	})

	// User A can view User A's task
	t.Run("User A can view own task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/offline/tasks/"+taskIDA, nil)
		req.Header.Set("Authorization", "Bearer "+tokenA)
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for User A on own task, got %d", w.Code)
		}
	})

	// Admin can view User A's task
	t.Run("Admin can view any user task", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/offline/tasks/"+taskIDA, nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 OK for Admin on task, got %d", w.Code)
		}
	})
}

// 4. Test IDOR Protection on File Batch Delete and Rename
func TestSecurity_IDOR_Files(t *testing.T) {
	server, database, authSvc, _, tokenA, tokenB := setupSecurityAuditEnvironment(t)

	claimsA, err := authSvc.ValidateSessionToken(tokenA)
	if err != nil {
		t.Fatalf("ValidateSessionToken failed: %v", err)
	}

	// Insert dummy pikpak account
	resAcc, err := database.Exec(`
		INSERT INTO pikpak_accounts (
			name, username, password_enc, device_id, priority, is_enabled,
			status, daily_task_count, total_files, used_space, total_space,
			created_at, updated_at
		) VALUES ('SecAcc2', 'sec2@pikpak.com', 'pwd', 'dev2', 10, 1, 'HEALTHY', 0, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("Failed to insert account: %v", err)
	}
	accID, _ := resAcc.LastInsertId()

	fileVID := "dummy_vid_user_a"
	_, err = database.Exec(`
		INSERT INTO file_cache (
			virtual_id, account_id, pikpak_file_id, parent_id, name, size, mime_type, kind, user_id, updated_at
		) VALUES (?, ?, 'pf1', '', 'file_a.mp4', 1024, 'video/mp4', 'drive#file', ?, CURRENT_TIMESTAMP)
	`, fileVID, accID, claimsA.UserID)
	if err != nil {
		t.Fatalf("Failed to insert file owned by User A: %v", err)
	}

	t.Run("User B cannot delete User A file", func(t *testing.T) {
		body, _ := json.Marshal(fileagg.BatchDeleteReq{
			VirtualIDs: []string{fileVID},
		})

		req := httptest.NewRequest(http.MethodPost, "/api/files/batch-delete", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tokenB)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected 403 Forbidden on unauthorized batch delete, got HTTP %d: %s", w.Code, w.Body.String())
		}

		// Verify file still exists in DB
		var count int
		_ = database.QueryRow("SELECT COUNT(*) FROM file_cache WHERE virtual_id = ?", fileVID).Scan(&count)
		if count != 1 {
			t.Errorf("File was deleted by unauthorized User B! count=%d", count)
		}
	})
}
