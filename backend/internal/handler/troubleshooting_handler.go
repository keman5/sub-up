package handler

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type TroubleshootingHandler struct {
	service *service.TroubleshootingAssistantService
}

func NewTroubleshootingHandler(service *service.TroubleshootingAssistantService) *TroubleshootingHandler {
	return &TroubleshootingHandler{service: service}
}

type troubleshootingAnalyzeRequest struct {
	Message string `json:"message" binding:"required"`
}

type troubleshootingNotifyAdminRequest struct {
	Message   string `json:"message" binding:"required"`
	Diagnosis string `json:"diagnosis" binding:"required"`
}

func (h *TroubleshootingHandler) Analyze(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "Troubleshooting service not available")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req troubleshootingAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.Analyze(c.Request.Context(), service.TroubleshootingAnalyzeInput{
		UserID:  subject.UserID,
		Message: strings.TrimSpace(req.Message),
		Locale:  c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *TroubleshootingHandler) NotifyAdmin(c *gin.Context) {
	if h == nil || h.service == nil {
		response.InternalError(c, "Troubleshooting service not available")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req troubleshootingNotifyAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.service.NotifyAdmin(c.Request.Context(), service.TroubleshootingAdminNotifyInput{
		UserID:    subject.UserID,
		Message:   strings.TrimSpace(req.Message),
		Diagnosis: strings.TrimSpace(req.Diagnosis),
		Locale:    c.GetHeader("Accept-Language"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}
