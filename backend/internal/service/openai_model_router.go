package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

type openAIModelRouterPressure string

const (
	openAIModelRouterPressureLow    openAIModelRouterPressure = "low"
	openAIModelRouterPressureMedium openAIModelRouterPressure = "medium"
	openAIModelRouterPressureHigh   openAIModelRouterPressure = "high"
)

type openAIModelRouterTier string

const (
	openAIModelRouterTierEconomy  openAIModelRouterTier = "economy"
	openAIModelRouterTierBalanced openAIModelRouterTier = "balanced"
	openAIModelRouterTierPremium  openAIModelRouterTier = "premium"
)

const (
	openAIModelRouterOAuthModePassthrough   = "passthrough"
	openAIModelRouterOAuthModeAdaptiveCodex = "adaptive_codex"
)

type openAIModelRouterRouteState struct {
	Tier                         openAIModelRouterTier `json:"tier"`
	CapabilityFailureConsecutive int                   `json:"capability_failure_consecutive"`
	LastEscalatedAtUnix          int64                 `json:"last_escalated_at_unix"`
	UpdatedAtUnix                int64                 `json:"updated_at_unix"`
}

type openAIModelRouterDecision struct {
	Enabled       bool
	Tier          openAIModelRouterTier
	UpstreamModel string
	Reason        string
}

type openAIModelRouterRequestMeta struct {
	RequestedModel string
	ImageIntent    bool
	ComplexText    bool
	PremiumText    bool
}

type openAIModelRouteEvalInput struct {
	SessionHash    string
	Account        *Account
	RequestedModel string
	RequestMeta    openAIModelRouterRequestMeta
}

type openAIModelRouteEvalCtxKey struct{}

func withOpenAIModelRouteEvalInput(ctx context.Context, input *openAIModelRouteEvalInput) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if input == nil {
		return ctx
	}
	return context.WithValue(ctx, openAIModelRouteEvalCtxKey{}, input)
}

func (s *OpenAIGatewayService) WithModelRouteRequestContext(
	ctx context.Context,
	sessionHash string,
	requestedModel string,
	imageIntent bool,
	reqBody map[string]any,
	rawBody []byte,
) context.Context {
	if s == nil || s.cfg == nil {
		return ctx
	}
	return withOpenAIModelRouteEvalInput(ctx, &openAIModelRouteEvalInput{
		SessionHash:    sessionHash,
		RequestedModel: requestedModel,
		RequestMeta: openAIModelRouterRequestMeta{
			RequestedModel: strings.TrimSpace(requestedModel),
			ImageIntent:    imageIntent,
			ComplexText: isOpenAIModelRouterComplexText(
				reqBody,
				rawBody,
				s.cfg.Gateway.ModelRouter.ComplexInputMinChars,
				0,
			),
			PremiumText: isOpenAIModelRouterComplexText(
				reqBody,
				rawBody,
				s.cfg.Gateway.ModelRouter.PremiumInputMinChars,
				s.cfg.Gateway.ModelRouter.PremiumInputMinItems,
			),
		},
	})
}

func (s *OpenAIGatewayService) isOpenAIModelRouterEnabled() bool {
	if s == nil || s.cfg == nil || !s.cfg.Gateway.ModelRouter.Enabled {
		return false
	}
	return s.isOpenAIAdvancedSchedulerEnabled(context.Background())
}

func (s *OpenAIGatewayService) decideOpenAIModelRoute(
	ctx context.Context,
	sessionHash string,
	account *Account,
	reqBody map[string]any,
	rawBody []byte,
	requestedModel string,
	imageIntent bool,
) openAIModelRouterDecision {
	if s == nil || s.cfg == nil {
		return openAIModelRouterDecision{
			Enabled:       false,
			Tier:          openAIModelRouterTierEconomy,
			UpstreamModel: strings.TrimSpace(requestedModel),
			Reason:        "router_disabled",
		}
	}
	meta := openAIModelRouterRequestMeta{
		RequestedModel: strings.TrimSpace(requestedModel),
		ImageIntent:    imageIntent,
		ComplexText:    isOpenAIModelRouterComplexText(reqBody, rawBody, s.cfg.Gateway.ModelRouter.ComplexInputMinChars, 0),
		PremiumText:    isOpenAIModelRouterComplexText(reqBody, rawBody, s.cfg.Gateway.ModelRouter.PremiumInputMinChars, s.cfg.Gateway.ModelRouter.PremiumInputMinItems),
	}
	return s.evaluateOpenAIModelRoute(ctx, openAIModelRouteEvalInput{
		SessionHash:    sessionHash,
		Account:        account,
		RequestedModel: requestedModel,
		RequestMeta:    meta,
	})
}

