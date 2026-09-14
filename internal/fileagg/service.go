package fileagg

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/pikpak"
)

type Service struct {
	db             *sql.DB
	accountService *account.Service
}

func NewService(db *sql.DB, accSvc *account.Service) *Service {
	return &Service{
		db:             db,
		accountService: accSvc,
	}
}

// ListFiles lists files in unified root or specific folder
func (s *Service) ListFiles(ctx context.Context, parentVirtualID string, sortBy, sortOrder string) ([]VirtualFile, error) {
	if parentVirtualID == "" || parentVirtualID == "root" {
		return s.listUnifiedRoot(ctx, sortBy, sortOrder)
	}

	// Specific folder inside a specific account
	accID, parentFileID, err := DecodeVirtualID(parentVirtualID)
	if err != nil {
		return nil, err
	}

	acc, err := s.accountService.GetAccount(accID)
	if err != nil {
		return nil, fmt.Errorf("parent folder account %d not found: %w", accID, err)
	}

	client, err := s.accountService.GetClient(accID)
	if err != nil {
		return nil, err
	}

	resp, err := client.ListFiles(ctx, parentFileID, "", 200)
	if err != nil {
		return nil, err
	}

	var result []VirtualFile
	for _, f := range resp.Files {
		vf := s.convertToFile(f, acc.ID, acc.Name)
		result = append(result, vf)
		// Update cache asynchronously
		go s.cacheFile(vf)
	}

	s.sortFiles(result, sortBy, sortOrder)
	return result, nil
}

func (s *Service) listUnifiedRoot(ctx context.Context, sortBy, sortOrder string) ([]VirtualFile, error) {
	accounts, err := s.accountService.ListAccounts()
	if err != nil {
		return nil, err
	}

	var enabledAccounts []*account.Account
	for _, acc := range accounts {
		if acc.IsEnabled && acc.Status != "AUTH_FAILED" && acc.Status != "PROXY_FAILED" {
			enabledAccounts = append(enabledAccounts, acc)
		}
	}

	var mu sync.Mutex
	var allFiles []VirtualFile
	var wg sync.WaitGroup

	for _, acc := range enabledAccounts {
		wg.Add(1)
		go func(a *account.Account) {
			defer wg.Done()
			client, err := s.accountService.GetClient(a.ID)
			if err != nil {
				return
			}

			resp, err := client.ListFiles(ctx, "", "", 100)
			if err != nil {
				log.Printf("[FILEAGG] Error listing files for account %d (%s): %v", a.ID, a.Name, err)
				return
			}

			var converted []VirtualFile
			for _, f := range resp.Files {
				vf := s.convertToFile(f, a.ID, a.Name)
				converted = append(converted, vf)
				go s.cacheFile(vf)
			}

			mu.Lock()
			allFiles = append(allFiles, converted...)
			mu.Unlock()
		}(acc)
	}

	wg.Wait()
	s.sortFiles(allFiles, sortBy, sortOrder)
	return allFiles, nil
}

func (s *Service) convertToFile(f pikpak.FileItem, accID int64, accName string) VirtualFile {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	isFolder := f.Kind == "drive#folder"
	isVideo := !isFolder && (IsVideoExtension(f.Name) || strings.HasPrefix(f.MimeType, "video/"))

	return VirtualFile{
		VirtualID:     EncodeVirtualID(accID, f.ID),
		AccountID:     accID,
		AccountName:   accName,
		PikPakFileID:  f.ID,
		ParentID:      f.ParentID,
		Name:          f.Name,
		Size:          size,
		MimeType:      f.MimeType,
		Kind:          f.Kind,
		IsFolder:      isFolder,
		IsVideo:       isVideo,
		ThumbnailLink: f.ThumbnailLink,
		CreatedTime:   f.CreatedTime,
		ModifiedTime:  f.ModifiedTime,
	}
}

func (s *Service) cacheFile(vf VirtualFile) {
	now := time.Now().UTC()
	_, _ = s.db.Exec(`
		INSERT INTO file_cache (
			virtual_id, account_id, pikpak_file_id, parent_id,
			name, size, mime_type, kind, thumbnail_link,
			created_time, modified_time, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(virtual_id) DO UPDATE SET
			name = excluded.name,
			size = excluded.size,
			thumbnail_link = excluded.thumbnail_link,
			modified_time = excluded.modified_time,
			updated_at = excluded.updated_at
	`, vf.VirtualID, vf.AccountID, vf.PikPakFileID, vf.ParentID,
		vf.Name, vf.Size, vf.MimeType, vf.Kind, vf.ThumbnailLink,
		vf.CreatedTime, vf.ModifiedTime, now)
}

func (s *Service) sortFiles(files []VirtualFile, sortBy, sortOrder string) {
	desc := strings.ToLower(sortOrder) == "desc"

	sort.Slice(files, func(i, j int) bool {
		// Folders always appear first
		if files[i].IsFolder != files[j].IsFolder {
			return files[i].IsFolder
		}

		var comp int
		switch strings.ToLower(sortBy) {
		case "size":
			if files[i].Size < files[j].Size {
				comp = -1
			} else if files[i].Size > files[j].Size {
				comp = 1
			}
		case "modified", "time", "date":
			if files[i].ModifiedTime.Before(files[j].ModifiedTime) {
				comp = -1
			} else if files[i].ModifiedTime.After(files[j].ModifiedTime) {
				comp = 1
			}
		default: // "name"
			comp = strings.Compare(strings.ToLower(files[i].Name), strings.ToLower(files[j].Name))
		}

		if desc {
			return comp > 0
		}
		return comp < 0
	})
}

