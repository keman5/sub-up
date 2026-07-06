package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type troubleshootingAIStub struct {
	answer string
	err    error
	calls  int
	report string
	hint   string
	locale string
}

func (s *troubleshootingAIStub) Diagnose(ctx context.Context, report string, localHint string, locale string) (string, int, error) {
	s.calls++
	s.report = report
	s.hint = localHint
	s.locale = locale
	if s.err != nil {
		return "", 2, s.err
	}
	return s.answer, 1, nil
}

type troubleshootingLimiterStub struct {
	err   error
	calls int
}

func (s *troubleshootingLimiterStub) Allow(ctx context.Context, userID int64) (*TroubleshootingLimitState, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &TroubleshootingLimitState{
		ShortWindowRemaining: 5,
		DailyRemaining:       19,
	}, nil
}

type troubleshootingEvidenceStub struct {
	evidence *TroubleshootingEvidence
	err      error
	calls    int
}

func (s *troubleshootingEvidenceStub) Collect(ctx context.Context, report string, locale string) (*TroubleshootingEvidence, error) {
	s.calls++
	return s.evidence, s.err
}

func TestTroubleshootingAssistantRejectsGeneralChat(t *testing.T) {
	ai := &troubleshootingAIStub{}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "帮我写一首关于夏天的诗",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 400, infraerrors.Code(err))
	require.Contains(t, err.Error(), "只能排查请求失败")
	require.Equal(t, 0, ai.calls)
}

func TestTroubleshootingAssistantAppliesUserRateLimitBeforeAI(t *testing.T) {
	ai := &troubleshootingAIStub{}
	limiter := &troubleshootingLimiterStub{
		err: infraerrors.TooManyRequests("TROUBLESHOOTING_RATE_LIMITED", "故障排查次数已达到限制，请稍后再试"),
	}
	svc := NewTroubleshootingAssistantService(ai, limiter, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST https://api.example.com/v1/responses 返回 503 Service Unavailable",
	})

	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, 429, infraerrors.Code(err))
	require.Equal(t, 1, limiter.calls)
	require.Equal(t, 0, ai.calls)
}

func TestTroubleshootingAssistantReturnsLocalDiagnosisWhenAIUnavailable(t *testing.T) {
	ai := &troubleshootingAIStub{err: errors.New("all accounts failed")}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "unexpected status 503 Service Unavailable, url: https://ap1.upit.top/51Token/v1/responses, request id: abc",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "rules", result.Source)
	require.True(t, result.NeedsAdmin)
	require.True(t, result.AIAttempted)
	require.False(t, result.AIAvailable)
	require.Equal(t, 2, result.AIAttempts)
	require.Contains(t, result.Answer, "503")
	require.Contains(t, result.Answer, "联系管理员")
	require.Contains(t, result.Answer, "request id")
}

func TestTroubleshootingAssistantDoesNotAppendAdminNoticeWhenAIUnavailableAndNotNeeded(t *testing.T) {
	ai := &troubleshootingAIStub{err: errors.New("all accounts failed")}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses 返回 401 Unauthorized token revoked",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "rules", result.Source)
	require.False(t, result.NeedsAdmin)
	require.True(t, result.AIAttempted)
	require.False(t, result.AIAvailable)
	require.Contains(t, result.Answer, "AI 诊断暂不可用")
	require.NotContains(t, result.Answer, "联系管理员")
	require.NotContains(t, result.Answer, "建议提供给管理员的信息")
}

func TestTroubleshootingAssistantUsesExactLogEvidenceWithoutAI(t *testing.T) {
	ai := &troubleshootingAIStub{answer: "不应该调用 AI"}
	evidence := &troubleshootingEvidenceStub{
		evidence: &TroubleshootingEvidence{
			Confirmed:  true,
			Reason:     "上游返回 503：Service temporarily unavailable。",
			NeedsAdmin: true,
			Request: &TroubleshootingEvidenceRequest{
				RequestID:  "rid-503",
				StatusCode: 503,
				Phase:      "upstream",
				Model:      "gpt-5.5",
				Message:    "Service temporarily unavailable",
			},
			UserAction:  "请稍后重试；如果仍失败，把 request id 发给管理员。",
			AdminAction: "检查该 request id 对应的上游账号和代理链路日志。",
		},
	}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, evidence)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "unexpected status 503 Service Unavailable request id rid-503",
	})

	require.NoError(t, err)
	require.Equal(t, "rules", result.Source)
	require.True(t, result.NeedsAdmin)
	require.False(t, result.AIAttempted)
	require.False(t, result.AIAvailable)
	require.Equal(t, 0, ai.calls)
	require.Contains(t, result.Answer, "上游返回 503")
	require.NotContains(t, result.Answer, "可能原因包括")
	require.NotContains(t, result.Answer, "1. 上游账号池")
}

