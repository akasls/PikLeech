package offline

import (
	"context"
	"fmt"
	"log"
	"time"
)

type CleanupResult struct {
	CleanedTasksCount int   `json:"cleaned_tasks_count"`
	ReclaimedBytes    int64 `json:"reclaimed_bytes"`
}

// CleanExpiredTasks cleans up offline tasks completed more than retentionDays ago
func (s *Service) CleanExpiredTasks(ctx context.Context, retentionDays int) (*CleanupResult, error) {
	if retentionDays <= 0 {
		retentionDays = 7
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, pikpak_file_id, file_name, completed_at
		FROM offline_tasks
		WHERE status = 'COMPLETE' AND completed_at IS NOT NULL AND completed_at <= ?
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired tasks: %w", err)
	}
	defer rows.Close()

	type expiredTask struct {
		id           string
		accountID    int64
		pikpakFileID string
		fileName     string
		completedAt  time.Time
	}

	var tasks []expiredTask
	for rows.Next() {
		var t expiredTask
		if err := rows.Scan(&t.id, &t.accountID, &t.pikpakFileID, &t.fileName, &t.completedAt); err == nil {
			tasks = append(tasks, t)
		}
	}

	res := &CleanupResult{}
	affectedAccounts := make(map[int64]bool)

	for _, t := range tasks {
		client, err := s.accountService.GetClient(t.accountID)
		if err != nil {
			log.Printf("[CLEANER] Account %d client error during cleanup of task %s: %v", t.accountID, t.id, err)
			continue
		}

		if t.pikpakFileID != "" {
			// Permanently delete remote file from PikPak
			_ = client.DeleteFiles(ctx, []string{t.pikpakFileID})
			// Empty recycle bin to release storage quota
			_ = client.EmptyTrash(ctx)

			// Remove cached file records
			_, _ = s.db.ExecContext(ctx, `DELETE FROM file_cache WHERE account_id = ? AND pikpak_file_id = ?`, t.accountID, t.pikpakFileID)
		}

		// Update offline task record status to CLEANED
		now := time.Now().UTC()
		_, err = s.db.ExecContext(ctx, `
			UPDATE offline_tasks 
			SET status = 'CLEANED', updated_at = ? 
			WHERE id = ?
		`, now, t.id)
		if err == nil {
			res.CleanedTasksCount++
			affectedAccounts[t.accountID] = true
		}
	}

	// Sync storage for all affected accounts to refresh quota
	for accID := range affectedAccounts {
		_ = s.accountService.SyncAccountStorage(ctx, accID)
	}

	log.Printf("[CLEANER] Cleaned %d expired tasks completed before %s", res.CleanedTasksCount, cutoff.Format("2006-01-02 15:04:05"))
	return res, nil
}
