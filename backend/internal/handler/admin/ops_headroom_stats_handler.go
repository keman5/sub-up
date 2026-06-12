package admin

import (
	"errors"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// GetHeadroomStats returns lightweight Headroom token-savings stats.
// GET /api/v1/admin/ops/headroom/stats
func (h *OpsHandler) GetHeadroomStats(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	stats, err := h.opsService.GetHeadroomStats(c.Request.Context())
	if err != nil {
		if errors.Is(err, service.ErrHeadroomStatsDisabled) {
			response.Error(c, http.StatusServiceUnavailable, "Headroom stats disabled")
			return
		}
		response.Error(c, http.StatusBadGateway, err.Error())
		return
	}
	response.Success(c, stats)
}
