package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/google/uuid"
)

const (
	// subscriptionExpiryReminderLeaderLockKey gates the per-cycle reminder scan so
	// that only one instance walks all active subscriptions and sends reminder
	// emails, avoiding redundant full scans and duplicate emails.
	subscriptionExpiryReminderLeaderLockKey = "subscription:expiry:reminder:leader"
	// subscriptionExpiryReminderLeaderLockTTL bounds crash recovery; the scan can
	// page through many subscriptions, so keep it comfortably above one cycle.
	subscriptionExpiryReminderLeaderLockTTL = 5 * time.Minute
)

// SubscriptionExpiryService periodically updates expired subscription status.
type SubscriptionExpiryService struct {
	userSubRepo              UserSubscriptionRepository
	settingRepo              SettingRepository
	notificationEmailService *NotificationEmailService
	interval                 time.Duration
	stopCh                   chan struct{}
	stopOnce                 sync.Once
	wg                       sync.WaitGroup

	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string
}

func NewSubscriptionExpiryService(userSubRepo UserSubscriptionRepository, interval time.Duration) *SubscriptionExpiryService {
	return &SubscriptionExpiryService{
		userSubRepo: userSubRepo,
		interval:    interval,
		stopCh:      make(chan struct{}),
		instanceID:  uuid.NewString(),
	}
}

// SetLeaderLock injects the leader-lock cache and DB used to elect a single
// instance for the periodic expiry-reminder scan. When both are nil the scan runs
// ungated (single-instance / test behavior).
func (s *SubscriptionExpiryService) SetLeaderLock(lockCache LeaderLockCache, db *sql.DB) {
	if s == nil {
		return
	}
	s.lockCache = lockCache
	s.db = db
}

func (s *SubscriptionExpiryService) SetSettingRepository(settingRepo SettingRepository) {
	s.settingRepo = settingRepo
}

func (s *SubscriptionExpiryService) SetNotificationEmailService(notificationEmailService *NotificationEmailService) {
	s.notificationEmailService = notificationEmailService
}

func (s *SubscriptionExpiryService) Start() {
	if s == nil || s.userSubRepo == nil || s.interval <= 0 {
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

func (s *SubscriptionExpiryService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stopCh)
	})
	s.wg.Wait()
}

func (s *SubscriptionExpiryService) runOnce() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	expired, err := s.userSubRepo.BatchUpdateExpiredStatus(ctx)
	if err != nil {
		log.Printf("[SubscriptionExpiry] Update expired subscriptions failed: %v", err)
		return
	}
	if len(expired) > 0 {
		log.Printf("[SubscriptionExpiry] Updated %d expired subscriptions", len(expired))
		s.sendExpiredAdminNotifications(ctx, expired)
	}
	s.sendExpiryReminders(ctx)
}

func (s *SubscriptionExpiryService) sendExpiredAdminNotifications(ctx context.Context, expired []UserSubscription) {
	if s == nil || s.userSubRepo == nil || s.settingRepo == nil || s.notificationEmailService == nil {
		return
	}
	if !s.expiredAdminNotifyEnabled(ctx) {
		return
	}
	recipients := s.expiredAdminNotifyEmails(ctx)
	if len(recipients) == 0 {
		return
	}

	for i := range expired {
		s.sendExpiredAdminNotification(ctx, &expired[i], recipients)
	}
}

func (s *SubscriptionExpiryService) expiredAdminNotifyEnabled(ctx context.Context) bool {
	value, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionExpiredAdminNotifyEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return false
		}
		log.Printf("[SubscriptionExpiry] Read expired admin notification switch failed: %v", err)
		return false
	}
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func (s *SubscriptionExpiryService) expiredAdminNotifyEmails(ctx context.Context) []string {
	raw, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionExpiredAdminNotifyEmails)
	if err != nil || strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "[]" {
		return nil
	}
	return filterVerifiedEmails(ParseNotifyEmails(raw))
}