// SearchFiles searches cached files across all accounts by name
func (s *Service) SearchFiles(ctx context.Context, keyword string, accountID int64) ([]VirtualFile, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}

	query := `
		SELECT c.virtual_id, c.account_id, COALESCE(a.name, 'Unknown'),
		       c.pikpak_file_id, c.parent_id, c.name, c.size, c.mime_type,
		       c.kind, c.thumbnail_link, c.created_time, c.modified_time
		FROM file_cache c
		LEFT JOIN pikpak_accounts a ON c.account_id = a.id
		WHERE c.name LIKE ?
	`
	args := []interface{}{"%" + keyword + "%"}
	if accountID > 0 {
		query += " AND c.account_id = ?"
		args = append(args, accountID)
	}
	query += " ORDER BY (c.kind = 'drive#folder') DESC, c.modified_time DESC LIMIT 100"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []VirtualFile
	for rows.Next() {
		var vf VirtualFile
		var thumb sql.NullString
		var cTime, mTime sql.NullTime

		err := rows.Scan(
			&vf.VirtualID, &vf.AccountID, &vf.AccountName,
			&vf.PikPakFileID, &vf.ParentID, &vf.Name, &vf.Size, &vf.MimeType,
			&vf.Kind, &thumb, &cTime, &mTime,
		)
		if err != nil {
			return nil, err
		}

		vf.ThumbnailLink = thumb.String
		if cTime.Valid {
			vf.CreatedTime = cTime.Time
		}
		if mTime.Valid {
			vf.ModifiedTime = mTime.Time
		}
		vf.IsFolder = vf.Kind == "drive#folder"
		vf.IsVideo = !vf.IsFolder && (IsVideoExtension(vf.Name) || strings.HasPrefix(vf.MimeType, "video/"))

		result = append(result, vf)
	}

	return result, nil
}

// BatchDelete deletes multiple files across different accounts
func (s *Service) BatchDelete(ctx context.Context, req BatchDeleteReq) (*BatchDeleteResponse, error) {
	if len(req.VirtualIDs) == 0 {
		return &BatchDeleteResponse{Total: 0}, nil
	}

	type fileRef struct {
		virtualID string
		fileID    string
	}

	// Group files by accountID
	accountGroups := make(map[int64][]fileRef)
	var invalidIDs []string

	for _, vid := range req.VirtualIDs {
		accID, fileID, err := DecodeVirtualID(vid)
		if err != nil {
			invalidIDs = append(invalidIDs, vid)
			continue
		}
		accountGroups[accID] = append(accountGroups[accID], fileRef{virtualID: vid, fileID: fileID})
	}

	resp := &BatchDeleteResponse{
		Total: len(req.VirtualIDs),
		Items: make([]BatchDeleteResultItem, 0, len(req.VirtualIDs)),
	}

	for _, inv := range invalidIDs {
		resp.Failed++
		resp.Items = append(resp.Items, BatchDeleteResultItem{
			VirtualID: inv,
			Success:   false,
			Error:     "invalid virtual_id format",
		})
	}

	for accID, files := range accountGroups {
		client, err := s.accountService.GetClient(accID)
		if err != nil {
			for _, f := range files {
				resp.Failed++
				resp.Items = append(resp.Items, BatchDeleteResultItem{
					VirtualID: f.virtualID,
					Success:   false,
					Error:     fmt.Sprintf("account client error: %v", err),
				})
			}
			continue
		}

		fileIDs := make([]string, len(files))
		for i, f := range files {
			fileIDs[i] = f.fileID
		}

		var delErr error
		if req.Permanent {
			delErr = client.DeleteFiles(ctx, fileIDs)
		} else {
			delErr = client.TrashFiles(ctx, fileIDs)
		}

		if delErr != nil {
			log.Printf("[FILEAGG] Batch delete failed for account %d (%d files): %v", accID, len(fileIDs), delErr)
			for _, f := range files {
				resp.Failed++
				resp.Items = append(resp.Items, BatchDeleteResultItem{
					VirtualID: f.virtualID,
					Success:   false,
					Error:     delErr.Error(),
				})
			}
		} else {
			for _, f := range files {
				resp.Success++
				resp.Items = append(resp.Items, BatchDeleteResultItem{
					VirtualID: f.virtualID,
					Success:   true,
				})
				// Remove from cache
				_, _ = s.db.ExecContext(ctx, "DELETE FROM file_cache WHERE virtual_id = ?", f.virtualID)
			}
		}
	}

	return resp, nil
}

// GetVirtualFile gets metadata and playable links for a single file
func (s *Service) GetVirtualFile(ctx context.Context, virtualID string) (*VirtualFile, *pikpak.Client, error) {
	accID, fileID, err := DecodeVirtualID(virtualID)
	if err != nil {
		return nil, nil, err
	}

	acc, err := s.accountService.GetAccount(accID)
	if err != nil {
		return nil, nil, err
	}

	client, err := s.accountService.GetClient(accID)
	if err != nil {
		return nil, nil, err
	}

	fileItem, err := client.GetFile(ctx, fileID)
	if err != nil {
		return nil, nil, err
	}

	vf := s.convertToFile(*fileItem, acc.ID, acc.Name)
	return &vf, client, nil
}
