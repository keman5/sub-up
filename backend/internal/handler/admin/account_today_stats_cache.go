package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var accountTodayStatsBatchCache = newSnapshotCache(30 * time.Second)

func buildAccountTodayStatsBatchCacheKey(accountIDs []int64, viewMode service.UsageViewMode) string {
	viewKey := "presentation"
	if viewMode == service.UsageViewRaw {
		viewKey = "raw"
	}
	if len(accountIDs) == 0 {
		return "accounts_today_stats:" + viewKey + ":empty"
	}
	var b strings.Builder
	b.Grow(len(accountIDs) * 6)
	_, _ = b.WriteString("accounts_today_stats:")
	_, _ = b.WriteString(viewKey)
	_ = b.WriteByte(':')
	for i, id := range accountIDs {
		if i > 0 {
			_ = b.WriteByte(',')
		}
		_, _ = b.WriteString(strconv.FormatInt(id, 10))
	}
	return b.String()
}
