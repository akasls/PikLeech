package fileagg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/config"
	"pikpak-manager/internal/db"
)

func TestFileAgg_VirtualID(t *testing.T) {
	accID := int64(42)
	fileID := "pikpak_file_unique_xyz123"

	vID := EncodeVirtualID(accID, fileID)
	if vID == "" {
		t.Fatalf("Encoded virtual ID should not be empty")
	}

	decodedAccID, decodedFileID, err := DecodeVirtualID(vID)
	if err != nil {
		t.Fatalf("DecodeVirtualID failed: %v", err)
	}

	if decodedAccID != accID || decodedFileID != fileID {
		t.Errorf("Expected (%d, %s), got (%d, %s)", accID, fileID, decodedAccID, decodedFileID)
	}
}

func TestFileAgg_SameNameFilesAndSearch(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_fileagg_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-fileagg-test",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	accSvc := account.NewService(database, cfg.AppSecret)
	svc := NewService(database, accSvc)

	// Insert two files with IDENTICAL names from different accounts into cache
	now := time.Now().UTC()
	vf1 := VirtualFile{
		VirtualID:    EncodeVirtualID(1, "file_id_account1"),
		AccountID:    1,
		AccountName:  "Account-A",
		PikPakFileID: "file_id_account1",
		Name:         "Interstellar.2014.mkv",
		Size:         2500000000,
		Kind:         "drive#file",
		CreatedTime:  now,
		ModifiedTime: now,
	}

	vf2 := VirtualFile{
		VirtualID:    EncodeVirtualID(2, "file_id_account2"),
		AccountID:    2,
		AccountName:  "Account-B",
		PikPakFileID: "file_id_account2",
		Name:         "Interstellar.2014.mkv", // Exact same name!
		Size:         4500000000,
		Kind:         "drive#file",
		CreatedTime:  now,
		ModifiedTime: now,
	}

	svc.cacheFile(vf1)
	svc.cacheFile(vf2)

	// Verify both exist with different virtual_ids
	if vf1.VirtualID == vf2.VirtualID {
		t.Fatalf("Virtual IDs must be unique for same-name files from different accounts")
	}

	// Search by keyword "Interstellar"
	results, err := svc.SearchFiles(context.Background(), "Interstellar", 0)
	if err != nil {
		t.Fatalf("SearchFiles failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 search results with same name from different accounts, found %d", len(results))
	}
}

func TestFileAgg_BatchDeleteGrouping(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pikpak_test_batchdel_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &config.Config{
		DBPath:        filepath.Join(tempDir, "test.db"),
		AppSecret:     "secret-32-chars-for-fileagg-test",
		AdminUsername: "admin",
		AdminPassword: "adminpassword",
	}

	database, err := db.InitDB(cfg)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	accSvc := account.NewService(database, cfg.AppSecret)
	svc := NewService(database, accSvc)

	// Create test accounts A and B
	ctx := context.Background()
	accA, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "AccA", IsEnabled: true})
	accB, _ := accSvc.CreateAccount(ctx, account.CreateAccountReq{Name: "AccB", IsEnabled: true})

	// Construct cross-account batch delete request: 2 from AccA, 1 from AccB
	vID_A1 := EncodeVirtualID(accA.ID, "f_a1")
	vID_A2 := EncodeVirtualID(accA.ID, "f_a2")
	vID_B1 := EncodeVirtualID(accB.ID, "f_b1")

	// Pre-populate cache
	svc.cacheFile(VirtualFile{VirtualID: vID_A1, AccountID: accA.ID, PikPakFileID: "f_a1", Name: "A1", Kind: "drive#file"})
	svc.cacheFile(VirtualFile{VirtualID: vID_A2, AccountID: accA.ID, PikPakFileID: "f_a2", Name: "A2", Kind: "drive#file"})
	svc.cacheFile(VirtualFile{VirtualID: vID_B1, AccountID: accB.ID, PikPakFileID: "f_b1", Name: "B1", Kind: "drive#file"})

	req := BatchDeleteReq{
		VirtualIDs: []string{vID_A1, vID_A2, vID_B1, "invalid_virtual_id"},
	}

	res, err := svc.BatchDelete(ctx, req)
	if err != nil {
		t.Fatalf("BatchDelete returned unexpected fatal error: %v", err)
	}

	if res.Total != 4 {
		t.Errorf("Expected Total 4, got %d", res.Total)
	}
	// The invalid one should be marked failed
	hasInvalidFailed := false
	for _, item := range res.Items {
		if item.VirtualID == "invalid_virtual_id" && !item.Success {
			hasInvalidFailed = true
		}
	}
	if !hasInvalidFailed {
		t.Errorf("Expected invalid virtual ID to be marked failed in items")
	}
}