func TestTroubleshootingAssistantDirectlyReportsExpiredSubscription(t *testing.T) {
	ai := &troubleshootingAIStub{answer: "不应该调用 AI"}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses failed: 403 No active subscription found for this group, request id: sub-expired-1",
	})

	require.NoError(t, err)
	require.Equal(t, "rules", result.Source)
	require.True(t, result.NeedsAdmin)
	require.False(t, result.AIAttempted)
	require.Equal(t, 0, ai.calls)
	require.Contains(t, result.Answer, "订阅已过期")
	require.Contains(t, result.Answer, "请联系管理员续期")
	require.NotContains(t, result.Answer, "权限、风控或策略")
	require.NotContains(t, result.Answer, "可能")
}

func TestTroubleshootingEvidenceNoAvailableAccountsUsesClearConclusion(t *testing.T) {
	status := 503
	evidence := troubleshootingEvidenceFromRequestDetail(&OpsRequestDetail{
		Kind:       OpsRequestKindError,
		CreatedAt:  time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
		RequestID:  "no-account-1",
		StatusCode: &status,
		Phase:      "routing",
		Model:      "gpt-5.3-codex-spark",
		Message:    "No available accounts: no available accounts supporting model: gpt-5.3-codex-spark",
	}, troubleshootingLocaleChinese)

	require.True(t, evidence.Confirmed)
	require.True(t, evidence.NeedsAdmin)
	require.Contains(t, evidence.Reason, "系统当前没有可用账户")
	require.Contains(t, evidence.UserAction, "联系管理员")
	require.NotContains(t, evidence.Reason, "可能")
}

func TestTroubleshootingEvidenceLocalBusinessLimitUsesClearConclusion(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		wantReason string
		needsAdmin bool
	}{
		{
			name:       "insufficient_balance",
			statusCode: 403,
			message:    "Insufficient account balance",
			wantReason: "账户余额不足",
			needsAdmin: true,
		},
		{
			name:       "model_not_allowed",
			statusCode: 403,
			message:    "model gemini-2.5-pro not in whitelist",
			wantReason: "当前分组不允许使用该模型",
			needsAdmin: false,
		},
		{
			name:       "route_not_allowed",
			statusCode: 403,
			message:    "This group does not allow /v1/messages dispatch",
			wantReason: "当前分组不允许使用该接口或客户端类型",
			needsAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evidence := troubleshootingEvidenceFromRequestDetail(&OpsRequestDetail{
				Kind:       OpsRequestKindError,
				CreatedAt:  time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC),
				RequestID:  "local-business-" + tt.name,
				StatusCode: &tt.statusCode,
				Phase:      "request",
				Model:      "gpt-5.3-codex-spark",
				Message:    tt.message,
			}, troubleshootingLocaleChinese)

			require.True(t, evidence.Confirmed)
			require.Equal(t, tt.needsAdmin, evidence.NeedsAdmin)
			require.Contains(t, evidence.Reason, tt.wantReason)
			require.NotContains(t, evidence.Reason, "可能")
		})
	}
}

