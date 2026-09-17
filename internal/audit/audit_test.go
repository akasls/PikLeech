package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestAudit_RecordAndList(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_audit_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "audit-secret-key-32-characters!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewService(database)

	// Record 3 events with slight delay to ensure sequential persistence
	svc.Record("account_create", "account:1", "Created account 1", "SUCCESS", "admin")
	time.Sleep(20 * time.Millisecond)
	svc.Record("file_delete", "file:xyz", "Deleted file xyz", "SUCCESS", "admin")
	time.Sleep(20 * time.Millisecond)
	svc.Record("offline_submit", "task:123", "Submitted magnet link", "SUCCESS", "user1")

	// Wait briefly for asynchronous goroutines to persist logs
	time.Sleep(100 * time.Millisecond)

	logs, total, err := svc.ListLogs(10, 0)
	if err != nil {
		t.Fatalf("ListLogs failed: %v", err)
	}

	if total != 3 {
		t.Errorf("Expected 3 total logs, got %d", total)
	}

	if len(logs) != 3 {
		t.Fatalf("Expected 3 log items, got %d", len(logs))
	}

	// Verify IDs are ordered descending
	if logs[0].ID <= logs[1].ID || logs[1].ID <= logs[2].ID {
		t.Errorf("Expected logs ordered by id DESC, got IDs: %d, %d, %d", logs[0].ID, logs[1].ID, logs[2].ID)
	}

	// Test pagination limit
	logsPaginated, _, err := svc.ListLogs(2, 0)
	if err != nil {
		t.Fatalf("ListLogs with limit failed: %v", err)
	}
	if len(logsPaginated) != 2 {
		t.Errorf("Expected 2 logs with limit=2, got %d", len(logsPaginated))
	}
}
