package apikey

import (
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestApiKey_CreateAndValidate(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_apikey_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-apikey-test",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewService(database)

	// Create key
	key, err := svc.GenerateAPIKey("External-Scraper", "offline:create,offline:read")
	if err != nil {
		t.Fatalf("GenerateAPIKey failed: %v", err)
	}
	if key.FullKey == "" || key.KeyPrefix == "" {
		t.Fatalf("Expected key and prefix")
	}

	// Validate valid key
	validated, err := svc.ValidateKey(key.FullKey, "offline:create")
	if err != nil {
		t.Fatalf("ValidateKey failed: %v", err)
	}
	if validated.ID != key.ID {
		t.Errorf("Expected key ID %d, got %d", key.ID, validated.ID)
	}

	// Validate missing permission
	_, err = svc.ValidateKey(key.FullKey, "admin:write")
	if err == nil {
		t.Errorf("Expected validation to fail with missing permission")
	}

	// Disable key
	err = svc.ToggleKey(key.ID, false)
	if err != nil {
		t.Fatalf("ToggleKey failed: %v", err)
	}

	// Validate disabled key
	_, err = svc.ValidateKey(key.FullKey, "offline:create")
	if err == nil {
		t.Errorf("Expected disabled key to fail validation")
	}
}
