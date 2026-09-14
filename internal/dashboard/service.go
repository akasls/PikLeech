package dashboard

import (
	"database/sql"
	"time"
)

type Stats struct {
	TotalAccounts       int   `json:"total_accounts"`
	HealthyAccounts     int   `json:"healthy_accounts"`
	QuotaExhaustedCount int   `json:"quota_exhausted_count"`
	AuthFailedCount     int   `json:"auth_failed_count"`
	ProxyFailedCount    int   `json:"proxy_failed_count"`
	CooldownCount       int   `json:"cooldown_count"`
	DisabledCount       int   `json:"disabled_count"`
	TotalTasks          int   `json:"total_tasks"`
	RunningTasks        int   `json:"running_tasks"`
	CompletedTasks      int   `json:"completed_tasks"`
	FailedTasks         int   `json:"failed_tasks"`
	TotalFiles          int   `json:"total_files"`
	UsedSpace           int64 `json:"used_space"`
	TotalSpace          int64 `json:"total_space"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) GetStats() (*Stats, error) {
	var st Stats

	// Accounts statistics
	rows, err := s.db.Query(`
		SELECT status, is_enabled, cooldown_until, used_space, total_space
		FROM pikpak_accounts
	`)
	if err == nil {
		defer rows.Close()
		now := time.Now()
		for rows.Next() {
			var status string
			var isEnabled bool
			var cooldown sql.NullTime
			var used, total int64

			if err := rows.Scan(&status, &isEnabled, &cooldown, &used, &total); err == nil {
				st.TotalAccounts++
				st.UsedSpace += used
				st.TotalSpace += total

				if !isEnabled {
					st.DisabledCount++
					continue
				}

				if cooldown.Valid && now.Before(cooldown.Time) {
					st.CooldownCount++
				}

				switch status {
				case "HEALTHY":
					st.HealthyAccounts++
				case "QUOTA_EXHAUSTED":
					st.QuotaExhaustedCount++
				case "AUTH_FAILED":
					st.AuthFailedCount++
				case "PROXY_FAILED":
					st.ProxyFailedCount++
				default:
					st.HealthyAccounts++
				}
			}
		}
	}

	// Tasks statistics
	taskRows, err := s.db.Query("SELECT status FROM offline_tasks")
	if err == nil {
		defer taskRows.Close()
		for taskRows.Next() {
			var status string
			if err := taskRows.Scan(&status); err == nil {
				st.TotalTasks++
				switch status {
				case "RUNNING", "PENDING":
					st.RunningTasks++
				case "COMPLETE":
					st.CompletedTasks++
				case "ERROR":
					st.FailedTasks++
				}
			}
		}
	}

	// File count from cache
	_ = s.db.QueryRow("SELECT COUNT(*) FROM file_cache WHERE kind != 'drive#folder'").Scan(&st.TotalFiles)

	return &st, nil
}
