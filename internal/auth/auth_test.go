package auth

import (
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestAuth_LoginAndSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_auth_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "super-secret-key-32-chars-for-auth",
		AdminUsername: "testadmin",
		AdminPassword: "password123",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	svc := NewService(database, cfg.AppSecret)

	// Test correct login
	token, err := svc.Login("testadmin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatalf("Expected non-empty token")
	}

	// Test validate token
	claims, err := svc.ValidateSessionToken(token)
	if err != nil {
		t.Fatalf("ValidateSessionToken failed: %v", err)
	}
	if claims.Username != "testadmin" {
		t.Errorf("Expected claims username testadmin, got %s", claims.Username)
	}

	// Test wrong password
	_, err = svc.Login("testadmin", "wrongpassword")
	if err == nil {
		t.Errorf("Expected login to fail with wrong password")
	}

	// Test change password
	err = svc.ChangePassword(claims.UserID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Verify old password fails
	_, err = svc.Login("testadmin", "password123")
	if err == nil {
		t.Errorf("Expected old password to fail")
	}

	// Verify new password succeeds
	_, err = svc.Login("testadmin", "newpassword456")
	if err != nil {
		t.Fatalf("Login with new password failed: %v", err)
	}
}
