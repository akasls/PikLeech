package offline

import (
	"context"
	"log"
	"sync"
	"time"
)

// StartPoller starts background task poller with responsive intervals and wake signals
func (s *Service) StartPoller(ctx context.Context, interval time.Duration) {
	if interval <= 0 || interval > 2*time.Second {
		interval = 2 * time.Second
	}

	pollCtx, cancel := context.WithCancel(ctx)
	s.pollerCancel = cancel

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-pollCtx.Done():
				return
			case <-ticker.C:
				s.pollActiveTasks(pollCtx)
			case <-s.wakeChan:
				s.pollActiveTasks(pollCtx)
			}
		}
	}()
}

// StopPoller stops background task poller
func (s *Service) StopPoller() {
	if s.pollerCancel != nil {
		s.pollerCancel()
	}
}

type activeTaskInfo struct {
	id           string
	accountID    int64
	pikpakTaskID string
}

// pollActiveTasks queries active tasks and synchronizes status from PikPak in parallel
func (s *Service) pollActiveTasks(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, pikpak_task_id
		FROM offline_tasks
		WHERE status IN ('RUNNING', 'PENDING') AND pikpak_task_id != ''
	`)
	if err != nil {
		return
	}
	defer rows.Close()

	accountTasks := make(map[int64][]activeTaskInfo)
	for rows.Next() {
		var t activeTaskInfo
		if err := rows.Scan(&t.id, &t.accountID, &t.pikpakTaskID); err == nil {
			accountTasks[t.accountID] = append(accountTasks[t.accountID], t)
		}
	}

	if len(accountTasks) == 0 {
		return
	}

	var wg sync.WaitGroup
	for accID, tasks := range accountTasks {
		wg.Add(1)
		go func(aID int64, tList []activeTaskInfo) {
			defer wg.Done()
			s.pollSingleAccountTasks(ctx, aID, tList)
		}(accID, tasks)
	}
	wg.Wait()
}

func (s *Service) pollSingleAccountTasks(ctx context.Context, accID int64, tasks []activeTaskInfo) {
	client, err := s.accountService.GetClient(accID)
	if err != nil {
		return
	}

	resp, err := client.ListOfflineTasks(ctx, "")
	if err != nil {
		return
	}

	taskMap := make(map[string]struct {
		phase    string
		progress int64
		fileID   string
		msg      string
	})

	for _, pt := range resp.Tasks {
		taskMap[pt.ID] = struct {
			phase    string
			progress int64
			fileID   string
			msg      string
		}{
			phase:    pt.Phase,
			progress: pt.Progress,
			fileID:   pt.FileID,
			msg:      pt.Message,
		}
	}

	now := time.Now().UTC()
	for _, t := range tasks {
		info, found := taskMap[t.pikpakTaskID]
		if !found {
			continue
		}

		switch info.phase {
		case "PHASE_TYPE_COMPLETE":
			_, _ = s.db.ExecContext(ctx, `
				UPDATE offline_tasks SET
					status = 'COMPLETE', progress = 100, pikpak_file_id = ?,
					completed_at = ?, updated_at = ?
				WHERE id = ?
			`, info.fileID, now, now, t.id)
			log.Printf("[POLLER] Offline task %s COMPLETED!", t.id)

		case "PHASE_TYPE_ERROR":
			errMsg := info.msg
			if errMsg == "" {
				errMsg = "PikPak offline task failed"
			}
			_, _ = s.db.ExecContext(ctx, `
				UPDATE offline_tasks SET
					status = 'ERROR', error_message = ?, updated_at = ?
				WHERE id = ?
			`, errMsg, now, t.id)
			log.Printf("[POLLER] Offline task %s ERROR: %s", t.id, errMsg)

		case "PHASE_TYPE_RUNNING":
			_, _ = s.db.ExecContext(ctx, `
				UPDATE offline_tasks SET
					status = 'RUNNING', progress = ?, updated_at = ?
				WHERE id = ?
			`, info.progress, now, t.id)
		}
	}
}
