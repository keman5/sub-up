//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func emailPolicyTestSender(t *testing.T) (*EmailService, *notificationEmailMemorySettingRepo, *fakeSMTPServer, *SMTPConfig) {
	t.Helper()
	srv, port := startFakeSMTPServer(t, false, false)
	repo := newNotificationEmailMemorySettingRepo()
	config := &SMTPConfig{Host: "127.0.0.1", Port: port, Username: "user", Password: "password", From: "sender@example.com"}
	require.NoError(t, repo.SetMultiple(context.Background(), map[string]string{
		SettingKeySMTPHost: config.Host, SettingKeySMTPPort: strconv.Itoa(port),
		SettingKeySMTPUsername: config.Username, SettingKeySMTPPassword: config.Password,
		SettingKeySMTPFrom: config.From,
	}))
	return NewEmailService(repo, &emailCacheStub{}), repo, srv, config
}

func TestEmailBlockedKeywordsSending(t *testing.T) {
	for _, tc := range []struct {
		name, policy, subject, body string
		blocked                     bool
	}{
		{"subject", `["never", "secret"]`, "a SECRET notice", "safe", true},
		{"body", `["secret"]`, "safe", "<p>SeCrEt</p>", true},
		{"Chinese", `["禁止"]`, "safe", "<p>禁止发送</p>", true},
		{"inline", `["foobar"]`, "safe", "foo<b>bar</b>", true},
		{"sanitized subject", `["foobar"]`, "foo\r\nbar", "safe", true},
		{"entity", `["foobar"]`, "safe", "foo&#98;ar", true},
		{"raw", `["tracking"]`, "safe", `<a href="https://tracking.example">link</a>`, true},
		{"normalize", `["", "  ", " Secret ", "secret"]`, "SECRET", "safe", true},
		{"unmatched", `["secret"]`, "hello", "safe", false},
		{"no subject body join", `["foobar"]`, "foo", "bar", false},
		{"no paragraph join", `["foobar"]`, "safe", "<p>foo</p><p>bar</p>", false},
		{"no break join", `["foobar"]`, "safe", "foo<br>bar", false},
		{"default", "", "secret", "secret", false},
		{"cleared", `[]`, "secret", "secret", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, direct := range []bool{false, true} {
				svc, repo, srv, config := emailPolicyTestSender(t)
				require.NoError(t, repo.Set(context.Background(), "email_blocked_keywords", tc.policy))
				var err error
				if direct {
					err = svc.SendEmailWithConfig(config, "recipient@example.com", tc.subject, tc.body)
				} else {
					err = svc.SendEmail(context.Background(), "recipient@example.com", tc.subject, tc.body)
				}
				if tc.blocked {
					require.ErrorIs(t, err, ErrEmailBlocked)
					require.EqualError(t, err, ErrEmailBlocked.Error())
					require.Zero(t, srv.conns.Load())
				} else {
					require.NoError(t, err)
					require.True(t, srv.sawCommand("DATA"))
				}
			}
		})
	}
}

type emailPolicyFailureRepo struct{ SettingRepository }

func (r emailPolicyFailureRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("settings unavailable")
}

func TestEmailBlockedKeywordsPolicyFailureDoesNotSend(t *testing.T) {
	svc, repo, srv, config := emailPolicyTestSender(t)
	for _, raw := range []string{`{}`, `null`, `[42]`, `["secret"`} {
		require.NoError(t, repo.Set(context.Background(), "email_blocked_keywords", raw))
		require.Error(t, svc.SendEmailWithConfig(config, "recipient@example.com", "safe", "safe"))
	}
	svc.settingRepo = emailPolicyFailureRepo{repo}
	require.Error(t, svc.SendEmailWithConfig(config, "recipient@example.com", "safe", "safe"))
	require.Zero(t, srv.conns.Load())
}

func TestEmailBlockedKeywordsNotificationNoDeliveryOrFallback(t *testing.T) {
	for _, event := range []string{NotificationEmailEventAuthVerifyCode, NotificationEmailEventSubscriptionExpiryReminder} {
		t.Run(event, func(t *testing.T) {
			ctx := context.Background()
			sender, repo, srv, _ := emailPolicyTestSender(t)
			notifications := NewNotificationEmailService(repo, sender)
			_, err := notifications.UpdateTemplate(ctx, event, "en", "secret", "<p>safe</p>")
			require.NoError(t, err)
			require.NoError(t, repo.Set(ctx, "email_blocked_keywords", `["secret"]`))
			input := NotificationEmailSendInput{Event: event, Locale: "en", RecipientEmail: "recipient@example.com", SourceType: "test", SourceID: "1", ReminderKey: "today"}
			err = notifications.Send(ctx, input)
			require.ErrorIs(t, err, ErrEmailBlocked)
			require.False(t, shouldFallbackNotificationEmail(err))
			key := notificationEmailDeliveryKey(event, input.SourceType, input.SourceID, input.RecipientEmail, input.ReminderKey)
			_, err = repo.GetValue(ctx, key)
			require.ErrorIs(t, err, ErrSettingNotFound)
			sender.SetNotificationEmailService(notifications)
			if event == NotificationEmailEventAuthVerifyCode {
				require.ErrorIs(t, sender.SendVerifyCode(ctx, input.RecipientEmail, "Example", "en"), ErrEmailBlocked)
			}
			require.Zero(t, srv.conns.Load())
			require.NoError(t, repo.Set(ctx, "email_blocked_keywords", `[]`))
			require.NoError(t, notifications.Send(ctx, input))
			_, err = repo.GetValue(ctx, key)
			require.NoError(t, err)
			require.True(t, srv.sawCommand("DATA"))
		})
	}
}

func TestEmailBlockedKeywordsModerationDoesNotMarkSent(t *testing.T) {
	ctx := context.Background()
	sender, repo, srv, _ := emailPolicyTestSender(t)
	require.NoError(t, repo.Set(ctx, SettingKeyEmailBlockedKeywords, `["<"]`))
	sender.SetNotificationEmailService(NewNotificationEmailService(repo, sender))
	svc := &ContentModerationService{emailService: sender}
	userID := int64(42)
	entry := &ContentModerationLog{ID: 1, UserID: &userID, UserEmail: "recipient@example.com", Flagged: true}
	svc.sendFlaggedNotificationSideEffects(ctx, &ContentModerationConfig{EmailOnHit: true}, entry, true)
	require.False(t, entry.EmailSent)
	require.Zero(t, srv.conns.Load())
}
