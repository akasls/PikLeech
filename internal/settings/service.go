package settings

import (
	"context"
	"database/sql"
	"strconv"
	"sync"
)

// SystemSettings holds global system automation and storage policy configurations
type SystemSettings struct {
	StorageBalancingEnabled bool `json:"storage_balancing_enabled"`
	StorageMinFreeGB        int  `json:"storage_min_free_gb"`
	AutoCleanupEnabled      bool `json:"auto_cleanup_enabled"`
	AutoCleanupDays         int  `json:"auto_cleanup_days"`
}

type Service struct {
	db       *sql.DB
	mu       sync.RWMutex
	cached   *SystemSettings
}

func NewService(db *sql.DB) *Service {
	s := &Service{db: db}
	_ = s.ensureDefaults(context.Background())
	return s
}

func (s *Service) ensureDefaults(ctx context.Context) error {
	defaults := map[string]string{
		"storage_balancing_enabled": "true",
		"storage_min_free_gb":        "10",
		"auto_cleanup_enabled":      "false",
		"auto_cleanup_days":         "7",
	}

	for k, v := range defaults {
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO settings (key, value) VALUES (?, ?)
			ON CONFLICT(key) DO NOTHING
		`, k, v)
		if err != nil {
			return err
		}
	}

	_, _ = s.GetSettings(ctx)
	return nil
}

// GetSettings retrieves current global settings
func (s *Service) GetSettings(ctx context.Context) (*SystemSettings, error) {
	s.mu.RLock()
	if s.cached != nil {
		copy := *s.cached
		s.mu.RUnlock()
		return &copy, nil
	}
	s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settingsMap := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err == nil {
			settingsMap[k] = v
		}
	}

	cfg := &SystemSettings{
		StorageBalancingEnabled: settingsMap["storage_balancing_enabled"] != "false",
		StorageMinFreeGB:        10,
		AutoCleanupEnabled:      settingsMap["auto_cleanup_enabled"] == "true",
		AutoCleanupDays:         7,
	}

	if val, ok := settingsMap["storage_min_free_gb"]; ok {
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			cfg.StorageMinFreeGB = n
		}
	}

	if val, ok := settingsMap["auto_cleanup_days"]; ok {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			cfg.AutoCleanupDays = n
		}
	}

	s.mu.Lock()
	s.cached = cfg
	s.mu.Unlock()

	copy := *cfg
	return &copy, nil
}

// UpdateSettings persists new global settings to database
func (s *Service) UpdateSettings(ctx context.Context, cfg SystemSettings) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	pairs := map[string]string{
		"storage_balancing_enabled": strconv.FormatBool(cfg.StorageBalancingEnabled),
		"storage_min_free_gb":        strconv.Itoa(cfg.StorageMinFreeGB),
		"auto_cleanup_enabled":      strconv.FormatBool(cfg.AutoCleanupEnabled),
		"auto_cleanup_days":         strconv.Itoa(cfg.AutoCleanupDays),
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for k, v := range pairs {
		if _, err := stmt.ExecContext(ctx, k, v); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	s.mu.Lock()
	s.cached = &cfg
	s.mu.Unlock()

	return nil
}
