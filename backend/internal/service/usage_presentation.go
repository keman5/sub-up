package service

const UsagePresentationThresholdTokens = 1000

type UsageViewMode int

const (
	UsageViewPresentation UsageViewMode = iota
	UsageViewRaw
)

func UsageViewModeForRole(role string) UsageViewMode {
	if role == RoleSuperAdmin {
		return UsageViewRaw
	}
	return UsageViewPresentation
}

func ResolvePresentationMultiplier(group *Group, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens int) float64 {
	return ResolvePresentationMultiplierWithImageOutput(group, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, 0)
}

func ResolvePresentationMultiplierWithImageOutput(group *Group, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, imageOutputTokens int) float64 {
	if group == nil || !group.UsageMultiplierEnabled {
		return 1
	}
	billableOutputTokens := outputTokens
	if imageOutputTokens > billableOutputTokens {
		billableOutputTokens = imageOutputTokens
	}
	total := inputTokens + billableOutputTokens + cacheCreationTokens + cacheReadTokens
	if total < UsagePresentationThresholdTokens {
		return 1
	}
	if group.UsageMultiplier <= 0 {
		return 1
	}
	return group.UsageMultiplier
}

func gatewayResponsePresentationGroup(c interface{ Get(string) (any, bool) }) *Group {
	if group := apiKeyGroup(getAPIKeyFromContext(c)); group != nil {
		return group
	}
	return &Group{
		UsageMultiplierEnabled: true,
		UsageMultiplier:        2,
	}
}

func ResolveGatewayResponsePresentationMultiplier(c interface{ Get(string) (any, bool) }, inputTokens, outputTokens, cacheCreationTokens, cacheReadTokens, imageOutputTokens int) float64 {
	return ResolvePresentationMultiplierWithImageOutput(
		gatewayResponsePresentationGroup(c),
		inputTokens,
		outputTokens,
		cacheCreationTokens,
		cacheReadTokens,
		imageOutputTokens,
	)
}

func UsagePresentationMultiplier(log *UsageLog) float64 {
	if log == nil || log.PresentationMultiplier <= 0 {
		return 1
	}
	return log.PresentationMultiplier
}

func UsageLogForView(log *UsageLog, mode UsageViewMode) UsageLog {
	if log == nil {
		return UsageLog{}
	}
	out := *log
	if mode == UsageViewRaw {
		return out
	}
	multiplier := UsagePresentationMultiplier(log)
	if multiplier == 1 {
		out.RateMultiplier = 1
		return out
	}
	out.InputTokens = multiplyInt(out.InputTokens, multiplier)
	out.OutputTokens = multiplyInt(out.OutputTokens, multiplier)
	out.CacheCreationTokens = multiplyInt(out.CacheCreationTokens, multiplier)
	out.CacheReadTokens = multiplyInt(out.CacheReadTokens, multiplier)
	out.CacheCreation5mTokens = multiplyInt(out.CacheCreation5mTokens, multiplier)
	out.CacheCreation1hTokens = multiplyInt(out.CacheCreation1hTokens, multiplier)
	out.ImageOutputTokens = multiplyInt(out.ImageOutputTokens, multiplier)
	out.InputCost *= multiplier
	out.OutputCost *= multiplier
	out.CacheCreationCost *= multiplier
	out.CacheReadCost *= multiplier
	out.ImageOutputCost *= multiplier
	out.TotalCost *= multiplier
	out.ActualCost *= multiplier
	out.RateMultiplier = 1
	return out
}

func multiplyInt(value int, multiplier float64) int {
	if value == 0 || multiplier == 1 {
		return value
	}
	return int(float64(value) * multiplier)
}
