package service

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const accountPoolHealthNotifyPageSize = 200

// AccountPoolHealthNotifyService sends a one-time admin alert while no account
// remains schedulable, then resets the guard after any account recovers.
type AccountPoolHealthNotifyService struct {
	accountRepo              AccountRepository
	settingRepo              SettingRepository
	notificationEmailService *NotificationEmailService
	interval                 time.Duration
	stopCh                   chan struct{}
	stopOnce                 sync.Once
	wg                       sync.WaitGroup
	nowFunc                  func() time.Time
}

func NewAccountPoolHealthNotifyService(
	accountRepo AccountRepository,
	settingRepo SettingRepository,
	notificationEmailService *NotificationEmailService,
	interval time.Duration,
) *AccountPoolHealthNotifyService {
	return &AccountPoolHealthNotifyService{
		accountRepo:              accountRepo,
		settingRepo:              settingRepo,
		notificationEmailService: notificationEmailService,
		interval:                 interval,
		stopCh:                   make(chan struct{}),
		nowFunc:                  time.Now,
	}
}

func (s *AccountPoolHealthNotifyService) Start() {
	if s == nil || s.accountRepo == nil || s.settingRepo == nil || s.notificationEmailService == nil || s.interval <= 0 {
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

func (s *AccountPoolHealthNotifyService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *AccountPoolHealthNotifyService) runOnce() {
	if s == nil || s.accountRepo == nil || s.settingRepo == nil || s.notificationEmailService == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	accounts, err := s.listAccounts(ctx)
	if err != nil {
		log.Printf("[AccountPoolHealthNotify] List accounts failed: %v", err)
		return
	}
	schedulable, err := s.accountRepo.ListSchedulable(ctx)
	if err != nil {
		log.Printf("[AccountPoolHealthNotify] List schedulable accounts failed: %v", err)
		return
	}
	if len(accounts) == 0 || len(schedulable) > 0 {
		s.clearOutageGuard(ctx)
		return
	}
	s.sendUnavailableAlertIfNeeded(ctx, accounts)
}

func (s *AccountPoolHealthNotifyService) listAccounts(ctx context.Context) ([]Account, error) {
	var accounts []Account
	for page := 1; ; page++ {
		items, pag, err := s.accountRepo.List(ctx, pagination.PaginationParams{Page: page, PageSize: accountPoolHealthNotifyPageSize})
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, items...)
		if pag == nil || page >= pag.Pages || len(items) == 0 {
			break
		}
	}
	return accounts, nil
}

func (s *AccountPoolHealthNotifyService) clearOutageGuard(ctx context.Context) {
	if err := s.settingRepo.Delete(ctx, SettingKeyAccountPoolUnavailableOutageID); err != nil && !errors.Is(err, ErrSettingNotFound) {
		log.Printf("[AccountPoolHealthNotify] Clear outage guard failed: %v", err)
	}
}

func (s *AccountPoolHealthNotifyService) sendUnavailableAlertIfNeeded(ctx context.Context, accounts []Account) {
	outageID, err := s.settingRepo.GetValue(ctx, SettingKeyAccountPoolUnavailableOutageID)
	if err == nil && strings.TrimSpace(outageID) != "" {
		return
	}
	if err != nil && !errors.Is(err, ErrSettingNotFound) {
		log.Printf("[AccountPoolHealthNotify] Read outage guard failed: %v", err)
		return
	}
	if !s.notifyEnabled(ctx) {
		return
	}
	recipients := s.notifyEmails(ctx)
	if len(recipients) == 0 {
		return
	}

	now := s.now()
	outageID = now.UTC().Format(time.RFC3339Nano)
	checkedAt := now.Format("2006-01-02 15:04:05")
	rows := renderAccountPoolUnavailableRows(accounts)
	sent := 0
	for _, recipient := range recipients {
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventAccountPoolUnavailable,
			RecipientEmail: recipient,
			RecipientName:  emailRecipientName(recipient),
			SourceType:     "account_pool_unavailable",
			SourceID:       outageID,
			ReminderKey:    "all_unavailable",
			Variables: map[string]string{
				"account_count": strconv.Itoa(len(accounts)),
				"checked_at":    checkedAt,
			},
			RawHTMLVariables: map[string]string{
				"accounts": rows,
			},
		}); err != nil {
			log.Printf("[AccountPoolHealthNotify] Send unavailable alert failed: count=%d recipient=%s err=%v", len(accounts), recipient, err)
			continue
		}
		sent++
	}
	if sent == 0 {
		return
	}
	if err := s.settingRepo.Set(ctx, SettingKeyAccountPoolUnavailableOutageID, outageID); err != nil {
		log.Printf("[AccountPoolHealthNotify] Set outage guard failed: %v", err)
	}
}

func (s *AccountPoolHealthNotifyService) notifyEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeyAccountQuotaNotifyEnabled)
	if err != nil {
		if !errors.Is(err, ErrSettingNotFound) {
			log.Printf("[AccountPoolHealthNotify] Read notification switch failed: %v", err)
		}
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func (s *AccountPoolHealthNotifyService) notifyEmails(ctx context.Context) []string {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyAccountQuotaNotifyEmails)
	if err != nil || strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil
	}
	return filterVerifiedEmails(ParseNotifyEmails(raw))
}

func (s *AccountPoolHealthNotifyService) now() time.Time {
	if s == nil || s.nowFunc == nil {
		return time.Now()
	}
	return s.nowFunc()
}

func renderAccountPoolUnavailableRows(accounts []Account) string {
	var b strings.Builder
	write := func(value string) {
		_, _ = b.WriteString(value)
	}
	write(`<table style="width:100%;border-collapse:collapse;">`)
	write(`<thead><tr><th align="left">ID</th><th align="left">Name</th><th align="left">Platform</th><th align="left">Status</th><th align="left">Schedulable</th><th align="left">Notes</th></tr></thead><tbody>`)
	for _, account := range accounts {
		notes := ""
		if account.Notes != nil {
			notes = strings.TrimSpace(*account.Notes)
		}
		write("<tr>")
		write("<td>" + html.EscapeString(strconv.FormatInt(account.ID, 10)) + "</td>")
		write("<td>" + html.EscapeString(account.Name) + "</td>")
		write("<td>" + html.EscapeString(account.Platform) + "</td>")
		write("<td>" + html.EscapeString(account.Status) + "</td>")
		write("<td>" + html.EscapeString(fmt.Sprintf("%t", account.Schedulable)) + "</td>")
		write("<td>" + html.EscapeString(notes) + "</td>")
		write("</tr>")
	}
	write("</tbody></table>")
	return b.String()
}
