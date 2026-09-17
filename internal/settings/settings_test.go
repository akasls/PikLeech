package settings

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestSettings_DefaultsAndUpdates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_settings_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "settings-secret-key-32-chars!!",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewService(database)
	ctx := context.Background()

	// 1. Verify Defaults
	initial, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings failed: %v", err)
	}

	if !initial.StorageBalancingEnabled {
		t.Errorf("Expected default StorageBalancingEnabled=true")
	}
	if initial.StorageMinFreeGB != 10 {
		t.Errorf("Expected default StorageMinFreeGB=10, got %d", initial.StorageMinFreeGB)
	}
	if initial.AutoCleanupEnabled {
		t.Errorf("Expected default AutoCleanupEnabled=false")
	}
	if initial.AutoCleanupDays != 7 {
		t.Errorf("Expected default AutoCleanupDays=7, got %d", initial.AutoCleanupDays)
	}

	// 2. Update Settings
	newCfg := SystemSettings{
		StorageBalancingEnabled: false,
		StorageMinFreeGB:        25,
		AutoCleanupEnabled:      true,
		AutoCleanupDays:         14,
	}

	if err := svc.UpdateSettings(ctx, newCfg); err != nil {
		t.Fatalf("UpdateSettings failed: %v", err)
	}

	// 3. Verify Cache and Persisted Settings
	updated, err := svc.GetSettings(ctx)
	if err != nil {
		t.Fatalf("GetSettings after update failed: %v", err)
	}

	if updated.StorageBalancingEnabled != false {
		t.Errorf("Expected StorageBalancingEnabled=false")
	}
	if updated.StorageMinFreeGB != 25 {
		t.Errorf("Expected StorageMinFreeGB=25, got %d", updated.StorageMinFreeGB)
	}
	if !updated.AutoCleanupEnabled {
		t.Errorf("Expected AutoCleanupEnabled=true")
	}
	if updated.AutoCleanupDays != 14 {
		t.Errorf("Expected AutoCleanupDays=14, got %d", updated.AutoCleanupDays)
	}
}
