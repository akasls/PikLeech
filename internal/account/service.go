package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"pikpak-manager/internal/crypto"
	"pikpak-manager/internal/pikpak"
)

type Service struct {
	db        *sql.DB
	appSecret string

	clientsMu sync.RWMutex
	clients   map[int64]*pikpak.Client
}

func NewService(db *sql.DB, appSecret string) *Service {
	return &Service{
		db:        db,
		appSecret: appSecret,
		clients:   make(map[int64]*pikpak.Client),
	}
}

func (s *Service) CreateAccount(ctx context.Context, req CreateAccountReq) (*Account, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("account name cannot be empty")
	}

	var pwdEnc, refreshEnc string
	var err error

	if req.Password != "" {
		pwdEnc, err = crypto.Encrypt(req.Password, s.appSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt password: %w", err)
		}
	}

	if req.RefreshToken != "" {
		refreshEnc, err = crypto.Encrypt(req.RefreshToken, s.appSecret)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	if req.Priority <= 0 {
		req.Priority = 10
	}

	now := time.Now().UTC()
	deviceID := pikpak.GenerateDeviceID(req.Username + req.Password)

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO pikpak_accounts (
			name, username, password_enc, refresh_token_enc, access_token_enc,
			user_id, device_id, captcha_token, proxy_url, priority, is_enabled,
			status, daily_task_count, total_files, used_space, total_space,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.Username, pwdEnc, refreshEnc, "", "", deviceID, "",
		req.ProxyURL, req.Priority, req.IsEnabled, "HEALTHY", 0, 0, 0, 0, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert account into database: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	log.Printf("[ACCOUNT] Added new PikPak account %d (%s) with proxy: %s", id, req.Name, req.ProxyURL)
	return s.GetAccount(id)
}

func (s *Service) UpdateAccount(ctx context.Context, id int64, req UpdateAccountReq) (*Account, error) {
	acc, err := s.GetAccount(id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		acc.Name = *req.Name
	}
	if req.Username != nil {
		acc.Username = *req.Username
	}
	if req.Password != nil && *req.Password != "" {
		enc, err := crypto.Encrypt(*req.Password, s.appSecret)
		if err != nil {
			return nil, err
		}
		acc.PasswordEnc = enc
	}
	if req.RefreshToken != nil && *req.RefreshToken != "" {
		enc, err := crypto.Encrypt(*req.RefreshToken, s.appSecret)
		if err != nil {
			return nil, err
		}
		acc.RefreshTokenEnc = enc
	}
	if req.ProxyURL != nil {
		acc.ProxyURL = *req.ProxyURL
	}
	if req.Priority != nil {
		acc.Priority = *req.Priority
	}
	if req.IsEnabled != nil {
		acc.IsEnabled = *req.IsEnabled
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE pikpak_accounts SET
			name = ?, username = ?, password_enc = ?, refresh_token_enc = ?,
			proxy_url = ?, priority = ?, is_enabled = ?, updated_at = ?
		WHERE id = ?
	`, acc.Name, acc.Username, acc.PasswordEnc, acc.RefreshTokenEnc,
		acc.ProxyURL, acc.Priority, acc.IsEnabled, now, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update account: %w", err)
	}

	// Invalidate cached client so changes (credentials, tokens, proxy) apply immediately
	s.clientsMu.Lock()
	delete(s.clients, id)
	s.clientsMu.Unlock()
	log.Printf("[ACCOUNT] Account %d (%s) updated, reloaded client cache", id, acc.Name)

	return s.GetAccount(id)
}

func (s *Service) DeleteAccount(id int64) error {
	s.clientsMu.Lock()
	delete(s.clients, id)
	s.clientsMu.Unlock()

	_, err := s.db.Exec("DELETE FROM pikpak_accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete account %d: %w", id, err)
	}

	log.Printf("[ACCOUNT] Deleted account %d", id)
	return nil
}

func (s *Service) GetAccount(id int64) (*Account, error) {
	row := s.db.QueryRow(`
		SELECT id, name, username, password_enc, refresh_token_enc, access_token_enc,
		       user_id, device_id, captcha_token, proxy_url, priority, is_enabled,
		       status, quota_exhausted_at, cooldown_until, daily_task_count, last_task_date,
		       total_files, used_space, total_space, last_login_at, last_success_at,
		       last_error, created_at, updated_at
		FROM pikpak_accounts WHERE id = ?
	`, id)

	acc, err := s.scanAccount(row)
	if err != nil {
		return nil, err
	}
	return acc, nil
}

func (s *Service) ListAccounts() ([]*Account, error) {
	rows, err := s.db.Query(`
		SELECT id, name, username, password_enc, refresh_token_enc, access_token_enc,
		       user_id, device_id, captcha_token, proxy_url, priority, is_enabled,
		       status, quota_exhausted_at, cooldown_until, daily_task_count, last_task_date,
		       total_files, used_space, total_space, last_login_at, last_success_at,
		       last_error, created_at, updated_at
		FROM pikpak_accounts ORDER BY priority DESC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*Account
	for rows.Next() {
		acc, err := s.scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, acc)
	}

	return accounts, nil
}

func (s *Service) scanAccount(scanner interface {
	Scan(dest ...interface{}) error
}) (*Account, error) {
	var acc Account
	var pwdEnc, refreshEnc, accessEnc sql.NullString
	var lastTaskDate, lastError sql.NullString
	var quotaExhaustedAt, cooldownUntil, lastLoginAt, lastSuccessAt sql.NullTime

	err := scanner.Scan(
		&acc.ID, &acc.Name, &acc.Username, &pwdEnc, &refreshEnc, &accessEnc,
		&acc.UserID, &acc.DeviceID, &acc.CaptchaToken, &acc.ProxyURL, &acc.Priority, &acc.IsEnabled,
		&acc.Status, &quotaExhaustedAt, &cooldownUntil, &acc.DailyTaskCount, &lastTaskDate,
		&acc.TotalFiles, &acc.UsedSpace, &acc.TotalSpace, &lastLoginAt, &lastSuccessAt,
		&lastError, &acc.CreatedAt, &acc.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	acc.PasswordEnc = pwdEnc.String
	acc.RefreshTokenEnc = refreshEnc.String
	acc.AccessTokenEnc = accessEnc.String
	acc.HasPassword = acc.PasswordEnc != ""
	acc.HasRefreshToken = acc.RefreshTokenEnc != ""
	acc.LastTaskDate = lastTaskDate.String
	acc.LastError = lastError.String

	if quotaExhaustedAt.Valid {
		acc.QuotaExhaustedAt = &quotaExhaustedAt.Time
	}
	if cooldownUntil.Valid {
		acc.CooldownUntil = &cooldownUntil.Time
	}
	if lastLoginAt.Valid {
		acc.LastLoginAt = &lastLoginAt.Time
	}
	if lastSuccessAt.Valid {
		acc.LastSuccessAt = &lastSuccessAt.Time
	}

	return &acc, nil
}

// GetClient retrieves or initializes an isolated PikPak Client for the account
func (s *Service) GetClient(id int64) (*pikpak.Client, error) {
	s.clientsMu.RLock()
	client, exists := s.clients[id]
	s.clientsMu.RUnlock()
	if exists {
		return client, nil
	}

	acc, err := s.GetAccount(id)
	if err != nil {
		return nil, err
	}

	var password, refreshToken, accessToken string
	if acc.PasswordEnc != "" {
		password, _ = crypto.Decrypt(acc.PasswordEnc, s.appSecret)
	}
	if acc.RefreshTokenEnc != "" {
		refreshToken, _ = crypto.Decrypt(acc.RefreshTokenEnc, s.appSecret)
	}
	if acc.AccessTokenEnc != "" {
		accessToken, _ = crypto.Decrypt(acc.AccessTokenEnc, s.appSecret)
	}

	client, err = pikpak.NewClient(pikpak.ClientOptions{
		AccountID:    acc.ID,
		AccountName:  acc.Name,
		Username:     acc.Username,
		Password:     password,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		DeviceID:     acc.DeviceID,
		UserID:       acc.UserID,
		CaptchaToken: acc.CaptchaToken,
		ProxyURL:     acc.ProxyURL,
		OnTokenUpdated: func(newAccess, newRefresh string) {
			_ = s.UpdateTokens(acc.ID, newAccess, newRefresh)
		},
	})
	if err != nil {
		return nil, err
	}

	s.clientsMu.Lock()
	s.clients[id] = client
	s.clientsMu.Unlock()

	return client, nil
}

func (s *Service) UpdateTokens(id int64, accessToken, refreshToken string) error {
	accessEnc, _ := crypto.Encrypt(accessToken, s.appSecret)
	refreshEnc, _ := crypto.Encrypt(refreshToken, s.appSecret)
	now := time.Now().UTC()

	_, err := s.db.Exec(`
		UPDATE pikpak_accounts SET
			access_token_enc = ?, refresh_token_enc = ?,
			last_success_at = ?, updated_at = ?
		WHERE id = ?
	`, accessEnc, refreshEnc, now, now, id)
	return err
}

func (s *Service) UpdateStatus(id int64, status string, errMsg string) error {
	now := time.Now().UTC()
	query := `
		UPDATE pikpak_accounts SET
			status = ?, last_error = ?, updated_at = ?
	`
	args := []interface{}{status, errMsg, now}
	if status == "QUOTA_EXHAUSTED" {
		today := now.Format("2006-01-02")
		query += `, quota_exhausted_at = ?, last_task_date = ?`
		args = append(args, now, today)
	}
	query += ` WHERE id = ?`
	args = append(args, id)

	_, err := s.db.Exec(query, args...)
	return err
}

func (s *Service) ResetQuota(id int64) error {
	now := time.Now().UTC()
	_, err := s.db.Exec(`
		UPDATE pikpak_accounts SET
			status = 'HEALTHY', quota_exhausted_at = NULL,
			daily_task_count = 0, updated_at = ?
		WHERE id = ?
	`, now, id)
	return err
}

func (s *Service) IncrementDailyTaskCount(id int64) error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := s.db.Exec(`
		UPDATE pikpak_accounts SET
			daily_task_count = CASE WHEN last_task_date = ? THEN daily_task_count + 1 ELSE 1 END,
			last_task_date = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, today, today, id)
	return err
}

// AutoResetDailyQuotas resets quota exhausted state and daily task counts if a new day has arrived (UTC 00:00)
func (s *Service) AutoResetDailyQuotas() error {
	today := time.Now().UTC().Format("2006-01-02")
	_, err := s.db.Exec(`
		UPDATE pikpak_accounts SET
			status = CASE WHEN status = 'QUOTA_EXHAUSTED' THEN 'HEALTHY' ELSE status END,
			quota_exhausted_at = CASE WHEN status = 'QUOTA_EXHAUSTED' THEN NULL ELSE quota_exhausted_at END,
			daily_task_count = 0,
			last_task_date = ?
		WHERE (status = 'QUOTA_EXHAUSTED' AND (last_task_date IS NULL OR last_task_date != ?))
		   OR (last_task_date IS NOT NULL AND last_task_date != ?)
	`, today, today, today)
	return err
}

// TestAccount tests login, token, and storage for an account
func (s *Service) TestAccount(ctx context.Context, id int64) (*AccountTestResult, error) {
	client, err := s.GetClient(id)
	if err != nil {
		return &AccountTestResult{Success: false, Error: err.Error()}, nil
	}

	start := time.Now()

	// Try RefreshToken or Login
	err = client.RefreshToken(ctx)
	if err != nil {
		err = client.Login(ctx)
	}

	if err != nil {
		s.UpdateStatus(id, "AUTH_FAILED", err.Error())
		return &AccountTestResult{
			Success: false,
			Status:  "AUTH_FAILED",
			Error:   err.Error(),
		}, nil
	}

	// Fetch Storage
	about, err := client.GetStorageAbout(ctx)
	latency := time.Since(start).Milliseconds()

	var used, total int64
	if err == nil && about != nil {
		used, _ = strconv.ParseInt(about.Quota.Usage, 10, 64)
		total, _ = strconv.ParseInt(about.Quota.Limit, 10, 64)

		_, _ = s.db.Exec(`UPDATE pikpak_accounts SET used_space = ?, total_space = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, used, total, id)
	}

	s.UpdateStatus(id, "HEALTHY", "")
	return &AccountTestResult{
		Success:    true,
		Status:     "HEALTHY",
		UsedSpace:  used,
		TotalSpace: total,
		LatencyMs:  latency,
	}, nil
}

// TestProxy tests an arbitrary proxy before saving
func (s *Service) TestProxy(ctx context.Context, proxyURL string) (*ProxyTestResult, error) {
	client, err := pikpak.NewClient(pikpak.ClientOptions{
		AccountID:   0,
		AccountName: "ProxyTest",
		ProxyURL:    proxyURL,
	})
	if err != nil {
		return &ProxyTestResult{Success: false, Error: err.Error()}, nil
	}

	ip, latency, err := client.TestProxy(ctx)
	if err != nil {
		return &ProxyTestResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return &ProxyTestResult{
		Success:   true,
		LatencyMs: latency,
		EgressIP:  ip,
	}, nil
}
