package audit

import (
	"database/sql"
	"time"
)

type AuditLog struct {
	ID        int64     `json:"id"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	Status    string    `json:"status"`
	Operator  string    `json:"operator"`
	CreatedAt time.Time `json:"created_at"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Record(action, target, details, status, operator string) {
	now := time.Now().UTC()
	go func() {
		_, _ = s.db.Exec(`
			INSERT INTO audit_logs (action, target, details, status, operator, created_at)
			VALUES (?, ?, ?, ?, ?, ?)
		`, action, target, details, status, operator, now)
	}()
}

func (s *Service) ListLogs(limit, offset int) ([]AuditLog, int, error) {
	if limit <= 0 {
		limit = 50
	}

	var total int
	err := s.db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.db.Query(`
		SELECT id, action, target, details, status, operator, created_at
		FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var det sql.NullString
		if err := rows.Scan(&l.ID, &l.Action, &l.Target, &det, &l.Status, &l.Operator, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		l.Details = det.String
		logs = append(logs, l)
	}

	return logs, total, nil
}
