//go:build unit

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettingHandler_PublicSettingsDoesNotExposeAdminNotifications(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &settingHandlerPublicRepoStub{values: map[string]string{
		service.SettingKeyNotificationEmailDefaultLocale:        "en",
		service.SettingKeySubscriptionExpiredAdminNotifyEnabled: "true",
		service.SettingKeySubscriptionExpiredAdminNotifyEmails:  `[{"email":"private-admin@example.com","verified":true}]`,
	}}
	h := NewSettingHandler(service.NewSettingService(repo, &config.Config{}), "test")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/settings/public", nil)
	h.GetPublicSettings(c)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	for _, key := range []string{"notification_email_default_locale", "subscription_expired_admin_notify_enabled", "subscription_expired_admin_notify_emails"} {
		require.NotContains(t, body.Data, key)
	}
	require.NotContains(t, rec.Body.String(), "private-admin@example.com")
}
