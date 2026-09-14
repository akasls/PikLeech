package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"pikpak-manager/internal/account"
	"pikpak-manager/internal/crypto"
	"pikpak-manager/internal/pikpak"
)

var (
	ErrNoAvailableAccounts         = errors.New("no active PikPak accounts available")
	ErrAllAccountsQuotaExhausted   = errors.New("all PikPak accounts have exhausted their daily offline quota")
)

type AccountScheduler struct {
	accountService *account.Service

	// Round-robin cursors keyed by priority level
	rrMu    sync.Mutex
	cursors map[int]*uint64

	// Per-account locks to avoid parallel collision on the same account
	accountLocksMu sync.Mutex
	accountLocks   map[int64]*sync.Mutex

	// Mock creator hook for testing without real PikPak API
	taskCreator func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error)
}

func NewAccountScheduler(accSvc *account.Service) *AccountScheduler {
	return &AccountScheduler{
		accountService: accSvc,
		cursors:        make(map[int]*uint64),
		accountLocks:   make(map[int64]*sync.Mutex),
	}
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

// SelectAccount picks an account using Priority + Round Robin
func (s *AccountScheduler) SelectAccount(excludeIDs map[int64]bool) (*account.Account, error) {
	allAvailable, err := s.GetAvailableAccounts()
	if err != nil {
		return nil, err
	}

	var candidates []*account.Account
	for _, acc := range allAvailable {
		if excludeIDs != nil && excludeIDs[acc.ID] {
			continue
		}
		candidates = append(candidates, acc)
	}

	if len(candidates) == 0 {
		return nil, ErrNoAvailableAccounts
	}

	// Group candidates by priority
	groups := make(map[int][]*account.Account)
	var priorities []int

	for _, acc := range candidates {
		p := acc.Priority
		if _, exists := groups[p]; !exists {
			priorities = append(priorities, p)
		}
		groups[p] = append(groups[p], acc)
	}

	// Sort priorities descending (highest number = highest priority)
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

// SubmitOfflineTask submits an offline download task with auto-failover across accounts
func (s *AccountScheduler) SubmitOfflineTask(ctx context.Context, downloadURL string, fileName string) (*pikpak.OfflineTask, *account.Account, error) {
	attemptedAccounts := make(map[int64]bool)

	for {
		acc, err := s.SelectAccount(attemptedAccounts)
		if err != nil {
			if errors.Is(err, ErrNoAvailableAccounts) {
				if len(attemptedAccounts) > 0 {
					return nil, nil, ErrAllAccountsQuotaExhausted
				}
				return nil, nil, ErrNoAvailableAccounts
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
		log.Printf("[SCHEDULER] Attempting task on Account %d (%s) [Priority %d] for %s", acc.ID, acc.Name, acc.Priority, maskedMagnet)

		// Execute creation with retries for network glitches
		var task *pikpak.OfflineTask
		var createErr error

		// Maximum 3 retries for transient network errors with exponential backoff
		for attempt := 1; attempt <= 3; attempt++ {
			if s.taskCreator != nil {
				task, createErr = s.taskCreator(ctx, client, downloadURL)
			} else {
				task, createErr = client.CreateOfflineTask(ctx, downloadURL, fileName, "")
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

		// Unknown/Network error that failed all 3 retries
		log.Printf("[SCHEDULER] Account %d (%s) error: %v", acc.ID, acc.Name, createErr)
		return nil, nil, fmt.Errorf("task creation failed on account %s: %w", acc.Name, createErr)
	}
}

// SetTaskCreatorHook allows injecting a mock creator for unit testing
func (s *AccountScheduler) SetTaskCreatorHook(hook func(ctx context.Context, client *pikpak.Client, downloadURL string) (*pikpak.OfflineTask, error)) {
	s.taskCreator = hook
}
