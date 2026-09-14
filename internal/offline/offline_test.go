package offline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/pikpak"
	"pikpak-manager/internal/scheduler"
)

func TestOffline_SubmitAndIdempotency(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_off_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-offline-test!",
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
	svc := NewService(database, accSvc, sched)
	ctx := context.Background()

	// Create test account
	acc, err := accSvc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "OfflineAcc",
		Username:  "off@test.com",
		Priority:  10,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	// Mock task creation
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return &pikpak.OfflineTask{
			ID:       "pikpak_task_999",
			Name:     "TestFilm.mkv",
			Phase:    "PHASE_TYPE_RUNNING",
			Progress: 15,
		}, nil
	})

	idempotencyKey := "idem_unique_key_12345"

	// First submission
	res1, err := svc.SubmitSingleLink(ctx, "magnet:?xt=urn:btih:hash1", "TestFilm.mkv", idempotencyKey)
	if err != nil {
		t.Fatalf("SubmitSingleLink 1 failed: %v", err)
	}
	if !res1.Success || res1.AccountID != acc.ID {
		t.Fatalf("Expected successful task creation on account %d", acc.ID)
	}

	// Second submission with identical Idempotency-Key
	res2, err := svc.SubmitSingleLink(ctx, "magnet:?xt=urn:btih:hash1", "TestFilm.mkv", idempotencyKey)
	if err != nil {
		t.Fatalf("SubmitSingleLink 2 failed: %v", err)
	}

	// Must return the exact same task ID (idempotency!)
	if res2.ID != res1.ID {
		t.Fatalf("Expected idempotent response with ID %s, got %s", res1.ID, res2.ID)
	}

	// Query task
	task, err := svc.GetTask(ctx, res1.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.PikPakTaskID != "pikpak_task_999" {
		t.Errorf("Expected pikpak_task_999, got %s", task.PikPakTaskID)
	}

	// List tasks
	list, total, err := svc.ListTasks(ctx, "", 10, 0)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("Expected exactly 1 task in database, found %d", total)
	}
}
