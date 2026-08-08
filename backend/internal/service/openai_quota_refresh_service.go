package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	openAIQuotaRefreshInterval       = 30 * time.Minute
	openAIQuotaRefreshStaleAfter     = 24 * time.Hour
	openAIQuotaRefreshRunTimeout     = 2 * time.Minute
	openAIQuotaRefreshAccountTimeout = 20 * time.Second
	openAIQuotaRefreshParallelism    = 4
)

// OpenAIQuotaRefreshService periodically refreshes OpenAI Codex quota snapshots so
// quota-auto-paused accounts can recover once 5h/7d windows reset, even without new traffic.
type OpenAIQuotaRefreshService struct {
	accountRepo         AccountRepository
	accountUsageService *AccountUsageService
	interval            time.Duration
	refreshAccountUsage func(ctx context.Context, accountID int64) error
	stopCh              chan struct{}
	stopOnce            sync.Once
	wg                  sync.WaitGroup
}

func NewOpenAIQuotaRefreshService(accountRepo AccountRepository, accountUsageService *AccountUsageService, interval time.Duration) *OpenAIQuotaRefreshService {
	svc := &OpenAIQuotaRefreshService{
		accountRepo:         accountRepo,
		accountUsageService: accountUsageService,
		interval:            interval,
		stopCh:              make(chan struct{}),
	}
	svc.refreshAccountUsage = svc.refreshAccount
	return svc
}

func (s *OpenAIQuotaRefreshService) Start() {
	if s == nil || s.accountRepo == nil || s.accountUsageService == nil || s.interval <= 0 {
		return
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		s.runOnce()
		for {
			select {
			case <-ticker.C:
				s.runOnce()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *OpenAIQuotaRefreshService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *OpenAIQuotaRefreshService) runOnce() {
	if s == nil || s.accountRepo == nil || s.refreshAccountUsage == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), openAIQuotaRefreshRunTimeout)
	defer cancel()

	accounts, err := s.accountRepo.ListByPlatform(ctx, PlatformOpenAI)
	if err != nil {
		log.Printf("[OpenAIQuotaRefresh] list openai accounts failed: %v", err)
		return
	}

	var candidates []Account
	now := time.Now()
	for _, account := range accounts {
		if shouldRefreshOpenAIQuotaSnapshotInBackground(&account, now) {
			candidates = append(candidates, account)
		}
	}
	if len(candidates) == 0 {
		return
	}

	var group errgroup.Group
	group.SetLimit(openAIQuotaRefreshParallelism)
	for i := range candidates {
		accountID := candidates[i].ID
		group.Go(func() error {
			accountCtx, accountCancel := context.WithTimeout(ctx, openAIQuotaRefreshAccountTimeout)
			defer accountCancel()
			if err := s.refreshAccountUsage(accountCtx, accountID); err != nil {
				log.Printf("[OpenAIQuotaRefresh] refresh account %d failed: %v", accountID, err)
			}
			return nil
		})
	}
	_ = group.Wait()
}

func (s *OpenAIQuotaRefreshService) refreshAccount(ctx context.Context, accountID int64) error {
	if s == nil || s.accountUsageService == nil {
		return nil
	}
	_, err := s.accountUsageService.GetUsage(ctx, accountID, true)
	if err != nil {
		return fmt.Errorf("refresh openai usage: %w", err)
	}
	return nil
}

func shouldRefreshOpenAIQuotaSnapshotInBackground(account *Account, now time.Time) bool {
	if account == nil || !account.IsOpenAIOAuth() || !account.IsOpenAIResponsesWebSocketV2Enabled() || !account.IsActive() {
		return false
	}
	if account.Extra == nil {
		return true
	}
	if readOpenAIQuotaUsedPercent(account.Extra, "5h") > 0 && openAIQuotaWindowReset(account.Extra, "5h", now) {
		return true
	}
	if readOpenAIQuotaUsedPercent(account.Extra, "7d") > 0 && openAIQuotaWindowReset(account.Extra, "7d", now) {
		return true
	}
	updatedRaw, ok := account.Extra["codex_usage_updated_at"]
	if !ok {
		return true
	}
	updatedAt, err := parseTime(fmt.Sprint(updatedRaw))
	if err != nil {
		return true
	}
	return now.Sub(updatedAt) >= openAIQuotaRefreshStaleAfter
}
