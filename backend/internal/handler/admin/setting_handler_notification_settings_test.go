package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func getNotificationSettings(t *testing.T, h *SettingHandler) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data
}

func TestNotificationSettingsHandlerDefaults(t *testing.T) {
	h, _ := newStepUpSwitchTestHandler(t, map[string]string{})
	data := getNotificationSettings(t, h)
	require.Equal(t, "zh", data["notification_email_default_locale"])
	require.Equal(t, false, data["subscription_expired_admin_notify_enabled"])
	require.Equal(t, []any{}, data["subscription_expired_admin_notify_emails"])
}

func TestNotificationSettingsHandlerRoundTripOmitAndClear(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	entries := []service.NotifyEmailEntry{
		{Email: "confirmed@example.com", Verified: true},
		{Email: "unconfirmed@example.com"},
		{Email: "disabled@example.com", Verified: true, Disabled: true},
	}
	response := doUpdateSettings(t, h, map[string]any{
		"notification_email_default_locale":         "en",
		"subscription_expired_admin_notify_enabled": true,
		"subscription_expired_admin_notify_emails":  entries,
	}, nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "en", repo.values[service.SettingKeyNotificationEmailDefaultLocale])
	require.Equal(t, "true", repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEnabled])
	require.Equal(t, entries, service.ParseNotifyEmails(repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEmails]))
	data := getNotificationSettings(t, h)
	require.Equal(t, "en", data["notification_email_default_locale"])
	require.Equal(t, true, data["subscription_expired_admin_notify_enabled"])
	var saved []service.NotifyEmailEntry
	raw, err := json.Marshal(data["subscription_expired_admin_notify_emails"])
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, entries, saved)
	var put struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &put))
	require.Equal(t, data["notification_email_default_locale"], put.Data["notification_email_default_locale"])
	require.Equal(t, data["subscription_expired_admin_notify_emails"], put.Data["subscription_expired_admin_notify_emails"])

	response = doUpdateSettings(t, h, map[string]any{"site_name": "preserve notifications"}, nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "en", repo.values[service.SettingKeyNotificationEmailDefaultLocale])
	require.Equal(t, "true", repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEnabled])
	require.Equal(t, entries, service.ParseNotifyEmails(repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEmails]))

	public, err := h.settingService.GetPublicSettings(context.Background())
	require.NoError(t, err)
	raw, err = json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "subscription_expired_admin_notify")
	require.NotContains(t, string(raw), "confirmed@example.com")
	require.NotContains(t, string(raw), "notification_email_default_locale")

	response = doUpdateSettings(t, h, map[string]any{
		"notification_email_default_locale":         "",
		"subscription_expired_admin_notify_enabled": false,
		"subscription_expired_admin_notify_emails":  []any{},
	}, nil)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Empty(t, repo.values[service.SettingKeyNotificationEmailDefaultLocale])
	require.Equal(t, "false", repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEnabled])
	require.Equal(t, "[]", repo.values[service.SettingKeySubscriptionExpiredAdminNotifyEmails])
	data = getNotificationSettings(t, h)
	require.Equal(t, "zh", data["notification_email_default_locale"])
	require.Equal(t, false, data["subscription_expired_admin_notify_enabled"])
	require.Equal(t, []any{}, data["subscription_expired_admin_notify_emails"])
}

func TestNotificationSettingsHandlerRejectsWithoutPartialWrite(t *testing.T) {
	for _, payload := range []map[string]any{
		{"notification_email_default_locale": "de"},
		{"subscription_expired_admin_notify_emails": []service.NotifyEmailEntry{{Email: "invalid", Verified: true}}},
		{"subscription_expired_admin_notify_emails": []service.NotifyEmailEntry{{Email: "ops@example.com\r\nBcc: bad@example.com", Verified: true}}},
	} {
		t.Run("invalid notification setting", func(t *testing.T) {
			stored := map[string]string{service.SettingKeySiteName: "unchanged"}
			h, repo := newStepUpSwitchTestHandler(t, stored)
			payload["site_name"] = "must not write"
			rec := doUpdateSettings(t, h, payload, nil)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			require.Equal(t, map[string]string{service.SettingKeySiteName: "unchanged"}, repo.values)
			require.Nil(t, repo.lastUpdates)
		})
	}
}