func TestTroubleshootingAssistantDirectlyReportsClearLocalBusinessErrors(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		wantReason  string
		wantAction  string
		needsAdmin  bool
		notContains string
	}{
		{
			name:        "api_key_required",
			message:     "401 API_KEY_REQUIRED API key is required in Authorization header",
			wantReason:  "缺少 API Key",
			wantAction:  "重新填写 API Key",
			needsAdmin:  false,
			notContains: "鉴权失败",
		},
		{
			name:        "invalid_api_key",
			message:     "401 INVALID_API_KEY Invalid API key",
			wantReason:  "API Key 无效",
			wantAction:  "重新复制 API Key",
			needsAdmin:  false,
			notContains: "鉴权失败",
		},
		{
			name:        "api_key_disabled",
			message:     "401 API_KEY_DISABLED API key is disabled",
			wantReason:  "API Key 已被停用",
			wantAction:  "联系管理员启用或更换 API Key",
			needsAdmin:  true,
			notContains: "鉴权失败",
		},
		{
			name:        "api_key_expired",
			message:     "403 API_KEY_EXPIRED API key 已过期",
			wantReason:  "API Key 已过期",
			wantAction:  "联系管理员续期或更换 API Key",
			needsAdmin:  true,
			notContains: "鉴权失败",
		},
		{
			name:        "insufficient_balance",
			message:     "403 INSUFFICIENT_BALANCE Insufficient account balance",
			wantReason:  "账户余额不足",
			wantAction:  "充值或联系管理员调整余额",
			needsAdmin:  true,
			notContains: "权限、风控或策略",
		},
		{
			name:        "subscription_daily_limit",
			message:     "429 DAILY_LIMIT_EXCEEDED daily usage limit exceeded",
			wantReason:  "订阅套餐日限额已用完",
			wantAction:  "等待限额窗口重置",
			needsAdmin:  false,
			notContains: "账号额度、套餐额度、RPM",
		},
		{
			name:        "rpm_limit",
			message:     "429 GROUP_RPM_EXCEEDED group requests-per-minute limit exceeded",
			wantReason:  "请求频率超过限制",
			wantAction:  "降低请求频率",
			needsAdmin:  false,
			notContains: "账号额度、套餐额度、RPM",
		},
		{
			name:        "image_generation_disabled",
			message:     "403 Image generation is not enabled for this group",
			wantReason:  "当前分组未启用图片生成",
			wantAction:  "切换到已启用图片生成的分组",
			needsAdmin:  false,
			notContains: "权限、风控或策略",
		},
		{
			name:        "model_whitelist",
			message:     "403 model claude-3-5-sonnet not in whitelist",
			wantReason:  "当前分组不允许使用该模型",
			wantAction:  "切换到模型列表中允许的模型",
			needsAdmin:  false,
			notContains: "请求参数或模型映射可能不匹配",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ai := &troubleshootingAIStub{answer: "不应该调用 AI"}
			svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

			result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
				UserID:  42,
				Message: "POST /v1/responses failed: " + tt.message + ", request id: clear-local-" + tt.name,
			})

			require.NoError(t, err)
			require.Equal(t, "rules", result.Source)
			require.Equal(t, tt.needsAdmin, result.NeedsAdmin)
			require.False(t, result.AIAttempted)
			require.Equal(t, 0, ai.calls)
			require.Contains(t, result.Answer, tt.wantReason)
			require.Contains(t, result.Answer, tt.wantAction)
			require.NotContains(t, result.Answer, tt.notContains)
			require.NotContains(t, result.Answer, "可能")
		})
	}
}

func TestTroubleshootingAssistantReportsRecoveredWhenNoLogAndAccountsAvailable(t *testing.T) {
	ai := &troubleshootingAIStub{}
	evidence := &troubleshootingEvidenceStub{
		evidence: &TroubleshootingEvidence{
			Confirmed:           true,
			Reason:              "系统未查到对应失败记录，且当前 OpenAI 账号池已有 2 个可用账号。",
			NeedsAdmin:          false,
			CurrentAvailable:    true,
			SchedulableAccounts: 2,
			UserAction:          "请重新发起请求；如果仍失败，请提供新的 request id。",
			AdminAction:         "暂不需要管理员处理，除非重试后继续失败。",
		},
	}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, evidence)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "unexpected status 503 Service Unavailable request id old-rid",
	})

	require.NoError(t, err)
	require.Equal(t, "rules", result.Source)
	require.False(t, result.NeedsAdmin)
	require.False(t, result.AIAttempted)
	require.Equal(t, 0, ai.calls)
	require.Contains(t, result.Answer, "请重新发起请求")
	require.NotContains(t, result.Answer, "常见于")
	require.NotContains(t, result.Answer, "是否需要联系管理员")
	require.NotContains(t, result.Answer, "暂时不需要联系管理员")
	require.NotContains(t, result.Answer, "建议提供给管理员的信息")
}

