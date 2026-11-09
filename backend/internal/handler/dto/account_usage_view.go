package dto

import "github.com/Wei-Shaw/sub2api/internal/service"

// AccountUsageInfoForViewer projects local account costs without changing the
// cached snapshot or the provider's quota windows and billing amounts.
func AccountUsageInfoForViewer(info *service.UsageInfo, mode service.UsageViewMode) *service.UsageInfo {
	if info == nil || mode == service.UsageViewRaw {
		return info
	}
	out := *info
	stats := func(s *service.WindowStats) *service.WindowStats {
		if s == nil {
			return nil
		}
		copy := *s
		copy.Cost, copy.StandardCost = copy.UserCost, copy.UserCost
		return &copy
	}
	window := func(w *service.UsageProgress) *service.UsageProgress {
		if w == nil {
			return nil
		}
		copy := *w
		copy.WindowStats = stats(w.WindowStats)
		return &copy
	}
	out.FiveHour = window(info.FiveHour)
	out.SevenDay = window(info.SevenDay)
	out.SevenDaySonnet = window(info.SevenDaySonnet)
	out.SevenDayFable = window(info.SevenDayFable)
	out.CodexPrimary = window(info.CodexPrimary)
	out.CodexSecondary = window(info.CodexSecondary)
	out.GeminiSharedDaily = window(info.GeminiSharedDaily)
	out.GeminiProDaily = window(info.GeminiProDaily)
	out.GeminiFlashDaily = window(info.GeminiFlashDaily)
	out.GeminiSharedMinute = window(info.GeminiSharedMinute)
	out.GeminiProMinute = window(info.GeminiProMinute)
	out.GeminiFlashMinute = window(info.GeminiFlashMinute)
	out.ThirtyDay = window(info.ThirtyDay)
	out.GrokLocalUsage = stats(info.GrokLocalUsage)
	out.GrokLocalUsage24h = stats(info.GrokLocalUsage24h)
	out.GrokLocalUsage7d = stats(info.GrokLocalUsage7d)
	out.GrokLocalUsageMonthly = stats(info.GrokLocalUsageMonthly)
	return &out
}
