package offline

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/crypto"
	"pikpak-manager/internal/scheduler"
)

type Task struct {
	ID           string     `json:"id"`
	SourceURL    string     `json:"source_url"`
	FileName     string     `json:"file_name"`
	AccountID    int64      `json:"account_id"`
	AccountName  string     `json:"account_name"`
	PikPakTaskID string     `json:"pikpak_task_id"`
	PikPakFileID string     `json:"pikpak_file_id"`
	Status       string     `json:"status"` // PENDING, RUNNING, COMPLETE, ERROR, CANCELLED
	Progress     int        `json:"progress"`
	ErrorMessage string     `json:"error_message,omitempty"`
	UserID       int64      `json:"user_id"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type SubmitTaskReq struct {
	URL        string   `json:"url"`
	URLs       []string `json:"urls"`
	Name       string   `json:"name"`
	FileSize   int64    `json:"file_size"`    // in bytes
	FileSizeGB float64  `json:"file_size_gb"` // in gigabytes
}

type TaskResult struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Success   bool   `json:"success"`
	Status    string `json:"status"`
	AccountID int64  `json:"account_id,omitempty"`
	Account   string `json:"account,omitempty"`
	Error     string `json:"error,omitempty"`
}

type BatchSubmitResponse struct {
	Success bool         `json:"success"`
	Results []TaskResult `json:"results"`
}

type Service struct {
	db             *sql.DB
	accountService *account.Service
	scheduler      *scheduler.AccountScheduler
	pollerCancel   context.CancelFunc
	wakeChan       chan struct{}
}

func NewService(db *sql.DB, accSvc *account.Service, sched *scheduler.AccountScheduler) *Service {
	return &Service{
		db:             db,
		accountService: accSvc,
		scheduler:      sched,
		wakeChan:       make(chan struct{}, 1),
	}
}

// WakePoller triggers an immediate check of active offline tasks
func (s *Service) WakePoller() {
	if s.wakeChan != nil {
		select {
		case s.wakeChan <- struct{}{}:
		default:
		}
	}
}

// SubmitSingleLink submits a single download link with idempotency check and user isolation
func (s *Service) SubmitSingleLink(ctx context.Context, downloadURL, name, idempotencyKey string, userID int64, username string, requiredBytes ...int64) (*TaskResult, error) {
	downloadURL = strings.TrimSpace(downloadURL)
	if downloadURL == "" {
		return nil, errors.New("download URL is empty")
	}

	var reqBytes int64
	if len(requiredBytes) > 0 {
		reqBytes = requiredBytes[0]
	}

	// 1. Check Idempotency
	if idempotencyKey != "" {
		var cachedResp string
		err := s.db.QueryRowContext(ctx, "SELECT response_json FROM idempotency_keys WHERE key = ?", idempotencyKey).Scan(&cachedResp)
		if err == nil && cachedResp != "" {
			var cached ResultPayload
			if err := json.Unmarshal([]byte(cachedResp), &cached); err == nil {
				return cached.TaskResult, nil
			}
		}
	}

	taskID := uuid.New().String()
	pikpakTask, usedAccount, err := s.scheduler.SubmitOfflineTask(ctx, downloadURL, name, username, reqBytes)
	if err != nil {
		return &TaskResult{
			ID:      taskID,
			URL:     crypto.MaskMagnet(downloadURL),
			Success: false,
			Status:  "ERROR",
			Error:   err.Error(),
		}, err
	}

	taskName := name
	if taskName == "" && pikpakTask.Name != "" {
		taskName = pikpakTask.Name
	}
	if taskName == "" {
		taskName = pikpakTask.FileName
	}

	if userID <= 0 {
		userID = 1
	}

	now := time.Now().UTC()
	initStatus := "RUNNING"
	initProgress := pikpakTask.Progress
	var completedAt sql.NullTime

	// Instant cloud matching (秒传) detection
	if pikpakTask.Phase == "PHASE_TYPE_COMPLETE" || pikpakTask.Progress >= 100 {
		initStatus = "COMPLETE"
		initProgress = 100
		completedAt = sql.NullTime{Time: now, Valid: true}
		log.Printf("[OFFLINE] Task %s matched instantly (秒传完成)!", taskID)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO offline_tasks (
			id, source_url, file_name, account_id, pikpak_task_id, pikpak_file_id,
			status, progress, error_message, user_id, created_at, updated_at, completed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, taskID, downloadURL, taskName, usedAccount.ID, pikpakTask.ID, pikpakTask.FileID,
		initStatus, initProgress, "", userID, now, now, completedAt)
	if err != nil {
		log.Printf("[OFFLINE] Failed to save offline task %s: %v", taskID, err)
	}

	// Wake poller immediately for high-speed tracking
	s.WakePoller()

	res := &TaskResult{
		ID:        taskID,
		URL:       downloadURL,
		Success:   true,
		Status:    initStatus,
		AccountID: usedAccount.ID,
		Account:   usedAccount.Name,
	}

	// Save idempotency key if provided
	if idempotencyKey != "" {
		jsonBytes, _ := json.Marshal(ResultPayload{TaskResult: res})
		_, _ = s.db.ExecContext(ctx, "INSERT OR REPLACE INTO idempotency_keys (key, response_json, created_at) VALUES (?, ?, ?)",
			idempotencyKey, string(jsonBytes), now)
	}

	return res, nil
}

type ResultPayload struct {
	TaskResult *TaskResult `json:"task_result"`
}

// SubmitBatchLinks submits multiple download links line by line
func (s *Service) SubmitBatchLinks(ctx context.Context, req SubmitTaskReq, idempotencyKey string, userID int64, username string) (*BatchSubmitResponse, error) {
	var urls []string
	if req.URL != "" {
		for _, line := range strings.Split(req.URL, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				urls = append(urls, line)
			}
		}
	}
	for _, u := range req.URLs {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}

	if len(urls) == 0 {
		return nil, errors.New("no valid download URLs provided")
	}

	reqBytes := req.FileSize
	if reqBytes == 0 && req.FileSizeGB > 0 {
		reqBytes = int64(req.FileSizeGB * 1024 * 1024 * 1024)
	}

	resp := &BatchSubmitResponse{
		Success: true,
		Results: make([]TaskResult, 0, len(urls)),
	}

	for i, u := range urls {
		subKey := ""
		if idempotencyKey != "" {
			subKey = fmt.Sprintf("%s:%d", idempotencyKey, i)
		}

		res, err := s.SubmitSingleLink(ctx, u, req.Name, subKey, userID, username, reqBytes)
		if err != nil {
			resp.Results = append(resp.Results, TaskResult{
				URL:     u,
				Success: false,
				Status:  "ERROR",
				Error:   err.Error(),
			})
		} else {
			resp.Results = append(resp.Results, *res)
		}
	}

	return resp, nil
}

// GetTask gets a task by ID
func (s *Service) GetTask(ctx context.Context, id string) (*Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT t.id, t.source_url, t.file_name, t.account_id, a.name,
		       t.pikpak_task_id, t.pikpak_file_id, t.status, t.progress,
		       t.error_message, COALESCE(t.user_id, 1), t.created_at, t.updated_at, t.completed_at
		FROM offline_tasks t
		LEFT JOIN pikpak_accounts a ON t.account_id = a.id
		WHERE t.id = ?
	`, id)

	return s.scanTask(row)
}

