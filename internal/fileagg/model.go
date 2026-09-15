package fileagg

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type VirtualFile struct {
	VirtualID     string    `json:"virtual_id"`
	AccountID     int64     `json:"account_id"`
	AccountName   string    `json:"account_name"`
	PikPakFileID  string    `json:"pikpak_file_id"`
	ParentID      string    `json:"parent_id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	MimeType      string    `json:"mime_type"`
	Kind          string    `json:"kind"` // "drive#file" or "drive#folder"
	IsFolder      bool      `json:"is_folder"`
	IsVideo       bool      `json:"is_video"`
	ThumbnailLink string    `json:"thumbnail_link"`
	UserID        int64     `json:"user_id,omitempty"`
	CreatedTime   time.Time `json:"created_time"`
	ModifiedTime  time.Time `json:"modified_time"`
}

type BatchDeleteReq struct {
	VirtualIDs []string `json:"virtual_ids"`
	Permanent  bool     `json:"permanent"`
}

type BatchDeleteResultItem struct {
	VirtualID string `json:"virtual_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

type BatchDeleteResponse struct {
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Failed  int                     `json:"failed"`
	Items   []BatchDeleteResultItem `json:"items"`
}

// EncodeVirtualID creates a URL-safe unique virtual ID
func EncodeVirtualID(accountID int64, fileID string) string {
	raw := fmt.Sprintf("%d:%s", accountID, fileID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeVirtualID parses accountID and pikpak fileID from virtualID
func DecodeVirtualID(virtualID string) (int64, string, error) {
	bytes, err := base64.RawURLEncoding.DecodeString(virtualID)
	if err != nil {
		return 0, "", fmt.Errorf("invalid virtual_id encoding: %w", err)
	}

	parts := strings.SplitN(string(bytes), ":", 2)
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("invalid virtual_id format")
	}

	accID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid account ID in virtual_id: %w", err)
	}

	return accID, parts[1], nil
}

// IsVideoExtension checks if filename has a video extension
func IsVideoExtension(name string) bool {
	lower := strings.ToLower(name)
	exts := []string{".mp4", ".mkv", ".avi", ".mov", ".webm", ".m4v", ".ts", ".flv", ".rmvb", ".iso"}
	for _, ext := range exts {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