func TestTroubleshootingAssistantUsesAIForFailureReports(t *testing.T) {
	ai := &troubleshootingAIStub{
		answer: strings.Join([]string{
			"可能原因: 上游账号池暂时不可用。",
			"是否需要联系管理员: 需要。",
			"用户可自行检查项: 检查模型名和端点。",
			"建议提供给管理员的信息: 请求 ID。",
		}, "\n"),
	}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses 报错 502 Bad Gateway request id xyz",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "ai", result.Source)
	require.True(t, result.NeedsAdmin)
	require.True(t, result.AIAttempted)
	require.True(t, result.AIAvailable)
	require.Equal(t, 1, result.AIAttempts)
	require.Contains(t, result.Answer, "可能原因")
	require.Contains(t, ai.report, "502 Bad Gateway")
	require.Contains(t, ai.hint, "可能原因")
}

func TestTroubleshootingAssistantDoesNotMarkAIAnswerAdminWhenItSaysNotNeeded(t *testing.T) {
	ai := &troubleshootingAIStub{
		answer: strings.Join([]string{
			"可能原因: API Key 填写错误。",
			"是否需要联系管理员: 暂不需要联系管理员。",
			"用户可自行检查项: 重新复制 API Key。",
			"建议提供给管理员的信息: 若仍失败再提供 request id。",
		}, "\n"),
	}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses 返回 401 Unauthorized token revoked",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.NeedsAdmin)
	require.NotContains(t, result.Answer, "是否需要联系管理员")
	require.NotContains(t, result.Answer, "暂不需要联系管理员")
	require.NotContains(t, result.Answer, "建议提供给管理员的信息")
	require.NotContains(t, result.Answer, "若仍失败再提供 request id")
	require.Contains(t, result.Answer, "用户可自行检查项")
}

func TestTroubleshootingAssistantStripsEnglishAdminSectionsWhenNotNeeded(t *testing.T) {
	ai := &troubleshootingAIStub{
		answer: strings.Join([]string{
			"Diagnosis Result: The API Key is invalid.",
			"Contact Administrator: Not required.",
			"User Action: Copy the API Key again.",
			"Information for Administrator: Only needed if retry still fails.",
		}, "\n"),
	}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses returned 401 Unauthorized invalid api key",
		Locale:  "en",
	})

	require.NoError(t, err)
	require.False(t, result.NeedsAdmin)
	require.NotContains(t, result.Answer, "Contact Administrator")
	require.NotContains(t, result.Answer, "Information for Administrator")
	require.Contains(t, result.Answer, "User Action")
}

func TestTroubleshootingAssistantLocalDiagnosisUsesEnglishLocale(t *testing.T) {
	ai := &troubleshootingAIStub{err: errors.New("all accounts failed")}
	svc := NewTroubleshootingAssistantService(ai, &troubleshootingLimiterStub{}, nil)

	result, err := svc.Analyze(context.Background(), TroubleshootingAnalyzeInput{
		UserID:  42,
		Message: "POST /v1/responses returned 401 Unauthorized token revoked",
		Locale:  "en-US",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.NeedsAdmin)
	require.True(t, result.AIAttempted)
	require.Equal(t, "en", ai.locale)
	require.Contains(t, result.Answer, "Possible Cause")
	require.Contains(t, result.Answer, "User Checks")
	require.Contains(t, result.Answer, "AI diagnosis is temporarily unavailable")
	require.NotContains(t, result.Answer, "可能原因")
	require.NotContains(t, result.Answer, "联系管理员")
}

func TestBuildTroubleshootingAIPromptUsesEnglishLocale(t *testing.T) {
	prompt := buildTroubleshootingAIPrompt("POST /v1/responses returned 503", "local hint", "en-US")

	require.Contains(t, prompt, "You are the built-in 51Token troubleshooting assistant")
	require.Contains(t, prompt, "Respond in English")
	require.NotContains(t, prompt, "请用中文输出")
}

func TestTroubleshootingAIModelUsesMappedCodexModel(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.3-codex-spark": "gpt-5.3-codex-spark",
				"gpt-5.5":             "gpt-5.5",
			},
		},
	}

	require.Equal(t, "gpt-5.3-codex-spark", troubleshootingAIModelForAccount(account))
}

func TestBuildTroubleshootingAIRequestBodyOmitsMaxOutputTokensForOAuth(t *testing.T) {
	oauthBody, err := buildTroubleshootingAIRequestBody(&Account{ID: 16, Type: AccountTypeOAuth}, "hi")
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(oauthBody, "max_output_tokens").Exists())
	require.Equal(t, "gpt-5.3-codex-spark", gjson.GetBytes(oauthBody, "model").String())

	apiKeyBody, err := buildTroubleshootingAIRequestBody(&Account{ID: 17, Type: AccountTypeAPIKey}, "hi")
	require.NoError(t, err)
	require.Equal(t, int64(700), gjson.GetBytes(apiKeyBody, "max_output_tokens").Int())
}

