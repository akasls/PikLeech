package account

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestAccount_CRUD(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_account_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-key-for-test-32bytes-ok!!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewService(database, cfg.AppSecret)
	ctx := context.Background()

	// 1. Create account
	acc, err := svc.CreateAccount(ctx, CreateAccountReq{
		Name:         "Account-A",
		Username:     "testuser@example.com",
		Password:     "supersecretpassword",
		RefreshToken: "initial_refresh_token_123",
		ProxyURL:     "http://127.0.0.1:8080",
		Priority:     10,
		IsEnabled:    true,
	})
	if err != nil {
		t.Fatalf("CreateAccount failed: %v", err)
	}

	if acc.Name != "Account-A" {
		t.Errorf("Expected Account-A, got %s", acc.Name)
	}
	if !acc.HasPassword || !acc.HasRefreshToken {
		t.Errorf("Expected password and refresh token flags to be true")
	}

	// 2. Client instantiation & decrypt check
	client, err := svc.GetClient(acc.ID)
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if client.GetAccountID() != acc.ID {
		t.Errorf("Expected client account ID %d, got %d", acc.ID, client.GetAccountID())
	}

	// 3. Update account proxy
	newProxy := "http://127.0.0.1:9090"
	updated, err := svc.UpdateAccount(ctx, acc.ID, UpdateAccountReq{
		ProxyURL: &newProxy,
	})
	if err != nil {
		t.Fatalf("UpdateAccount failed: %v", err)
	}
	if updated.ProxyURL != newProxy {
		t.Errorf("Expected %s, got %s", newProxy, updated.ProxyURL)
	}

	// 4. Update tokens callback
	err = svc.UpdateTokens(acc.ID, "new_access_token", "new_refresh_token")
	if err != nil {
		t.Fatalf("UpdateTokens failed: %v", err)
	}

	// 5. Status update & Quota reset
	err = svc.UpdateStatus(acc.ID, "QUOTA_EXHAUSTED", "task_daily_create_limit")
	if err != nil {
		t.Fatalf("UpdateStatus failed: %v", err)
	}
	accAfter, _ := svc.GetAccount(acc.ID)
	if accAfter.Status != "QUOTA_EXHAUSTED" {
		t.Errorf("Expected QUOTA_EXHAUSTED, got %s", accAfter.Status)
	}

	err = svc.ResetQuota(acc.ID)
	if err != nil {
		t.Fatalf("ResetQuota failed: %v", err)
	}
	accReset, _ := svc.GetAccount(acc.ID)
	if accReset.Status != "HEALTHY" {
		t.Errorf("Expected HEALTHY after reset, got %s", accReset.Status)
	}
}
