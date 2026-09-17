package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pikpak-manager/internal/crypto"
)

func TestConfig_GenerateRandomString(t *testing.T) {
	s1 := GenerateRandomString(16)
	s2 := GenerateRandomString(16)

	if len(s1) != 32 { // 16 bytes hex encoded = 32 chars
		t.Errorf("Expected 32 chars for 16 bytes, got %d", len(s1))
	}
	if s1 == s2 {
		t.Errorf("Expected random strings to differ, got identical %s", s1)
	}
}

func TestConfig_LoadDefaultsAndEnv(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_cfg_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("PORT", "9999")
	os.Setenv("DATA_DIR", tempDir)
	os.Setenv("ADMIN_USERNAME", "custom_admin")
	os.Setenv("ADMIN_PASSWORD", "custom_pass_123")
	os.Setenv("LOG_LEVEL", "DEBUG")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATA_DIR")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
		os.Unsetenv("LOG_LEVEL")
	}()

	cfg := Load()

	if cfg.Port != "9999" {
		t.Errorf("Expected port 9999, got %s", cfg.Port)
	}
	if cfg.AdminUsername != "custom_admin" {
		t.Errorf("Expected admin username custom_admin, got %s", cfg.AdminUsername)
	}
	if cfg.AdminPassword != "custom_pass_123" {
		t.Errorf("Expected admin password custom_pass_123, got %s", cfg.AdminPassword)
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("Expected log level DEBUG, got %s", cfg.LogLevel)
	}

	// Verify .secret_key was generated and persisted
	secretFile := filepath.Join(tempDir, ".secret_key")
	data, err := os.ReadFile(secretFile)
	if err != nil {
		t.Fatalf("Expected .secret_key to be created: %v", err)
	}
	if len(strings.TrimSpace(string(data))) < 16 {
		t.Errorf("Secret key in file too short: %s", string(data))
	}
}

func TestConfig_LegacyDbDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_cfg_legacy_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Simulate existing pikpak.db without .secret_key
	dummyDb := filepath.Join(tempDir, "pikpak.db")
	_ = os.WriteFile(dummyDb, []byte("sqlite format 3\x00"), 0600)

	os.Setenv("DATA_DIR", tempDir)
	defer os.Unsetenv("DATA_DIR")

	cfg := Load()

	if cfg.AppSecret != crypto.LegacyDefaultSecret {
		t.Errorf("Expected legacy default secret for existing db, got %s", cfg.AppSecret)
	}
}