func TestExtractTroubleshootingAITextFromResponseWrapper(t *testing.T) {
	body := []byte(`{"response":{"output":[{"type":"message","content":[{"type":"output_text","text":"可能原因\n上游账号不可用。"}]}]}}`)

	require.Equal(t, "可能原因\n上游账号不可用。", extractTroubleshootingAIText(body))
}

func TestExtractTroubleshootingAITextFromSSE(t *testing.T) {
	body := []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"可能原因\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"\\n上游账号不可用。\"}\n\n")

	require.Equal(t, "可能原因\n上游账号不可用。", extractTroubleshootingAIText(body))
}

func TestRedisTroubleshootingRateLimiterAllowsFiftyDailyRequests(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	limiter := ProvideTroubleshootingRateLimiter(rdb)
	now := time.Now().UTC()
	userID := int64(42)
	shortKey := fmt.Sprintf("troubleshooting:rate:user:%d:5m:%d", userID, now.Unix()/int64(troubleshootingShortWindowDuration/time.Second))
	dayKey := fmt.Sprintf("troubleshooting:rate:user:%d:day:%s", userID, now.Format("20060102"))
	mr.Set(shortKey, "0")
	mr.Set(dayKey, "49")

	state, err := limiter.Allow(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, 0, state.DailyRemaining)

	mr.Set(shortKey, "0")
	_, err = limiter.Allow(context.Background(), userID)
	require.Error(t, err)
	require.Equal(t, 429, infraerrors.Code(err))
}

func TestRedisTroubleshootingRateLimiterAllowsTenRequestsPerShortWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	limiter := ProvideTroubleshootingRateLimiter(rdb)
	now := time.Now().UTC()
	userID := int64(42)
	shortKey := fmt.Sprintf("troubleshooting:rate:user:%d:5m:%d", userID, now.Unix()/int64(troubleshootingShortWindowDuration/time.Second))
	dayKey := fmt.Sprintf("troubleshooting:rate:user:%d:day:%s", userID, now.Format("20060102"))
	mr.Set(shortKey, "9")
	mr.Set(dayKey, "0")

	state, err := limiter.Allow(context.Background(), userID)

	require.NoError(t, err)
	require.Equal(t, 0, state.ShortWindowRemaining)

	mr.Set(dayKey, "0")
	_, err = limiter.Allow(context.Background(), userID)
	require.Error(t, err)
	require.Equal(t, 429, infraerrors.Code(err))
}

func TestImageGenerationPermissionMessageLocalizesByAcceptLanguage(t *testing.T) {
	require.Equal(t, "当前分组未启用图片生成", ImageGenerationPermissionMessageForAcceptLanguage("zh-CN,zh;q=0.9"))
	require.Equal(t, ImageGenerationPermissionMessage(), ImageGenerationPermissionMessageForAcceptLanguage("en-US,en;q=0.9"))
}

func TestClientErrorMessageLocalizesOpenAIValidationMessages(t *testing.T) {
	tests := []struct {
		name           string
		acceptLanguage string
		message        string
		want           string
	}{
		{
			name:           "chinese_empty_body",
			acceptLanguage: "zh-CN,zh;q=0.9",
			message:        "Request body is empty",
			want:           "请求体不能为空",
		},
		{
			name:           "chinese_images_model",
			acceptLanguage: "zh-CN",
			message:        "images endpoint requires an image model, got \"gpt-5.3-codex-spark\"",
			want:           "图片接口需要使用图片模型，当前模型为 \"gpt-5.3-codex-spark\"",
		},
		{
			name:           "english_keeps_original",
			acceptLanguage: "en-US,en;q=0.9",
			message:        "Request body is empty",
			want:           "Request body is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClientErrorMessageForAcceptLanguage(tt.acceptLanguage, tt.message))
		})
	}
}

