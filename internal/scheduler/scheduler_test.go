package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
	"pikpak-manager/internal/pikpak"
)

func setupTestScheduler(t *testing.T) (*AccountScheduler, *account.Service, func()) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_sched_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-key-32-chars-for-testing!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}

	svc := account.NewService(database, cfg.AppSecret)
	sched := NewAccountScheduler(svc)

	cleanup := func() {
		database.Close()
		os.RemoveAll(tempDir)
	}

	return sched, svc, cleanup
}

// Scenario 1: Account-A quota exhausted, system automatically fails over to Account-B!
func TestScheduler_QuotaExhaustedFailover(t *testing.T) {
	sched, svc, cleanup := setupTestScheduler(t)
	defer cleanup()

	ctx := context.Background()

	// Create Account-A (priority 20) and Account-B (priority 10)
	accA, err := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-A",
		Username:  "a@test.com",
		Password:  "passA",
		Priority:  20,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateAccount A failed: %v", err)
	}

	accB, err := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-B",
		Username:  "b@test.com",
		Password:  "passB",
		Priority:  10,
		IsEnabled: true,
	})
	if err != nil {
		t.Fatalf("CreateAccount B failed: %v", err)
	}

	// Mock task creator: Account-A returns ErrQuotaExceeded, Account-B succeeds
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		if client.GetAccountID() == accA.ID {
			return nil, pikpak.ErrQuotaExceeded
		}
		if client.GetAccountID() == accB.ID {
			return &pikpak.OfflineTask{
				ID:       "task_b_123",
				Name:     "Movie.mp4",
				Phase:    "PHASE_TYPE_RUNNING",
				Progress: 0,
			}, nil
		}
		return nil, errors.New("unknown account")
	})

	// Submit task
	task, usedAcc, err := sched.SubmitOfflineTask(ctx, "magnet:?xt=urn:btih:test1", "Movie.mp4")
	if err != nil {
		t.Fatalf("SubmitOfflineTask failed: %v", err)
	}

	if task.ID != "task_b_123" {
		t.Errorf("Expected task created on B (task_b_123), got %s", task.ID)
	}
	if usedAcc.ID != accB.ID {
		t.Errorf("Expected task used Account-B (%d), got Account %d", accB.ID, usedAcc.ID)
	}

	// Verify Account-A status is now marked as QUOTA_EXHAUSTED in db
	accAUpdated, err := svc.GetAccount(accA.ID)
	if err != nil {
		t.Fatalf("GetAccount A failed: %v", err)
	}
	if accAUpdated.Status != "QUOTA_EXHAUSTED" {
		t.Errorf("Expected Account-A to be QUOTA_EXHAUSTED, got %s", accAUpdated.Status)
	}
}

// Scenario 2: Both A and B are quota exhausted -> returns ErrAllAccountsQuotaExhausted
func TestScheduler_AllAccountsQuotaExhausted(t *testing.T) {
	sched, svc, cleanup := setupTestScheduler(t)
	defer cleanup()

	ctx := context.Background()

	accA, _ := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-A",
		Username:  "a@test.com",
		Priority:  10,
		IsEnabled: true,
	})
	accB, _ := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-B",
		Username:  "b@test.com",
		Priority:  10,
		IsEnabled: true,
	})

	// Both return Quota Exceeded
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return nil, pikpak.ErrQuotaExceeded
	})

	task, usedAcc, err := sched.SubmitOfflineTask(ctx, "magnet:?xt=urn:btih:test2", "")
	if task != nil || usedAcc != nil {
		t.Errorf("Expected nil task and account when all exhausted")
	}
	if !errors.Is(err, ErrAllAccountsQuotaExhausted) {
		t.Fatalf("Expected ErrAllAccountsQuotaExhausted, got: %v", err)
	}

	// Both accounts should now be QUOTA_EXHAUSTED
	a1, _ := svc.GetAccount(accA.ID)
	b1, _ := svc.GetAccount(accB.ID)
	if a1.Status != "QUOTA_EXHAUSTED" || b1.Status != "QUOTA_EXHAUSTED" {
		t.Errorf("Expected both accounts to be marked QUOTA_EXHAUSTED, got A=%s, B=%s", a1.Status, b1.Status)
	}
}

