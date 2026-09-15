package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/crypto"
	"pikpak-manager/internal/pikpak"
	"pikpak-manager/internal/settings"
)

var (
	ErrNoAvailableAccounts         = errors.New("no active PikPak accounts available")
	ErrAllAccountsQuotaExhausted   = errors.New("all PikPak accounts have exhausted their daily offline quota")
	ErrInsufficientStorage         = errors.New("no available accounts have sufficient free storage space for this task")
)

type AccountScheduler struct {
	accountService  *account.Service
	settingsService *settings.Service

	// Round-robin cursors keyed by priority level
	rrMu    sync.Mutex
	cursors map[int]*uint64

	// Per-account locks to avoid parallel collision on the same account
	accountLocksMu sync.Mutex
	accountLocks   map[int64]*sync.Mutex

	// Cache user folder IDs across accounts: "accountID:username" -> folderID
	userFoldersMu sync.RWMutex
	userFolders   map[string]string

	// Mock creator hook for testing without real PikPak API
	taskCreator func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error)
}

func NewAccountScheduler(accSvc *account.Service, optSettings ...*settings.Service) *AccountScheduler {
	var setSvc *settings.Service
	if len(optSettings) > 0 {
		setSvc = optSettings[0]
	}
	return &AccountScheduler{
		accountService:  accSvc,
		settingsService: setSvc,
		cursors:         make(map[int]*uint64),
		accountLocks:    make(map[int64]*sync.Mutex),
		userFolders:     make(map[string]string),
	}
}

func (s *AccountScheduler) SetSettingsService(setSvc *settings.Service) {
	s.settingsService = setSvc
}

func (s *AccountScheduler) getAccountLock(accountID int64) *sync.Mutex {
	s.accountLocksMu.Lock()
	defer s.accountLocksMu.Unlock()

	l, exists := s.accountLocks[accountID]
	if !exists {
		l = &sync.Mutex{}
		s.accountLocks[accountID] = l
	}
	return l
}

// GetAvailableAccounts retrieves all enabled, healthy accounts ready for offline tasks
func (s *AccountScheduler) GetAvailableAccounts() ([]*account.Account, error) {
	// Auto check date reset first
	_ = s.accountService.AutoResetDailyQuotas()

	accounts, err := s.accountService.ListAccounts()
	if err != nil {
		return nil, err
	}

	var available []*account.Account
	now := time.Now()

	for _, acc := range accounts {
		if !acc.IsEnabled {
			continue
		}

		// Exclude quota exhausted
		if acc.Status == "QUOTA_EXHAUSTED" {
			continue
		}

		// Exclude disabled or fatal auth/proxy failed
		if acc.Status == "DISABLED" || acc.Status == "AUTH_FAILED" || acc.Status == "PROXY_FAILED" {
			continue
		}

		// Check cooldown (429 rate limit)
		if acc.CooldownUntil != nil && now.Before(*acc.CooldownUntil) {
			continue
		}

		available = append(available, acc)
	}

	return available, nil
}