// ListTasks lists tasks with optional status filter, user filtering, and pagination
func (s *Service) ListTasks(ctx context.Context, status string, userID int64, limit, offset int) ([]*Task, int, error) {
	if limit <= 0 {
		limit = 50
	}

	countQuery := "SELECT COUNT(*) FROM offline_tasks t"
	query := `
		SELECT t.id, t.source_url, t.file_name, t.account_id, COALESCE(a.name, 'Unknown'),
		       t.pikpak_task_id, t.pikpak_file_id, t.status, t.progress,
		       t.error_message, COALESCE(t.user_id, 1), t.created_at, t.updated_at, t.completed_at
		FROM offline_tasks t
		LEFT JOIN pikpak_accounts a ON t.account_id = a.id
	`

	var conditions []string
	var args []interface{}
	if status != "" {
		conditions = append(conditions, "t.status = ?")
		args = append(args, status)
	}
	if userID > 0 {
		conditions = append(conditions, "t.user_id = ?")
		args = append(args, userID)
	}

	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		countQuery += whereClause
		query += whereClause
	}

	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query += " ORDER BY t.created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tasks []*Task
	for rows.Next() {
		t, err := s.scanTask(rows)
		if err != nil {
			return nil, 0, err
		}
		tasks = append(tasks, t)
	}

	return tasks, total, nil
}

func (s *Service) scanTask(scanner interface {
	Scan(dest ...interface{}) error
}) (*Task, error) {
	var t Task
	var fileName, accName, ptID, pfID, errMsg sql.NullString
	var completedAt sql.NullTime

	err := scanner.Scan(
		&t.ID, &t.SourceURL, &fileName, &t.AccountID, &accName,
		&ptID, &pfID, &t.Status, &t.Progress,
		&errMsg, &t.UserID, &t.CreatedAt, &t.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}

	t.FileName = fileName.String
	t.AccountName = accName.String
	t.PikPakTaskID = ptID.String
	t.PikPakFileID = pfID.String
	t.ErrorMessage = errMsg.String
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}

	return &t, nil
}

// DeleteTask removes an offline task record and optionally removes it from PikPak
func (s *Service) DeleteTask(ctx context.Context, id string) error {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return err
	}

	if task.PikPakTaskID != "" {
		if client, err := s.accountService.GetClient(task.AccountID); err == nil {
			_ = client.DeleteOfflineTasks(ctx, []string{task.PikPakTaskID}, false)
		}
	}

	_, err = s.db.ExecContext(ctx, "DELETE FROM offline_tasks WHERE id = ?", id)
	return err
}

// CancelTask cancels an active task
func (s *Service) CancelTask(ctx context.Context, id string) error {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return err
	}

	if task.PikPakTaskID != "" {
		if client, err := s.accountService.GetClient(task.AccountID); err == nil {
			_ = client.DeleteOfflineTasks(ctx, []string{task.PikPakTaskID}, false)
		}
	}

	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		UPDATE offline_tasks SET status = 'CANCELLED', updated_at = ? WHERE id = ?
	`, now, id)
	return err
}

// RetryTask resubmits a failed task
func (s *Service) RetryTask(ctx context.Context, id string, userID int64, username string) (*TaskResult, error) {
	task, err := s.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	uid := userID
	if uid <= 0 {
		uid = task.UserID
	}

	return s.SubmitSingleLink(ctx, task.SourceURL, task.FileName, "", uid, username)
}
