package fileagg

import (
	"context"
	"database/sql"
	"errors"
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

// ListFiles lists files in unified root or specific folder with user isolation
func (s *Service) ListFiles(ctx context.Context, parentVirtualID string, userID int64, username, userRole string, sortBy, sortOrder string) ([]VirtualFile, error) {
	if parentVirtualID == "" || parentVirtualID == "root" {
		return s.listUnifiedRoot(ctx, userID, username, userRole, sortBy, sortOrder)
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

	result := make([]VirtualFile, 0)
	for _, f := range resp.Files {
		vf := s.convertToFile(f, acc.ID, acc.Name, userID)
		result = append(result, vf)
	}
	if len(result) > 0 {
		go s.cacheFiles(result, userID)
	}

	s.sortFiles(result, sortBy, sortOrder)
	return result, nil
}

func (s *Service) listUnifiedRoot(ctx context.Context, userID int64, username, userRole string, sortBy, sortOrder string) ([]VirtualFile, error) {
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
	allFiles := make([]VirtualFile, 0)
	var wg sync.WaitGroup

	for _, acc := range enabledAccounts {
		wg.Add(1)
		go func(a *account.Account) {
			defer wg.Done()
			client, err := s.accountService.GetClient(a.ID)
			if err != nil {
				return
			}

			var converted []VirtualFile

			if username != "" && userRole != "admin" {
				// Normal user: look for folder User_<username> on this account
				userFolderName := "User_" + username
				resp, err := client.ListFiles(ctx, "", "", 100)
				if err != nil {
					log.Printf("[FILEAGG] Error listing files for account %d (%s): %v", a.ID, a.Name, err)
					return
				}

				var userFolderID string
				for _, f := range resp.Files {
					if f.Kind == "drive#folder" && strings.EqualFold(f.Name, userFolderName) {
						userFolderID = f.ID
						break
					}
				}

				if userFolderID != "" {
					userFilesResp, err := client.ListFiles(ctx, userFolderID, "", 200)
					if err == nil {
						for _, f := range userFilesResp.Files {
							vf := s.convertToFile(f, a.ID, a.Name, userID)
							converted = append(converted, vf)
						}
					}
				}
			} else {
				// Admin or unspecified user:
				resp, err := client.ListFiles(ctx, "", "", 100)
				if err != nil {
					log.Printf("[FILEAGG] Error listing files for account %d (%s): %v", a.ID, a.Name, err)
					return
				}

				// Check if User_admin exists
				adminFolderName := "User_" + username
				if username == "" {
					adminFolderName = "User_admin"
				}

				var adminFolderID string
				for _, f := range resp.Files {
					if f.Kind == "drive#folder" && strings.EqualFold(f.Name, adminFolderName) {
						adminFolderID = f.ID
						break
					}
				}

				if adminFolderID != "" {
					adminFilesResp, err := client.ListFiles(ctx, adminFolderID, "", 200)
					if err == nil {
						for _, f := range adminFilesResp.Files {
							vf := s.convertToFile(f, a.ID, a.Name, userID)
							converted = append(converted, vf)
						}
					}
				}

				// Also include items that do NOT start with "User_" (including My Pack and root downloads)
				for _, f := range resp.Files {
					// Isolate: hide any tenant User_* folders from admin view
					if strings.HasPrefix(f.Name, "User_") {
						continue
					}
					vf := s.convertToFile(f, a.ID, a.Name, userID)
					converted = append(converted, vf)
				}
			}

			if len(converted) > 0 {
				go s.cacheFiles(converted, userID)
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

func (s *Service) convertToFile(f pikpak.FileItem, accID int64, accName string, userID ...int64) VirtualFile {
	size, _ := strconv.ParseInt(f.Size, 10, 64)
	isFolder := f.Kind == "drive#folder"
	isVideo := !isFolder && (IsVideoExtension(f.Name) || strings.HasPrefix(f.MimeType, "video/"))

	var uid int64 = 1
	if len(userID) > 0 && userID[0] > 0 {
		uid = userID[0]
	}

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
		UserID:        uid,
		CreatedTime:   f.CreatedTime.Time(),
		ModifiedTime:  f.ModifiedTime.Time(),
	}
}

func (s *Service) cacheFiles(files []VirtualFile, userID ...int64) {
	if len(files) == 0 {
		return
	}
	now := time.Now().UTC()
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO file_cache (
			virtual_id, account_id, pikpak_file_id, parent_id,
			name, size, mime_type, kind, thumbnail_link,
			created_time, modified_time, user_id, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(virtual_id) DO UPDATE SET
			name = excluded.name,
			size = excluded.size,
			thumbnail_link = excluded.thumbnail_link,
			modified_time = excluded.modified_time,
			user_id = excluded.user_id,
			updated_at = excluded.updated_at
	`)
	if err != nil {
		return
	}
	defer stmt.Close()

	var defaultUID int64 = 1
	if len(userID) > 0 && userID[0] > 0 {
		defaultUID = userID[0]
	}

	for _, vf := range files {
		uid := vf.UserID
		if uid == 0 {
			uid = defaultUID
		}
		_, _ = stmt.Exec(
			vf.VirtualID, vf.AccountID, vf.PikPakFileID, vf.ParentID,
			vf.Name, vf.Size, vf.MimeType, vf.Kind, vf.ThumbnailLink,
			vf.CreatedTime, vf.ModifiedTime, uid, now,
		)
	}
	_ = tx.Commit()
}

func (s *Service) cacheFile(vf VirtualFile) {
	s.cacheFiles([]VirtualFile{vf}, vf.UserID)
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

// SearchFiles searches cached files across all accounts by name with optional user isolation
func (s *Service) SearchFiles(ctx context.Context, keyword string, userID int64, accountID int64) ([]VirtualFile, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}

	query := `
		SELECT c.virtual_id, c.account_id, COALESCE(a.name, 'Unknown'),
		       c.pikpak_file_id, c.parent_id, c.name, c.size, c.mime_type,
		       c.kind, c.thumbnail_link, c.created_time, c.modified_time, COALESCE(c.user_id, 1)
		FROM file_cache c
		LEFT JOIN pikpak_accounts a ON c.account_id = a.id
		WHERE c.name LIKE ?
	`
	args := []interface{}{"%" + keyword + "%"}
	if userID > 0 {
		query += " AND c.user_id = ?"
		args = append(args, userID)
	}
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
			&vf.Kind, &thumb, &cTime, &mTime, &vf.UserID,
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

// BatchDelete deletes multiple files across different accounts, checking user ownership if not admin
func (s *Service) BatchDelete(ctx context.Context, req BatchDeleteReq, userID int64, userRole string) (*BatchDeleteResponse, error) {
	if len(req.VirtualIDs) == 0 {
		return &BatchDeleteResponse{Total: 0}, nil
	}

	// Verify permissions for non-admin users
	if userRole != "admin" && userID > 0 {
		for _, vid := range req.VirtualIDs {
			var ownerID int64
			err := s.db.QueryRowContext(ctx, "SELECT COALESCE(user_id, 1) FROM file_cache WHERE virtual_id = ?", vid).Scan(&ownerID)
			if err == nil && ownerID != userID {
				return nil, errors.New("无权删除其他用户的文件")
			}
		}
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
		// 默认直接永久删除，不进回收站
		delErr = client.DeleteFiles(ctx, fileIDs)

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

// RenameFile renames a virtual file or folder in PikPak and updates the database cache
func (s *Service) RenameFile(ctx context.Context, virtualID, newName string, userID int64, userRole string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return fmt.Errorf("file name cannot be empty")
	}

	// Verify permissions for non-admin users
	if userRole != "admin" && userID > 0 {
		var ownerID int64
		err := s.db.QueryRowContext(ctx, "SELECT COALESCE(user_id, 1) FROM file_cache WHERE virtual_id = ?", virtualID).Scan(&ownerID)
		if err == nil && ownerID != userID {
			return errors.New("无权重命名其他用户的文件")
		}
	}

	accID, fileID, err := DecodeVirtualID(virtualID)
	if err != nil {
		return err
	}

	client, err := s.accountService.GetClient(accID)
	if err != nil {
		return err
	}

	if err := client.RenameFile(ctx, fileID, newName); err != nil {
		return fmt.Errorf("failed to rename on PikPak: %w", err)
	}

	now := time.Now().UTC()
	_, _ = s.db.ExecContext(ctx, "UPDATE file_cache SET name = ?, updated_at = ? WHERE virtual_id = ?", newName, now, virtualID)
	return nil
}
