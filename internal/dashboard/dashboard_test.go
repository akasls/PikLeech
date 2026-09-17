package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestDashboard_GetStats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_dash_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "dashboard-secret-key-32-chars!!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// Insert accounts with various statuses
	now := time.Now().UTC()
	future := now.Add(10 * time.Minute)
	_, _ = database.Exec(`
		INSERT INTO pikpak_accounts (name, status, is_enabled, cooldown_until, used_space, total_space, created_at, updated_at)
		VALUES 
			('acc1', 'HEALTHY', 1, NULL, 5000, 10000, ?, ?),
			('acc2', 'QUOTA_EXHAUSTED', 1, NULL, 9000, 10000, ?, ?),
			('acc3', 'HEALTHY', 1, ?, 1000, 10000, ?, ?),
			('acc4', 'DISABLED', 0, NULL, 0, 10000, ?, ?)
	`, now, now, now, now, future, now, now, now, now)

	// Insert offline tasks
	_, _ = database.Exec(`
		INSERT INTO offline_tasks (id, source_url, account_id, status, progress, created_at, updated_at)
		VALUES 
			('t1', 'magnet:?1', 1, 'RUNNING', 50, ?, ?),
			('t2', 'magnet:?2', 1, 'COMPLETE', 100, ?, ?),
			('t3', 'magnet:?3', 2, 'ERROR', 0, ?, ?)
	`, now, now, now, now, now, now)

	// Insert files in cache
	_, _ = database.Exec(`
		INSERT INTO file_cache (virtual_id, account_id, pikpak_file_id, parent_id, name, size, mime_type, kind, user_id, created_time, modified_time, updated_at)
		VALUES 
			('v1', 1, 'f1', '', 'video.mp4', 5000, 'video/mp4', 'drive#file', 1, ?, ?, ?),
			('v2', 1, 'f2', '', 'folder', 0, 'application/x-directory', 'drive#folder', 1, ?, ?, ?)
	`, now, now, now, now, now, now)

	svc := NewService(database)
	stats, err := svc.GetStats()
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalAccounts != 4 {
		t.Errorf("Expected 4 total accounts, got %d", stats.TotalAccounts)
	}
	if stats.HealthyAccounts != 2 {
		t.Errorf("Expected 2 healthy accounts, got %d", stats.HealthyAccounts)
	}
	if stats.QuotaExhaustedCount != 1 {
		t.Errorf("Expected 1 quota exhausted account, got %d", stats.QuotaExhaustedCount)
	}
	if stats.DisabledCount != 1 {
		t.Errorf("Expected 1 disabled account, got %d", stats.DisabledCount)
	}
	if stats.CooldownCount != 1 {
		t.Errorf("Expected 1 cooldown account, got %d", stats.CooldownCount)
	}

	if stats.TotalSpace != 40000 {
		t.Errorf("Expected 40000 total space, got %d", stats.TotalSpace)
	}
	if stats.UsedSpace != 15000 {
		t.Errorf("Expected 15000 used space, got %d", stats.UsedSpace)
	}

	if stats.TotalTasks != 3 {
		t.Errorf("Expected 3 total tasks, got %d", stats.TotalTasks)
	}
	if stats.RunningTasks != 1 {
		t.Errorf("Expected 1 running task, got %d", stats.RunningTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("Expected 1 completed task, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("Expected 1 failed task, got %d", stats.FailedTasks)
	}

	// Total files should only count non-folders
	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file (excluding folder), got %d", stats.TotalFiles)
	}
}