func (s *OpenAIGatewayService) evaluateOpenAIModelRoute(
	ctx context.Context,
	input openAIModelRouteEvalInput,
) openAIModelRouterDecision {
	decision := openAIModelRouterDecision{
		Enabled:       s.isOpenAIModelRouterEnabled(),
		Tier:          openAIModelRouterTierEconomy,
		UpstreamModel: strings.TrimSpace(input.RequestedModel),
		Reason:        "router_disabled",
	}
	if !decision.Enabled || input.Account == nil || input.Account.Platform != PlatformOpenAI {
		return decision
	}

	cfg := s.cfg.Gateway.ModelRouter
	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	balancedModel := strings.TrimSpace(cfg.BalancedModel)
	premiumModel := strings.TrimSpace(cfg.PremiumModel)
	if defaultModel == "" {
		defaultModel = strings.TrimSpace(input.RequestedModel)
	}
	if balancedModel == "" {
		balancedModel = defaultModel
	}
	if premiumModel == "" {
		premiumModel = balancedModel
	}
	meta := input.RequestMeta
	if strings.TrimSpace(meta.RequestedModel) == "" {
		meta.RequestedModel = strings.TrimSpace(input.RequestedModel)
	}
	requestedCanonical := normalizeCodexModel(meta.RequestedModel)
	if requestedCanonical == "gpt-5.3-codex-spark" {
		if explicitDecision, ok := decideExplicitSparkPreferredRoute(input.Account, requestedCanonical, defaultModel); ok {
			return explicitDecision
		}
	}
	if openAIModelRouterShouldPassthroughAccount(input.Account, cfg) && (!meta.ImageIntent || !cfg.ImageOrVisionForcePremium) {
		decision.Tier = openAIModelRouterTierPremium
		decision.UpstreamModel = strings.TrimSpace(input.RequestedModel)
		if decision.UpstreamModel == "" {
			decision.UpstreamModel = premiumModel
		}
		decision.Reason = "oauth_account_passthrough"
		return decision
	}
	if !meta.ImageIntent {
		if explicitDecision, ok := decideExplicitSparkPreferredRoute(input.Account, strings.TrimSpace(input.RequestedModel), defaultModel); ok {
			return explicitDecision
		}
	}

	state := s.getOpenAIModelRouterState(ctx, input.SessionHash)
	nowUnix := time.Now().Unix()
	if state.UpdatedAtUnix == 0 {
		state.UpdatedAtUnix = nowUnix
	}
	if state.Tier == "" {
		state.Tier = openAIModelRouterTierEconomy
	}

	pressure := openAIModelRouterPressureFromAccount(input.Account, cfg)
	targetTier := openAIModelRouterTierEconomy
	reason := "economy_default"

	if meta.ImageIntent {
		if cfg.ImageOrVisionForcePremium {
			targetTier = openAIModelRouterTierPremium
			reason = "image_or_vision_force_premium"
		} else {
			targetTier = openAIModelRouterPromoteOneTier(targetTier)
			reason = "image_intent_promotion"
		}
	} else if meta.PremiumText {
		targetTier = openAIModelRouterTierPremium
		reason = "premium_text_promote_premium"
	} else if pressure == openAIModelRouterPressureLow {
		targetTier = openAIModelRouterTierEconomy
		reason = "low_remaining_budget_economy"
	} else if pressure == openAIModelRouterPressureMedium {
		if meta.ComplexText {
			targetTier = openAIModelRouterTierBalanced
			reason = "complex_text_pressure_balanced"
		} else {
			targetTier = openAIModelRouterTierEconomy
			reason = "medium_remaining_budget_economy"
		}
	} else if meta.ComplexText {
		targetTier = openAIModelRouterTierBalanced
		reason = "complex_text_promote_balanced"
	}

	// Capability-failure escalation: promote one tier in cooldown-safe window.
	if cfg.CapabilityErrorEscalateConsecutiveFailures > 0 &&
		state.CapabilityFailureConsecutive >= cfg.CapabilityErrorEscalateConsecutiveFailures &&
		!openAIModelRouterEscalationCooldown(state, nowUnix, cfg.EscalateCooldownSeconds) {
		targetTier = openAIModelRouterPromoteOneTier(targetTier)
		state.LastEscalatedAtUnix = nowUnix
		state.CapabilityFailureConsecutive = 0
		reason = "capability_failure_escalation"
	}

	if meta.ImageIntent {
		cappedTier := openAIModelRouterCapImageIntentTier(targetTier, requestedCanonical, defaultModel, balancedModel, premiumModel)
		if cappedTier != targetTier {
			reason = "image_intent_capped_to_selected_model"
		}
		targetTier = cappedTier
	}

	switch targetTier {
	case openAIModelRouterTierPremium:
		decision.UpstreamModel = premiumModel
	case openAIModelRouterTierBalanced:
		decision.UpstreamModel = balancedModel
	default:
		decision.UpstreamModel = defaultModel
		targetTier = openAIModelRouterTierEconomy
	}
	decision.Tier = targetTier
	decision.Reason = reason
	state.Tier = decision.Tier
	state.UpdatedAtUnix = nowUnix
	s.setOpenAIModelRouterState(ctx, input.SessionHash, state, time.Duration(cfg.SessionRouteTTLSeconds)*time.Second)
	return decision
}