// SelectAccount picks an account using Priority + Storage Balancing + Round Robin
func (s *AccountScheduler) SelectAccount(excludeIDs map[int64]bool, requiredBytes ...int64) (*account.Account, error) {
	allAvailable, err := s.GetAvailableAccounts()
	if err != nil {
		return nil, err
	}

	var reqBytes int64
	if len(requiredBytes) > 0 {
		reqBytes = requiredBytes[0]
	}

	balancingEnabled := true
	var minFreeBytes int64 = 10 * 1024 * 1024 * 1024 // 10 GB default
	if s.settingsService != nil {
		if set, err := s.settingsService.GetSettings(context.Background()); err == nil {
			balancingEnabled = set.StorageBalancingEnabled
			if set.StorageMinFreeGB > 0 {
				minFreeBytes = int64(set.StorageMinFreeGB) * 1024 * 1024 * 1024
			}
		}
	}

	var candidates []*account.Account
	for _, acc := range allAvailable {
		if excludeIDs != nil && excludeIDs[acc.ID] {
			continue
		}

		freeSpace := acc.TotalSpace - acc.UsedSpace
		if freeSpace < 0 {
			freeSpace = 0
		}

		// If a specific size is required (e.g. 10GB), exclude accounts that have less free space
		if reqBytes > 0 && freeSpace < reqBytes {
			continue
		}

		candidates = append(candidates, acc)
	}

	if len(candidates) == 0 {
		if reqBytes > 0 {
			return nil, ErrInsufficientStorage
		}
		return nil, ErrNoAvailableAccounts
	}

	// If storage balancing is enabled and no explicit reqBytes was given,
	// prefer accounts that have at least minFreeBytes to prevent filling accounts to 100%
	if balancingEnabled && reqBytes == 0 && minFreeBytes > 0 {
		var healthySpaceCandidates []*account.Account
		for _, acc := range candidates {
			freeSpace := acc.TotalSpace - acc.UsedSpace
			if freeSpace >= minFreeBytes {
				healthySpaceCandidates = append(healthySpaceCandidates, acc)
			}
		}
		if len(healthySpaceCandidates) > 0 {
			candidates = healthySpaceCandidates
		}
	}

	// If storage auto-balancing is active:
	// Sort candidates by Priority DESC, then by Free Space DESC
	if balancingEnabled {
		sort.SliceStable(candidates, func(i, j int) bool {
			if candidates[i].Priority != candidates[j].Priority {
				return candidates[i].Priority > candidates[j].Priority
			}
			freeI := candidates[i].TotalSpace - candidates[i].UsedSpace
			freeJ := candidates[j].TotalSpace - candidates[j].UsedSpace
			return freeI > freeJ
		})

		topPriority := candidates[0].Priority
		var topPriorityGroup []*account.Account
		for _, acc := range candidates {
			if acc.Priority == topPriority {
				topPriorityGroup = append(topPriorityGroup, acc)
			}
		}

		// Top account has the largest free space
		maxFree := topPriorityGroup[0].TotalSpace - topPriorityGroup[0].UsedSpace
		var maxFreeGroup []*account.Account
		for _, acc := range topPriorityGroup {
			free := acc.TotalSpace - acc.UsedSpace
			// Group accounts with nearly identical free space (within 200MB) for round-robin
			if maxFree-free < 200*1024*1024 {
				maxFreeGroup = append(maxFreeGroup, acc)
			}
		}

		if len(maxFreeGroup) == 1 {
			return maxFreeGroup[0], nil
		}

		s.rrMu.Lock()
		cursorPtr, exists := s.cursors[topPriority]
		if !exists {
			var initVal uint64 = 0
			cursorPtr = &initVal
			s.cursors[topPriority] = cursorPtr
		}
		s.rrMu.Unlock()

		idx := atomic.AddUint64(cursorPtr, 1) % uint64(len(maxFreeGroup))
		return maxFreeGroup[idx], nil
	}

	// Standard Round Robin if balancing is disabled
	groups := make(map[int][]*account.Account)
	var priorities []int

	for _, acc := range candidates {
		p := acc.Priority
		if _, exists := groups[p]; !exists {
			priorities = append(priorities, p)
		}
		groups[p] = append(groups[p], acc)
	}

	sort.Slice(priorities, func(i, j int) bool {
		return priorities[i] > priorities[j]
	})

	topPriority := priorities[0]
	topGroup := groups[topPriority]

	s.rrMu.Lock()
	cursorPtr, exists := s.cursors[topPriority]
	if !exists {
		var initVal uint64 = 0
		cursorPtr = &initVal
		s.cursors[topPriority] = cursorPtr
	}
	s.rrMu.Unlock()

	idx := atomic.AddUint64(cursorPtr, 1) % uint64(len(topGroup))
	return topGroup[idx], nil
}

// GetOrCreateUserFolder returns the folder ID for a user on a given account, creating it if necessary.
func (s *AccountScheduler) GetOrCreateUserFolder(ctx context.Context, client *pikpak.Client, accountID int64, username string) (string, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return "", nil
	}

	cacheKey := fmt.Sprintf("%d:%s", accountID, username)
	s.userFoldersMu.RLock()
	if id, exists := s.userFolders[cacheKey]; exists && id != "" {
		s.userFoldersMu.RUnlock()
		return id, nil
	}
	s.userFoldersMu.RUnlock()

	s.userFoldersMu.Lock()
	defer s.userFoldersMu.Unlock()

	if id, exists := s.userFolders[cacheKey]; exists && id != "" {
		return id, nil
	}

	folderName := "User_" + username
	resp, err := client.ListFiles(ctx, "", "", 100)
	if err == nil && resp != nil {
		for _, f := range resp.Files {
			if f.Kind == "drive#folder" && strings.EqualFold(f.Name, folderName) {
				s.userFolders[cacheKey] = f.ID
				return f.ID, nil
			}
		}
	}

	newFolder, err := client.MakeDir(ctx, "", folderName)
	if err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", folderName, err)
	}

	s.userFolders[cacheKey] = newFolder.ID
	return newFolder.ID, nil
}