func (s *SubscriptionExpiryService) sendExpiredAdminNotification(ctx context.Context, sub *UserSubscription, recipients []string) {
	if sub == nil || sub.User == nil || sub.Group == nil {
		return
	}
	for _, recipient := range recipients {
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventSubscriptionExpiredAdmin,
			RecipientEmail: recipient,
			RecipientName:  emailRecipientName(recipient),
			SourceType:     "user_subscription_expired",
			SourceID:       strconv.FormatInt(sub.ID, 10),
			ReminderKey:    "expired",
			Variables: map[string]string{
				"subscription_id":    strconv.FormatInt(sub.ID, 10),
				"subscription_group": sub.Group.Name,
				"user_id":            strconv.FormatInt(sub.UserID, 10),
				"user_email":         sub.User.Email,
				"user_name":          firstNonEmpty(sub.User.Username, sub.User.Email),
				"expiry_time":        sub.ExpiresAt.Format("2006-01-02 15:04"),
			},
		}); err != nil {
			log.Printf("[SubscriptionExpiry] Send expired admin notification failed: subscription=%d recipient=%s err=%v", sub.ID, recipient, err)
		}
	}
}

func (s *SubscriptionExpiryService) sendExpiryReminders(ctx context.Context) {
	if s == nil || s.userSubRepo == nil || s.notificationEmailService == nil {
		return
	}
	if !s.expiryReminderEnabled(ctx) {
		return
	}

	// Multi-instance guard: only the leader walks every active subscription and
	// sends reminders, avoiding N× full scans and duplicate reminder emails.
	release, ok := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, subscriptionExpiryReminderLeaderLockKey, s.instanceID, subscriptionExpiryReminderLeaderLockTTL)
	if !ok {
		return
	}
	defer release()
	for page := 1; ; page++ {
		subs, pag, err := s.userSubRepo.List(ctx, pagination.PaginationParams{Page: page, PageSize: 200}, nil, nil, SubscriptionStatusActive, "", "expires_at", "asc")
		if err != nil {
			log.Printf("[SubscriptionExpiry] List active subscriptions for reminder failed: %v", err)
			return
		}
		for i := range subs {
			s.sendExpiryReminderIfDue(ctx, &subs[i])
		}
		if pag == nil || page >= pag.Pages || len(subs) == 0 {
			return
		}
	}
}

func (s *SubscriptionExpiryService) expiryReminderEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return true
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeySubscriptionExpiryNotifyEnabled)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return true
		}
		log.Printf("[SubscriptionExpiry] Read expiry reminder switch failed: %v", err)
		return false
	}
	return !isFalseSettingValue(value)
}

func (s *SubscriptionExpiryService) sendExpiryReminderIfDue(ctx context.Context, sub *UserSubscription) {
	if sub == nil || sub.User == nil || sub.Group == nil || sub.User.Email == "" {
		return
	}
	daysRemaining := sub.DaysRemaining()
	if daysRemaining != 7 && daysRemaining != 3 && daysRemaining != 1 {
		return
	}
	if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventSubscriptionExpiryReminder,
		RecipientEmail: sub.User.Email,
		RecipientName:  firstNonEmpty(sub.User.Username, sub.User.Email),
		UserID:         sub.UserID,
		SourceType:     "user_subscription",
		SourceID:       strconv.FormatInt(sub.ID, 10),
		ReminderKey:    fmt.Sprintf("%dd", daysRemaining),
		Variables: map[string]string{
			"subscription_group": sub.Group.Name,
			"expiry_time":        sub.ExpiresAt.Format("2006-01-02 15:04"),
			"days_remaining":     strconv.Itoa(daysRemaining),
		},
	}); err != nil {
		log.Printf("[SubscriptionExpiry] Send expiry reminder failed: subscription=%d user=%d err=%v", sub.ID, sub.UserID, err)
	}
}