func decideExplicitSparkPreferredRoute(account *Account, requestedModel string, defaultModel string) (openAIModelRouterDecision, bool) {
	requestedCanonical := normalizeCodexModel(requestedModel)
	if requestedCanonical == "" {
		return openAIModelRouterDecision{}, false
	}
	if normalizeCodexModel(defaultModel) != "gpt-5.3-codex-spark" {
		return openAIModelRouterDecision{}, false
	}
	if requestedCanonical == "gpt-5.3-codex-spark" {
		return openAIModelRouterDecision{
			Enabled:       true,
			Tier:          openAIModelRouterTierEconomy,
			UpstreamModel: defaultModel,
			Reason:        "explicit_spark_requested",
		}, true
	}
	if !isExplicitGPT5MinorModel(requestedCanonical) {
		return openAIModelRouterDecision{}, false
	}

	if openAIModelRouterSparkRemainingPercent(account) > 5 {
		return openAIModelRouterDecision{
			Enabled:       true,
			Tier:          openAIModelRouterTierEconomy,
			UpstreamModel: defaultModel,
			Reason:        "spark_remaining_over_threshold",
		}, true
	}

	return openAIModelRouterDecision{
		Enabled:       true,
		Tier:          openAIModelRouterTierBalanced,
		UpstreamModel: requestedCanonical,
		Reason:        "spark_remaining_below_threshold_use_requested",
	}, true
}

func openAIModelRouterTierRank(t openAIModelRouterTier) int {
	switch t {
	case openAIModelRouterTierPremium:
		return 3
	case openAIModelRouterTierBalanced:
		return 2
	default:
		return 1
	}
}

func openAIModelRouterCapImageIntentTier(
	targetTier openAIModelRouterTier,
	requestedCanonical string,
	defaultModel string,
	balancedModel string,
	premiumModel string,
) openAIModelRouterTier {
	capVersion, ok := openAIGPT5MinorVersion(requestedCanonical)
	if !ok {
		return targetTier
	}

	bestTier := openAIModelRouterTierEconomy
	bestVersion := -1
	bestRank := openAIModelRouterTierRank(bestTier)

	candidates := []struct {
		tier  openAIModelRouterTier
		model string
	}{
		{openAIModelRouterTierEconomy, defaultModel},
		{openAIModelRouterTierBalanced, balancedModel},
		{openAIModelRouterTierPremium, premiumModel},
	}

	for _, candidate := range candidates {
		version, ok := openAIGPT5MinorVersion(candidate.model)
		if !ok || version > capVersion {
			continue
		}
		rank := openAIModelRouterTierRank(candidate.tier)
		if version > bestVersion || (version == bestVersion && rank > bestRank) {
			bestVersion = version
			bestRank = rank
			bestTier = candidate.tier
		}
	}

	if bestVersion < 0 {
		return targetTier
	}
	if openAIModelRouterTierRank(targetTier) <= bestRank {
		return targetTier
	}
	return bestTier
}

