//go:build unit

package admin

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func assertEmailPolicyResponse(t *testing.T, rec *httptest.ResponseRecorder, want []string) {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var payload struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Contains(t, payload.Data, "email_blocked_keywords")
	var got []string
	require.NoError(t, json.Unmarshal(payload.Data["email_blocked_keywords"], &got))
	require.Equal(t, want, got)
}

func TestUpdateSettingsEmailBlockedKeywords(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{})
	assertEmailPolicyResponse(t, doUpdateSettings(t, h, map[string]any{}, nil), []string{})
	assertEmailPolicyResponse(t, doUpdateSettings(t, h, map[string]any{
		"email_blocked_keywords": []string{"  Secret ", "secret", "", "  ", "禁止"},
	}, nil), []string{"Secret", "禁止"})
	require.JSONEq(t, `["Secret", "禁止"]`, repo.values["email_blocked_keywords"])
	assertEmailPolicyResponse(t, doUpdateSettings(t, h, map[string]any{"site_name": "Example"}, nil), []string{"Secret", "禁止"})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings", nil)
	h.GetSettings(c)
	assertEmailPolicyResponse(t, rec, []string{"Secret", "禁止"})
	assertEmailPolicyResponse(t, doUpdateSettings(t, h, map[string]any{"email_blocked_keywords": []string{}}, nil), []string{})
	require.Equal(t, "[]", repo.values["email_blocked_keywords"])
}

func TestUpdateSettingsEmailBlockedKeywordsValidation(t *testing.T) {
	for _, keywords := range [][]string{make([]string, 101), {strings.Repeat("x", 201)}} {
		h, repo := newStepUpSwitchTestHandler(t, map[string]string{"email_blocked_keywords": `["saved"]`})
		rec := doUpdateSettings(t, h, map[string]any{"email_blocked_keywords": keywords}, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, `["saved"]`, repo.values["email_blocked_keywords"])
	}
}

func TestSendTestEmailEnforcesSavedEmailBlockedKeywords(t *testing.T) {
	h, repo := newStepUpSwitchTestHandler(t, map[string]string{"email_blocked_keywords": `["test email"]`})
	h.emailService = service.NewEmailService(repo, nil)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	raw, err := json.Marshal(map[string]any{"email": "recipient@example.com", "smtp_host": "127.0.0.1", "smtp_port": port, "smtp_from_email": "sender@example.com", "email_blocked_keywords": []string{}})
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/send-test-email", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	h.SendTestEmail(c)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "email blocked by keyword policy")
	require.NotContains(t, rec.Body.String(), "recipient@example.com")
	require.NoError(t, listener.(*net.TCPListener).SetDeadline(time.Now().Add(20*time.Millisecond)))
	conn, acceptErr := listener.Accept()
	if conn != nil {
		conn.Close()
	}
	require.Nil(t, conn, "blocked admin test must not connect to SMTP")
	require.Error(t, acceptErr)
}
