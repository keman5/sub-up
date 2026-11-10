package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestNotificationSettingsPersistedConsumers(t *testing.T) {
	ctx := context.Background()
	repo := newNotificationEmailMemorySettingRepo()
	settingsSvc := NewSettingService(repo, &config.Config{})
	settings, err := settingsSvc.GetAllSettings(ctx)
	require.NoError(t, err)
	settings.NotificationEmailDefaultLocale = "en"
	settings.SubscriptionExpiredAdminNotifyEnabled = true
	settings.SubscriptionExpiredAdminNotifyEmails = []NotifyEmailEntry{
		{Email: "confirmed@example.com", Verified: true},
		{Email: "unconfirmed@example.com"},
		{Email: "disabled@example.com", Verified: true, Disabled: true},
	}
	require.NoError(t, settingsSvc.UpdateSettings(ctx, settings))
	expiry := NewSubscriptionExpiryService(nil, time.Minute)
	expiry.SetSettingRepository(repo)
	require.True(t, expiry.expiredAdminNotifyEnabled(ctx))
	require.Equal(t, []string{"confirmed@example.com"}, expiry.expiredAdminNotifyEmails(ctx))
	mail := NewNotificationEmailService(repo, nil)
	require.Equal(t, "en", mail.ResolveRecipientLocale(ctx, 0, "new@example.com"))
	mail.RememberRecipientLocale(ctx, 42, "known@example.com", "zh-CN")
	require.Equal(t, "zh", mail.ResolveRecipientLocale(ctx, 42, "known@example.com"))

	settings.NotificationEmailDefaultLocale = ""
	settings.SubscriptionExpiredAdminNotifyEnabled = false
	settings.SubscriptionExpiredAdminNotifyEmails = nil
	require.NoError(t, settingsSvc.UpdateSettings(ctx, settings))
	require.False(t, expiry.expiredAdminNotifyEnabled(ctx))
	require.Empty(t, expiry.expiredAdminNotifyEmails(ctx))
	require.Equal(t, "zh", mail.ResolveRecipientLocale(ctx, 0, "new@example.com"))
}