func TestTroubleshootingAssistantNotifyAdminUsesConfiguredNotificationEmailsAndDeduplicatesTenMinutes(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	require.NoError(t, settings.SetMultiple(ctx, smtpServer.settings()))
	require.NoError(t, settings.Set(ctx, SettingKeySiteName, "51token"))
	require.NoError(t, settings.Set(ctx, SettingKeyNotificationEmailDefaultLocale, "zh"))
	require.NoError(t, settings.Set(ctx, SettingKeyAccountQuotaNotifyEmails, MarshalNotifyEmails([]NotifyEmailEntry{
		{Email: "ops@example.com", Verified: true},
		{Email: "disabled@example.com", Verified: true, Disabled: true},
		{Email: "unverified@example.com", Verified: false},
	})))

	emailSvc := NewEmailService(settings, nil)
	notificationSvc := NewNotificationEmailService(settings, emailSvc)
	svc := ProvideTroubleshootingAssistantService(nil, nil, nil, settings, notificationSvc)
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	svc.nowFunc = func() time.Time { return now }

	input := TroubleshootingAdminNotifyInput{
		UserID:    42,
		Message:   "unexpected status 503 Service Unavailable, url: https://api.upit.top/51Token/v1/responses, request id: req-123",
		Diagnosis: "已确认当前账号池无可用账号，需要管理员处理。",
		Locale:    "zh-CN",
	}
	result, err := svc.NotifyAdmin(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "已通知管理员，请等待 5 分钟后重试。", result.Message)
	require.EqualValues(t, 1, smtpServer.messageCount())
	require.Contains(t, smtpServer.messageBodies()[0], "用户 ID")
	require.Contains(t, smtpServer.messageBodies()[0], "42")
	require.Contains(t, smtpServer.messageBodies()[0], "req-123")
	require.Contains(t, smtpServer.messageBodies()[0], "已确认当前账号池无可用账号")

	result, err = svc.NotifyAdmin(ctx, input)
	require.NoError(t, err)
	require.Equal(t, "管理员已收到，正在处理，请等待 5 分钟后重试。", result.Message)
	require.EqualValues(t, 1, smtpServer.messageCount())

	anotherUser := input
	anotherUser.UserID = 84
	result, err = svc.NotifyAdmin(ctx, anotherUser)
	require.NoError(t, err)
	require.Equal(t, "管理员已收到，正在处理，请等待 5 分钟后重试。", result.Message)
	require.EqualValues(t, 1, smtpServer.messageCount())

	now = now.Add(11 * time.Minute)
	_, err = svc.NotifyAdmin(ctx, input)
	require.NoError(t, err)
	require.EqualValues(t, 2, smtpServer.messageCount())
}

func TestTroubleshootingAssistantNotifyAdminRejectsNonTroubleshootingReportBeforeEmail(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyAccountQuotaNotifyEmails, MarshalNotifyEmails([]NotifyEmailEntry{
		{Email: "ops@example.com", Verified: true},
	})))
	svc := ProvideTroubleshootingAssistantService(nil, &troubleshootingLimiterStub{}, nil, settings, nil)

	_, err := svc.NotifyAdmin(ctx, TroubleshootingAdminNotifyInput{
		UserID:    42,
		Message:   "帮我写一首诗",
		Diagnosis: "需要管理员处理。",
	})

	require.Error(t, err)
	require.Equal(t, 400, infraerrors.Code(err))
	require.Equal(t, "TROUBLESHOOTING_ONLY", infraerrorsReason(err))
}

func TestTroubleshootingAssistantNotifyAdminAppliesRateLimit(t *testing.T) {
	ctx := context.Background()
	settings := newNotificationEmailMemorySettingRepo()
	require.NoError(t, settings.Set(ctx, SettingKeyAccountQuotaNotifyEmails, MarshalNotifyEmails([]NotifyEmailEntry{
		{Email: "ops@example.com", Verified: true},
	})))
	limiter := &troubleshootingLimiterStub{
		err: infraerrors.TooManyRequests("TROUBLESHOOTING_RATE_LIMITED", "故障排查次数已达到限制，请稍后再试"),
	}
	svc := ProvideTroubleshootingAssistantService(nil, limiter, nil, settings, nil)

	_, err := svc.NotifyAdmin(ctx, TroubleshootingAdminNotifyInput{
		UserID:    42,
		Message:   "POST https://api.example.com/v1/responses 返回 503 Service Unavailable",
		Diagnosis: "需要管理员处理。",
	})

	require.Error(t, err)
	require.Equal(t, 429, infraerrors.Code(err))
	require.Equal(t, 1, limiter.calls)
}
