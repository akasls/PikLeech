package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"

	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db/migrations"
)

var DB *sql.DB

// InitDB initializes SQLite, configures WAL mode, runs migrations, and seeds admin.
func InitDB(cfg *config.Config) (*sql.DB, error) {
	// DSN with pragmas
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)", cfg.DBPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", cfg.DBPath, err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	// Run migrations
	if err := migrations.Run(db); err != nil {
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Initialize admin user if empty
	if err := seedAdmin(db, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return nil, fmt.Errorf("failed to seed admin user: %w", err)
	}

	DB = db
	return db, nil
}

func seedAdmin(db *sql.DB, username, password string) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		_, err = db.Exec("INSERT INTO users (username, password_hash, role, created_at, updated_at) VALUES (?, ?, 'admin', ?, ?)",
			username, string(hash), now, now)
		if err != nil {
			return err
		}
		log.Printf("[INIT] Created default admin user: %s (password hashed)", username)
	}

	return nil
}
