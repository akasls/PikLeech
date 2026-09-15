package auth

import (
	"context"
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
	token, role, err := svc.Login("testadmin", "password123")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	if token == "" {
		t.Fatalf("Expected non-empty token")
	}
	if role != "admin" {
		t.Errorf("Expected role 'admin', got '%s'", role)
	}

	// Test validate token
	claims, err := svc.ValidateSessionToken(token)
	if err != nil {
		t.Fatalf("ValidateSessionToken failed: %v", err)
	}
	if claims.Username != "testadmin" {
		t.Errorf("Expected claims username testadmin, got %s", claims.Username)
	}
	if claims.Role != "admin" {
		t.Errorf("Expected claims role admin, got %s", claims.Role)
	}

	// Test wrong password
	_, _, err = svc.Login("testadmin", "wrongpassword")
	if err == nil {
		t.Errorf("Expected login to fail with wrong password")
	}

	// Test change password
	err = svc.ChangePassword(claims.UserID, "password123", "newpassword456")
	if err != nil {
		t.Fatalf("ChangePassword failed: %v", err)
	}

	// Verify old password fails
	_, _, err = svc.Login("testadmin", "password123")
	if err == nil {
		t.Errorf("Expected old password to fail")
	}

	// Verify new password succeeds
	_, _, err = svc.Login("testadmin", "newpassword456")
	if err != nil {
		t.Fatalf("Login with new password failed: %v", err)
	}

	// Test user management: Create a normal user
	ctx := context.Background()
	newUser, err := svc.CreateUser(ctx, "normaluser", "normalpass123", "user")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if newUser.Username != "normaluser" || newUser.Role != "user" {
		t.Errorf("Unexpected created user: %+v", newUser)
	}

	// Test login with normal user
	uToken, uRole, err := svc.Login("normaluser", "normalpass123")
	if err != nil {
		t.Fatalf("Normal user login failed: %v", err)
	}
	if uRole != "user" {
		t.Errorf("Expected normaluser role to be user, got %s", uRole)
	}
	uClaims, err := svc.ValidateSessionToken(uToken)
	if err != nil || uClaims.Role != "user" {
		t.Errorf("Invalid session claims for normal user: %+v, err: %v", uClaims, err)
	}

	// Test Admin reset password for normal user
	err = svc.AdminResetPassword(ctx, newUser.ID, "brandnewpass999")
	if err != nil {
		t.Fatalf("AdminResetPassword failed: %v", err)
	}
	_, _, err = svc.Login("normaluser", "brandnewpass999")
	if err != nil {
		t.Fatalf("Login with reset password failed: %v", err)
	}

	// Test delete user
	err = svc.DeleteUser(ctx, claims.UserID, newUser.ID)
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	// Cannot delete self
	err = svc.DeleteUser(ctx, claims.UserID, claims.UserID)
	if err == nil {
		t.Errorf("Expected error when deleting self")
	}
}
