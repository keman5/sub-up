package admin

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

func canManageSubscriptionSync(c *gin.Context) bool {
	return c.GetString("auth_method") == "jwt" && strings.EqualFold(c.GetString(middleware.ContextKeyAuthEmail), "macseek@upit.top")
}

func (h *PaymentHandler) ListSubscriptionSyncRequests(c *gin.Context) {
	if !canManageSubscriptionSync(c) {
		response.Forbidden(c, "无权操作订阅同步")
		return
	}
	requests, err := h.paymentService.ListSubscriptionSyncRequests(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, requests)
}

func (h *PaymentHandler) RequestSubscriptionSync(c *gin.Context) {
	if !canManageSubscriptionSync(c) {
		response.Forbidden(c, "无权操作订阅同步")
		return
	}
	orderID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var body struct {
		Enabled *bool `json:"enabled" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "enabled 必须为布尔值")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Forbidden(c, "无权操作订阅同步")
		return
	}
	request, err := h.paymentService.RequestSubscriptionSync(c.Request.Context(), orderID, subject.UserID, *body.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, request)
}