// SubmitOfflineTask submits an offline download task with auto-failover across accounts
func (s *AccountScheduler) SubmitOfflineTask(ctx context.Context, downloadURL string, fileName string, username string, requiredBytes ...int64) (*pikpak.OfflineTask, *account.Account, error) {
	var reqBytes int64
	if len(requiredBytes) > 0 {
		reqBytes = requiredBytes[0]
	}

	attemptedAccounts := make(map[int64]bool)

	for {
		acc, err := s.SelectAccount(attemptedAccounts, reqBytes)
		if err != nil {
			if errors.Is(err, ErrNoAvailableAccounts) || errors.Is(err, ErrInsufficientStorage) {
				if len(attemptedAccounts) > 0 {
					return nil, nil, ErrAllAccountsQuotaExhausted
				}
				return nil, nil, err
			}
			return nil, nil, err
		}

		attemptedAccounts[acc.ID] = true

		// Lock this account to prevent concurrent token/quota race
		accLock := s.getAccountLock(acc.ID)
		accLock.Lock()

		client, err := s.accountService.GetClient(acc.ID)
		if err != nil {
			accLock.Unlock()
			log.Printf("[SCHEDULER] Failed to get client for account %d: %v", acc.ID, err)
			continue
		}

		maskedMagnet := crypto.MaskMagnet(downloadURL)
		log.Printf("[SCHEDULER] Attempting task on Account %d (%s) [Priority %d] for %s (user: %s)", acc.ID, acc.Name, acc.Priority, maskedMagnet, username)

		// Execute creation with retries for network glitches
		var task *pikpak.OfflineTask
		var createErr error

		// Maximum 3 retries for transient network errors with exponential backoff
		for attempt := 1; attempt <= 3; attempt++ {
			if s.taskCreator != nil {
				task, createErr = s.taskCreator(ctx, client, downloadURL)
			} else {
				var parentID string
				if username != "" {
					folderID, fErr := s.GetOrCreateUserFolder(ctx, client, acc.ID, username)
					if fErr == nil {
						parentID = folderID
					} else {
						log.Printf("[SCHEDULER] Warning: failed to create user folder for %s: %v", username, fErr)
					}
				}
				task, createErr = client.CreateOfflineTask(ctx, downloadURL, fileName, parentID)
			}

			if createErr == nil {
				break
			}

			// If error is Quota Exceeded, do not retry this account!
			if errors.Is(createErr, pikpak.ErrQuotaExceeded) {
				break
			}

			// If error is 429, break to cooldown
			if errors.Is(createErr, pikpak.ErrRateLimited) {
				break
			}

			// If proxy error, break
			if errors.Is(createErr, pikpak.ErrProxyFailed) {
				break
			}

			// If network error, back off and retry
			if errors.Is(createErr, pikpak.ErrNetwork) {
				time.Sleep(time.Duration(attempt) * time.Second)
				continue
			}

			break
		}

		accLock.Unlock()

		if createErr == nil && task != nil {
			// Successfully created task
			log.Printf("[SCHEDULER] Offline task created successfully on Account %d (%s), task_id: %s", acc.ID, acc.Name, task.ID)
			_ = s.accountService.IncrementDailyTaskCount(acc.ID)
			return task, acc, nil
		}

		// Handle error cases:
		if errors.Is(createErr, pikpak.ErrQuotaExceeded) {
			log.Printf("[SCHEDULER] Account %d (%s) QUOTA EXHAUSTED! Marking and failing over...", acc.ID, acc.Name)
			_ = s.accountService.UpdateStatus(acc.ID, "QUOTA_EXHAUSTED", "task_daily_create_limit: daily offline limit reached")
			continue // Immediately try next account
		}

		if errors.Is(createErr, pikpak.ErrRateLimited) {
			log.Printf("[SCHEDULER] Account %d (%s) RATE LIMITED! Entering 60s cooldown...", acc.ID, acc.Name)
			client.SetCooldown(60 * time.Second)
			continue // Try next account
		}

		if errors.Is(createErr, pikpak.ErrProxyFailed) {
			log.Printf("[SCHEDULER] Account %d (%s) PROXY FAILED (%v). NOT marking quota exhausted.", acc.ID, acc.Name, createErr)
			_ = s.accountService.UpdateStatus(acc.ID, "PROXY_FAILED", createErr.Error())
			continue // Try next account
		}

		if errors.Is(createErr, pikpak.ErrAuthFailed) {
			log.Printf("[SCHEDULER] Account %d (%s) AUTH FAILED (%v).", acc.ID, acc.Name, createErr)
			_ = s.accountService.UpdateStatus(acc.ID, "AUTH_FAILED", createErr.Error())
			continue // Try next account
		}

		if errors.Is(createErr, pikpak.ErrNetwork) {
			log.Printf("[SCHEDULER] Account %d (%s) NETWORK ERROR after retries (%v). Failing over to next account...", acc.ID, acc.Name, createErr)
			continue // Try next account
		}

		// Unknown error that failed all attempts
		log.Printf("[SCHEDULER] Account %d (%s) error: %v", acc.ID, acc.Name, createErr)
		return nil, nil, fmt.Errorf("task creation failed on account %s: %w", acc.Name, createErr)
	}
}

// SetTaskCreatorHook allows injecting a mock creator for unit testing
func (s *AccountScheduler) SetTaskCreatorHook(hook func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error)) {
	s.taskCreator = hook
}
