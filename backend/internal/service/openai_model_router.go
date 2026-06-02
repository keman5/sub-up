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
}

func (s *OpenAIGatewayService) isOpenAIModelRouterEnabled() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.ModelRouter.Enabled
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
	decision := openAIModelRouterDecision{
		Enabled:       s.isOpenAIModelRouterEnabled(),
		Tier:          openAIModelRouterTierEconomy,
		UpstreamModel: strings.TrimSpace(requestedModel),
		Reason:        "router_disabled",
	}
	if !decision.Enabled || account == nil || account.Platform != PlatformOpenAI {
		return decision
	}

	cfg := s.cfg.Gateway.ModelRouter
	defaultModel := strings.TrimSpace(cfg.DefaultModel)
	balancedModel := strings.TrimSpace(cfg.BalancedModel)
	premiumModel := strings.TrimSpace(cfg.PremiumModel)
	if defaultModel == "" {
		defaultModel = strings.TrimSpace(requestedModel)
	}
	if balancedModel == "" {
		balancedModel = defaultModel
	}
	if premiumModel == "" {
		premiumModel = balancedModel
	}

	meta := openAIModelRouterRequestMeta{
		RequestedModel: strings.TrimSpace(requestedModel),
		ImageIntent:    imageIntent,
		ComplexText:    isOpenAIModelRouterComplexText(reqBody, rawBody, cfg.ComplexInputMinChars, cfg.ComplexInputMinItems),
	}

	state := s.getOpenAIModelRouterState(ctx, sessionHash)
	nowUnix := time.Now().Unix()
	if state.UpdatedAtUnix == 0 {
		state.UpdatedAtUnix = nowUnix
	}
	if state.Tier == "" {
		state.Tier = openAIModelRouterTierEconomy
	}

	pressure := openAIModelRouterPressureFromAccount(account, cfg)
	targetTier := openAIModelRouterTierEconomy
	reason := "economy_default"

	if meta.ImageIntent && cfg.ImageOrVisionForcePremium {
		targetTier = openAIModelRouterTierPremium
		reason = "image_or_vision_force_premium"
	} else if meta.ComplexText {
		if pressure == openAIModelRouterPressureHigh || pressure == openAIModelRouterPressureMedium {
			targetTier = openAIModelRouterTierBalanced
			reason = "complex_text_pressure_balanced"
		} else {
			targetTier = openAIModelRouterTierPremium
			reason = "complex_text_promote_premium"
		}
	} else if pressure == openAIModelRouterPressureLow {
		targetTier = openAIModelRouterTierEconomy
		reason = "low_remaining_budget_economy"
	} else if pressure == openAIModelRouterPressureMedium {
		targetTier = openAIModelRouterTierEconomy
		reason = "medium_remaining_budget_economy"
	}

	// Capability-failure escalation: promote one tier in cooldown-safe window.
	if state.CapabilityFailureConsecutive >= cfg.CapabilityErrorEscalateConsecutiveFailures &&
		!openAIModelRouterEscalationCooldown(state, nowUnix, cfg.EscalateCooldownSeconds) {
		targetTier = openAIModelRouterPromoteOneTier(targetTier)
		state.LastEscalatedAtUnix = nowUnix
		state.CapabilityFailureConsecutive = 0
		reason = "capability_failure_escalation"
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

	state.Tier = targetTier
	state.UpdatedAtUnix = nowUnix
	s.setOpenAIModelRouterState(ctx, sessionHash, state, time.Duration(cfg.SessionRouteTTLSeconds)*time.Second)
	return decision
}

func (s *OpenAIGatewayService) onOpenAIModelRouterResult(
	ctx context.Context,
	sessionHash string,
	err error,
) {
	if !s.isOpenAIModelRouterEnabled() || strings.TrimSpace(sessionHash) == "" {
		return
	}
	state := s.getOpenAIModelRouterState(ctx, sessionHash)
	if state.Tier == "" {
		return
	}

	if isOpenAIModelRouterCapabilityError(err) {
		state.CapabilityFailureConsecutive++
	} else {
		state.CapabilityFailureConsecutive = 0
	}
	state.UpdatedAtUnix = time.Now().Unix()
	ttl := time.Duration(s.cfg.Gateway.ModelRouter.SessionRouteTTLSeconds) * time.Second
	s.setOpenAIModelRouterState(ctx, sessionHash, state, ttl)
}

func isOpenAIModelRouterCapabilityError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "not supported") ||
		strings.Contains(msg, "only supported") ||
		strings.Contains(msg, "unsupported") ||
		strings.Contains(msg, "requires")
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
	if len(rawBody) > 0 && gjson.ValidBytes(rawBody) {
		if input := gjson.GetBytes(rawBody, "input"); input.Exists() {
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
			charCount += len(instructions)
		}
	}
	if reqBody != nil {
		if input, ok := reqBody["input"].([]any); ok {
			itemCount += len(input)
			for _, v := range input {
				charCount += len(openAIModelRouterJSONString(v))
			}
		}
		if instructions, ok := reqBody["instructions"].(string); ok {
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
	tier := openAIModelRouterTierEconomy
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
