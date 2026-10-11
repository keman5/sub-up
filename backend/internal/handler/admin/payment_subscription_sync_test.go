package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func TestSubscriptionSyncAuthorization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		email, method string
		allowed       bool
	}{
		{"macseek@upit.top", "jwt", true},
		{"MACSEEK@UPIT.TOP", "jwt", true},
		{"other@upit.top", "jwt", false},
		{"macseek@upit.top", "admin_api_key", false},
		{"", "", false},
	} {
		t.Run(tc.email+tc.method, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
			c.Set(middleware.ContextKeyAuthEmail, tc.email)
			c.Set("auth_method", tc.method)
			if canManageSubscriptionSync(c) != tc.allowed {
				t.Fatal("unexpected permission")
			}
			if !tc.allowed {
				h := &PaymentHandler{}
				h.RequestSubscriptionSync(c)
				if w.Code != http.StatusForbidden {
					t.Fatalf("write: got %d", w.Code)
				}
			}
		})
	}
}
