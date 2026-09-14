package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type APIKey struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	Permissions string     `json:"permissions"`
	IsEnabled   bool       `json:"is_enabled"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`

	// Only returned on initial creation
	FullKey string `json:"full_key,omitempty"`
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// GenerateAPIKey creates a new API key, hashes it, and stores the hash in SQLite
func (s *Service) GenerateAPIKey(name, permissions string) (*APIKey, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("API key name is required")
	}
	if permissions == "" {
		permissions = "offline:create,offline:read"
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, err
	}
	rawKey := "pk_" + hex.EncodeToString(randomBytes)
	prefix := rawKey[:8]

	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	now := time.Now().UTC()
	res, err := s.db.Exec(`
		INSERT INTO api_keys (name, key_hash, key_prefix, permissions, is_enabled, created_at)
		VALUES (?, ?, ?, ?, 1, ?)
	`, name, keyHash, prefix, permissions, now)
	if err != nil {
		return nil, fmt.Errorf("failed to save api key: %w", err)
	}

	id, _ := res.LastInsertId()
	return &APIKey{
		ID:          id,
		Name:        name,
		KeyPrefix:   prefix,
		Permissions: permissions,
		IsEnabled:   true,
		CreatedAt:   now,
		FullKey:     rawKey,
	}, nil
}

func (s *Service) ListKeys() ([]*APIKey, error) {
	rows, err := s.db.Query(`
		SELECT id, name, key_prefix, permissions, is_enabled, last_used_at, created_at
		FROM api_keys ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*APIKey
	for rows.Next() {
		var k APIKey
		var lastUsed sql.NullTime
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Permissions, &k.IsEnabled, &lastUsed, &k.CreatedAt); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			k.LastUsedAt = &lastUsed.Time
		}
		list = append(list, &k)
	}
	return list, nil
}

func (s *Service) DeleteKey(id int64) error {
	_, err := s.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	return err
}

func (s *Service) ToggleKey(id int64, enabled bool) error {
	_, err := s.db.Exec("UPDATE api_keys SET is_enabled = ? WHERE id = ?", enabled, id)
	return err
}

// ValidateKey verifies raw key against database hash and permission
func (s *Service) ValidateKey(rawKey, requiredPerm string) (*APIKey, error) {
	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	row := s.db.QueryRow(`
		SELECT id, name, key_prefix, permissions, is_enabled, last_used_at, created_at
		FROM api_keys WHERE key_hash = ?
	`, keyHash)

	var k APIKey
	var lastUsed sql.NullTime
	if err := row.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Permissions, &k.IsEnabled, &lastUsed, &k.CreatedAt); err != nil {
		return nil, errors.New("invalid API key")
	}

	if !k.IsEnabled {
		return nil, errors.New("API key is disabled")
	}

	if requiredPerm != "" && !strings.Contains(k.Permissions, requiredPerm) && !strings.Contains(k.Permissions, "*") {
		return nil, fmt.Errorf("API key lacks required permission: %s", requiredPerm)
	}

	now := time.Now().UTC()
	go func() {
		_, _ = s.db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", now, k.ID)
	}()

	return &k, nil
}

// KeyAuthMiddleware enforces Bearer API key authentication
func (s *Service) KeyAuthMiddleware(requiredPerm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid Bearer API Key in Authorization header"})
			return
		}

		rawKey := strings.TrimPrefix(authHeader, "Bearer ")
		k, err := s.ValidateKey(rawKey, requiredPerm)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("api_key_id", k.ID)
		c.Set("api_key_name", k.Name)
		c.Next()
	}
}