func openAIGPT5MinorVersion(model string) (int, bool) {
	normalized := normalizeCodexModel(strings.TrimSpace(model))
	if normalized == "" {
		return 0, false
	}
	if !strings.HasPrefix(normalized, "gpt-5.") {
		return 0, false
	}
	suffix := strings.TrimPrefix(normalized, "gpt-5.")
	if suffix == "" {
		return 0, false
	}

	version := 0
	found := false
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			break
		}
		found = true
		version = version*10 + int(ch-'0')
	}
	if !found {
		return 0, false
	}
	return version, true
}

func isExplicitGPT5MinorModel(model string) bool {
	if !strings.HasPrefix(model, "gpt-5.") {
		return false
	}
	suffix := strings.TrimPrefix(model, "gpt-5.")
	if suffix == "" {
		return false
	}
	for _, ch := range suffix {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func openAIModelRouterSparkRemainingPercent(account *Account) float64 {
	if account == nil || len(account.Extra) == 0 {
		return 0
	}
	remaining5h := 100.0 - readOpenAIQuotaUsedPercent(account.Extra, "5h")
	remaining7d := 100.0 - readOpenAIQuotaUsedPercent(account.Extra, "7d")
	if remaining5h <= 0 && remaining7d <= 0 {
		return 0
	}
	if remaining5h <= 0 {
		return remaining7d
	}
	if remaining7d <= 0 {
		return remaining5h
	}
	if remaining5h < remaining7d {
		return remaining5h
	}
	return remaining7d
}

func openAIModelRouterShouldPassthroughAccount(account *Account, cfg config.GatewayModelRouterConfig) bool {
	if account == nil || account.Platform != PlatformOpenAI || account.Type != AccountTypeOAuth {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.OAuthMode))
	return mode == "" || mode == openAIModelRouterOAuthModePassthrough
}

func openAIModelRouterPromoteOneTier(tier openAIModelRouterTier) openAIModelRouterTier {
	switch tier {
	case openAIModelRouterTierEconomy:
		return openAIModelRouterTierBalanced
	case openAIModelRouterTierBalanced:
		return openAIModelRouterTierPremium
	default:
		return openAIModelRouterTierPremium
	}
}

func openAIModelRouterEscalationCooldown(state openAIModelRouterRouteState, nowUnix int64, cooldownSeconds int) bool {
	if cooldownSeconds <= 0 || state.LastEscalatedAtUnix <= 0 {
		return false
	}
	return nowUnix-state.LastEscalatedAtUnix < int64(cooldownSeconds)
}

func openAIModelRouterPressureFromAccount(account *Account, cfg config.GatewayModelRouterConfig) openAIModelRouterPressure {
	if account == nil || len(account.Extra) == 0 {
		return openAIModelRouterPressureHigh
	}
	remaining5h := 100.0 - readOpenAIQuotaUsedPercent(account.Extra, "5h")
	remaining7d := 100.0 - readOpenAIQuotaUsedPercent(account.Extra, "7d")
	remaining := 100.0
	if remaining5h > 0 && remaining7d > 0 {
		if remaining5h < remaining7d {
			remaining = remaining5h
		} else {
			remaining = remaining7d
		}
	} else if remaining5h > 0 {
		remaining = remaining5h
	} else if remaining7d > 0 {
		remaining = remaining7d
	}

	if remaining <= cfg.PressureLowRemainingPercent {
		return openAIModelRouterPressureLow
	}
	if remaining <= cfg.PressureMediumRemainingPercent {
		return openAIModelRouterPressureMedium
	}
	return openAIModelRouterPressureHigh
}

func isOpenAIModelRouterComplexText(reqBody map[string]any, rawBody []byte, minChars, minItems int) bool {
	if minChars <= 0 && minItems <= 0 {
		return false
	}
	itemCount := 0
	charCount := 0
	rawHadInput := false
	rawHadInstructions := false
	if len(rawBody) > 0 && gjson.ValidBytes(rawBody) {
		if input := gjson.GetBytes(rawBody, "input"); input.Exists() {
			rawHadInput = true
			if input.IsArray() {
				arr := input.Array()
				itemCount += len(arr)
				for _, item := range arr {
					charCount += len(strings.TrimSpace(item.Raw))
					charCount += len(strings.TrimSpace(item.String()))
				}
			} else {
				itemCount++
				charCount += len(strings.TrimSpace(input.Raw))
				charCount += len(strings.TrimSpace(input.String()))
			}
		}
		if instructions := strings.TrimSpace(gjson.GetBytes(rawBody, "instructions").String()); instructions != "" {
			rawHadInstructions = true
			charCount += len(instructions)
		}
	}
	if reqBody != nil {
		if input, ok := reqBody["input"].([]any); ok && !rawHadInput {
			itemCount += len(input)
			for _, v := range input {
				charCount += len(openAIModelRouterJSONString(v))
			}
		}
		if instructions, ok := reqBody["instructions"].(string); ok && !rawHadInstructions {
			charCount += len(strings.TrimSpace(instructions))
		}
	}
	return (minItems > 0 && itemCount >= minItems) || (minChars > 0 && charCount >= minChars)
}

func openAIModelRouterJSONString(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *OpenAIGatewayService) getOpenAIModelRouterState(ctx context.Context, sessionHash string) openAIModelRouterRouteState {
	if s == nil || s.cache == nil {
		return openAIModelRouterRouteState{}
	}
	key := s.openAIModelRouterSessionKey(sessionHash)
	if key == "" {
		return openAIModelRouterRouteState{}
	}
	rawID, err := s.cache.GetSessionAccountID(ctx, 0, key)
	if err != nil || rawID <= 0 {
		return openAIModelRouterRouteState{}
	}
	decoded := decodeOpenAIModelRouterState(rawID)
	return decoded
}

func (s *OpenAIGatewayService) setOpenAIModelRouterState(ctx context.Context, sessionHash string, state openAIModelRouterRouteState, ttl time.Duration) {
	if s == nil || s.cache == nil || ttl <= 0 {
		return
	}
	key := s.openAIModelRouterSessionKey(sessionHash)
	if key == "" {
		return
	}
	encoded := encodeOpenAIModelRouterState(state)
	if encoded <= 0 {
		return
	}
	_ = s.cache.SetSessionAccountID(ctx, 0, key, encoded, ttl)
}

func (s *OpenAIGatewayService) openAIModelRouterSessionKey(sessionHash string) string {
	trimmed := strings.TrimSpace(sessionHash)
	if trimmed == "" {
		return ""
	}
	return "openai_model_router:" + trimmed
}

func encodeOpenAIModelRouterState(state openAIModelRouterRouteState) int64 {
	// decimal packing: TTFFFF
	// T: tier (1=economy,2=balanced,3=premium)
	// F: consecutive capability failures (0-9999)
	tier := int64(1)
	switch state.Tier {
	case openAIModelRouterTierBalanced:
		tier = 2
	case openAIModelRouterTierPremium:
		tier = 3
	}
	failures := state.CapabilityFailureConsecutive
	if failures < 0 {
		failures = 0
	}
	if failures > 9999 {
		failures = 9999
	}
	return tier*10000 + int64(failures)
}

func decodeOpenAIModelRouterState(raw int64) openAIModelRouterRouteState {
	if raw <= 0 {
		return openAIModelRouterRouteState{}
	}
	tierCode := raw / 10000
	failures := int(raw % 10000)
	var tier openAIModelRouterTier
	switch tierCode {
	case 2:
		tier = openAIModelRouterTierBalanced
	case 3:
		tier = openAIModelRouterTierPremium
	default:
		tier = openAIModelRouterTierEconomy
	}
	return openAIModelRouterRouteState{
		Tier:                         tier,
		CapabilityFailureConsecutive: failures,
	}
}