// Scenario 3: Account-A proxy fails -> recognized as PROXY_FAILED, NOT marked quota exhausted
func TestScheduler_ProxyFailureDoesNotExhaustQuota(t *testing.T) {
	sched, svc, cleanup := setupTestScheduler(t)
	defer cleanup()

	ctx := context.Background()

	accA, _ := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-A",
		Username:  "a@test.com",
		Priority:  20,
		IsEnabled: true,
	})
	accB, _ := svc.CreateAccount(ctx, account.CreateAccountReq{
		Name:      "Account-B",
		Username:  "b@test.com",
		Priority:  10,
		IsEnabled: true,
	})

	// Account-A fails with proxy error, Account-B succeeds
	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		if client.GetAccountID() == accA.ID {
			return nil, pikpak.ErrProxyFailed
		}
		return &pikpak.OfflineTask{ID: "task_b_ok"}, nil
	})

	task, usedAcc, err := sched.SubmitOfflineTask(ctx, "magnet:?xt=urn:btih:test3", "")
	if err != nil {
		t.Fatalf("SubmitOfflineTask failed: %v", err)
	}
	if usedAcc.ID != accB.ID || task.ID != "task_b_ok" {
		t.Errorf("Expected task executed on Account-B")
	}

	// Account-A must be PROXY_FAILED, NOT QUOTA_EXHAUSTED!
	aUpdated, _ := svc.GetAccount(accA.ID)
	if aUpdated.Status != "PROXY_FAILED" {
		t.Errorf("Expected Account-A to be PROXY_FAILED, got %s", aUpdated.Status)
	}
}

// Scenario: Round Robin rotation among equal priorities
func TestScheduler_RoundRobinRotation(t *testing.T) {
	sched, svc, cleanup := setupTestScheduler(t)
	defer cleanup()

	ctx := context.Background()

	acc1, _ := svc.CreateAccount(ctx, account.CreateAccountReq{Name: "Acc1", Priority: 10, IsEnabled: true})
	acc2, _ := svc.CreateAccount(ctx, account.CreateAccountReq{Name: "Acc2", Priority: 10, IsEnabled: true})

	pickedIDs := make(map[int64]int)
	for i := 0; i < 10; i++ {
		picked, err := sched.SelectAccount(nil)
		if err != nil {
			t.Fatalf("SelectAccount failed: %v", err)
		}
		pickedIDs[picked.ID]++
	}

	if pickedIDs[acc1.ID] != 5 || pickedIDs[acc2.ID] != 5 {
		t.Errorf("Expected even 5/5 distribution, got: %v", pickedIDs)
	}
}

// Scenario: Concurrent task submissions
func TestScheduler_ConcurrentSubmissions(t *testing.T) {
	sched, svc, cleanup := setupTestScheduler(t)
	defer cleanup()

	ctx := context.Background()

	_, _ = svc.CreateAccount(ctx, account.CreateAccountReq{Name: "Acc1", Priority: 10, IsEnabled: true})
	_, _ = svc.CreateAccount(ctx, account.CreateAccountReq{Name: "Acc2", Priority: 10, IsEnabled: true})
	_, _ = svc.CreateAccount(ctx, account.CreateAccountReq{Name: "Acc3", Priority: 10, IsEnabled: true})

	sched.SetTaskCreatorHook(func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error) {
		return &pikpak.OfflineTask{ID: "concurrent_task_" + client.GetAccountName()}, nil
	})

	var wg sync.WaitGroup
	errCount := 0
	var mu sync.Mutex

	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(taskIdx int) {
			defer wg.Done()
			task, _, err := sched.SubmitOfflineTask(ctx, "magnet:?xt=urn:btih:concurrent", "")
			if err != nil || task == nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	if errCount > 0 {
		t.Errorf("Expected 0 concurrent task errors, got %d", errCount)
	}
}
