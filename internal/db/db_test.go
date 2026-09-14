package db

import (
	"os"
	"path/filepath"
	"testing"

	"pikpak-manager/internal/config"
)

func TestDB_InitAndMigrations(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_db_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AdminUsername: "testadmin",
		AdminPassword: "testpassword123",
	}

	database, err := InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	// Check that tables exist
	var userCount int
	err = database.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'testadmin'").Scan(&userCount)
	if err != nil {
		t.Fatalf("Query users failed: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("Expected 1 admin user, found %d", userCount)
	}

	// Check schema_migrations
	var migrationCount int
	err = database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount)
	if err != nil {
		t.Fatalf("Query schema_migrations failed: %v", err)
	}
	if migrationCount == 0 {
		t.Fatalf("Expected migrations to be applied")
	}
}
