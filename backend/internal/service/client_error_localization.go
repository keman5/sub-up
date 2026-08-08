package service

import (
	"fmt"
	"strings"
)

const (
	clientErrorLocaleChinese = "zh"
	clientErrorLocaleEnglish = "en"
)

var localizedClientErrorMessages = map[string]string{
	"API key 额度已用完":                                                      "API Key 额度已用完",
	"API key 已过期":                                                        "API Key 已过期",
	"API key group platform is not gemini":                               "API Key 所属分组不是 Gemini 平台",
	"All available accounts exhausted":                                   "所有可用上游账号均请求失败，请稍后重试",
	"Billing service temporarily unavailable. Please retry later.":       "计费服务暂时不可用，请稍后重试",
	"Daily usage quota exhausted for this platform.":                     "当前平台今日额度已用完，请在额度重置后重试",
	"Empty upstream response":                                            "上游返回为空",
	"Failed to maintain subscription usage windows":                      "套餐额度状态刷新失败，请稍后重试",
	"Failed to parse request body":                                       "请求体解析失败",
	"Failed to read request body":                                        "读取请求体失败",
	"Image generation is not enabled for this group":                     "当前分组未启用图片生成",
	"Invalid API key":                                                    "API Key 无效",
	"Insufficient account balance":                                       "账户余额不足，请充值后重试",
	"insufficient balance":                                               "账户余额不足，请充值后重试",
	"Missing model in URL":                                               "URL 中缺少模型",
	"No active subscription found for this group":                        "当前分组没有有效订阅，请联系管理员续期",
	"No available accounts":                                              "系统当前没有可用上游账号，请联系管理员",
	"No available Gemini accounts":                                       "暂无可用 Gemini 账号",
	"OpenAI codex passthrough requires a non-empty instructions field":   "OpenAI Codex 透传请求必须包含非空 instructions 字段",
	"Request body is empty":                                              "请求体不能为空",
	"Too many pending requests, please retry later":                      "等待中的请求过多，请稍后重试",
	"Upstream access forbidden, please contact administrator":            "上游拒绝访问，请联系管理员",
	"Upstream gateway error":                                             "上游网关错误",
	"Upstream rate limit exceeded, please retry later":                   "上游限流，请稍后重试",
	"Upstream request failed":                                            "上游请求失败",
	"Upstream response timed out. Please retry later.":                   "上游响应超时，请稍后重试",
	"Upstream service overloaded, please retry later":                    "上游服务过载，请稍后重试",
	"Upstream service temporarily unavailable":                           "上游服务暂时不可用",
	"User context not found":                                             "用户上下文不存在",
	"Weekly usage quota exhausted for this platform.":                    "当前平台本周额度已用完，请在额度重置后重试",
	"Monthly usage quota exhausted for this platform.":                   "当前平台本月额度已用完，请在额度重置后重试",
	"daily usage limit exceeded":                                         "当前套餐今日额度已用完，请在额度重置后重试",
	"failed to parse request body":                                       "请求体解析失败",
	"image file is required":                                             "必须上传图片文件",
	"images endpoint requires an image model":                            "图片接口需要使用图片模型",
	"images[].file_id is not supported (use images[].image_url instead)": "images[].file_id 不支持，请改用 images[].image_url",
	"images[].image_url is required":                                     "images[].image_url 不能为空",
	"invalid images field type":                                          "images 字段类型无效",
	"invalid n field type":                                               "n 字段类型无效",
	"invalid output_compression field type":                              "output_compression 字段类型无效",
	"invalid output_compression field value":                             "output_compression 字段值无效",
	"invalid partial_images field type":                                  "partial_images 字段类型无效",
	"invalid partial_images field value":                                 "partial_images 字段值无效",
	"invalid stream field type":                                          "stream 字段类型无效",
	"invalid stream field value":                                         "stream 字段值无效",
	"mask.file_id is not supported (use mask.image_url instead)":         "mask.file_id 不支持，请改用 mask.image_url",
	"n must be a positive integer":                                       "n 必须是正整数",
	"n must be greater than 0":                                           "n 必须大于 0",
	"request body is empty":                                              "请求体不能为空",
	"subscription has expired":                                           "当前订阅已过期，请联系管理员续期",
	"subscription is suspended":                                          "当前订阅已暂停，请联系管理员",
	"total usage limit exceeded":                                         "当前套餐总额度已用完，请联系管理员续期或更换套餐",
	"weekly usage limit exceeded":                                        "当前套餐本周额度已用完，请在额度重置后重试",
	"monthly usage limit exceeded":                                       "当前套餐本月额度已用完，请在额度重置后重试",
}

var canonicalEnglishClientErrorMessages = map[string]string{
	"API key 额度已用完":                                    "The API key quota has been exhausted.",
	"API key 已过期":                                      "The API key has expired.",
	"All available accounts exhausted":                 "All available upstream accounts failed. Please retry later.",
	"Daily usage quota exhausted for this platform.":   "The daily quota for this platform has been exhausted. Retry after the quota resets.",
	"Failed to maintain subscription usage windows":    "The subscription quota status could not be refreshed. Please retry later.",
	"Insufficient account balance":                     "The account balance is insufficient. Add funds and retry.",
	"insufficient balance":                             "The account balance is insufficient. Add funds and retry.",
	"No active subscription found for this group":      "No active subscription is available for this group. Contact the administrator to renew it.",
	"No available accounts":                            "No upstream accounts are currently available. Contact the administrator.",
	"Weekly usage quota exhausted for this platform.":  "The weekly quota for this platform has been exhausted. Retry after the quota resets.",
	"Monthly usage quota exhausted for this platform.": "The monthly quota for this platform has been exhausted. Retry after the quota resets.",
	"daily usage limit exceeded":                       "The current subscription's daily quota has been exhausted. Retry after the quota resets.",
	"monthly usage limit exceeded":                     "The current subscription's monthly quota has been exhausted. Retry after the quota resets.",
	"subscription has expired":                         "The current subscription has expired. Contact the administrator to renew it.",
	"subscription is suspended":                        "The current subscription is suspended. Contact the administrator.",
	"total usage limit exceeded":                       "The current subscription's total quota has been exhausted. Contact the administrator to renew or change the subscription.",
	"weekly usage limit exceeded":                      "The current subscription's weekly quota has been exhausted. Retry after the quota resets.",
}

// ClientErrorMessageForAcceptLanguage localizes locally generated gateway-compatible
// error messages while preserving upstream messages and the response shape.
func ClientErrorMessageForAcceptLanguage(acceptLanguage string, message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return message
	}
	// Upstream errors are usually supplied in English and are valuable for
	// debugging. Keep that diagnostic intact, while adding a Chinese hint for
	// well-known, stable messages regardless of the client's language header.
	if chineseHint := commonUpstreamErrorChineseHint(message); chineseHint != "" {
		return message + " (中文：" + chineseHint + ")"
	}
	translated, known := localizedClientErrorMessages[message]
	locale := clientErrorLocaleForAcceptLanguage(acceptLanguage)
	if locale == clientErrorLocaleChinese && known && strings.TrimSpace(translated) != "" {
		return translated
	}
	english := message
	if canonical, ok := canonicalEnglishClientErrorMessages[message]; ok && strings.TrimSpace(canonical) != "" {
		english = canonical
	}
	if locale == clientErrorLocaleEnglish {
		return english
	}
	if locale == "" && known && strings.TrimSpace(translated) != "" && english != translated {
		return english + " (" + translated + ")"
	}
	if strings.HasPrefix(message, "images endpoint requires an image model, got ") {
		return fmt.Sprintf("图片接口需要使用图片模型，当前模型为 %s", strings.TrimPrefix(message, "images endpoint requires an image model, got "))
	}
	if strings.HasPrefix(message, "No available Gemini accounts: ") {
		return "暂无可用 Gemini 账号：" + strings.TrimPrefix(message, "No available Gemini accounts: ")
	}
	if strings.HasPrefix(message, "invalid multipart content-type: ") {
		return "multipart Content-Type 无效：" + strings.TrimPrefix(message, "invalid multipart content-type: ")
	}
	if strings.HasPrefix(message, "read multipart body: ") {
		return "读取 multipart 请求体失败：" + strings.TrimPrefix(message, "read multipart body: ")
	}
	if strings.HasPrefix(message, "read multipart field ") {
		return "读取 multipart 字段失败：" + strings.TrimPrefix(message, "read multipart field ")
	}
	return message
}

// commonUpstreamErrorChineseHint returns an explanation only for stable error
// patterns emitted by major upstream providers. Unknown provider messages must
// remain untouched so that users can still diagnose provider-specific issues.
func commonUpstreamErrorChineseHint(message string) string {
	lower := strings.ToLower(strings.TrimSpace(message))
	switch {
	case strings.Contains(lower, "selected model is at capacity"):
		return "所选模型当前容量已满，请稍后重试或更换模型"
	case strings.Contains(lower, "server_is_overloaded"),
		strings.Contains(lower, "server is overloaded"),
		strings.Contains(lower, "overloaded_error"):
		return "上游服务当前过载，请稍后重试"
	case strings.Contains(lower, "slow_down"):
		return "请求速度过快，请稍后重试"
	case strings.Contains(lower, "rate_limit_exceeded"),
		strings.Contains(lower, "rate limit exceeded"),
		strings.Contains(lower, "too many requests"):
		return "请求频率已达到上游限制，请稍后重试"
	case strings.Contains(lower, "insufficient_quota"),
		strings.Contains(lower, "insufficient balance"),
		strings.Contains(lower, "insufficient credits"):
		return "上游账户额度或余额不足"
	case strings.Contains(lower, "invalid_api_key"),
		strings.Contains(lower, "invalid api key"),
		strings.Contains(lower, "incorrect api key"):
		return "上游 API Key 无效或已失效"
	case strings.Contains(lower, "model_not_found"),
		strings.Contains(lower, "model does not exist"),
		strings.Contains(lower, "model not found"):
		return "上游未找到该模型，或当前账号无权使用该模型"
	case strings.Contains(lower, "context_length_exceeded"),
		strings.Contains(lower, "context_too_large"),
		strings.Contains(lower, "maximum context length"),
		strings.Contains(lower, "context window") && strings.Contains(lower, "exceed"):
		return "输入内容超过模型上下文长度限制，请缩短上下文后重试"
	case strings.Contains(lower, "request timed out"),
		strings.Contains(lower, "request timeout"),
		strings.Contains(lower, "upstream timeout"):
		return "上游请求超时，请稍后重试"
	case strings.Contains(lower, "internal server error"),
		strings.Contains(lower, "internal_error"):
		return "上游服务内部错误，请稍后重试"
	case strings.Contains(lower, "instructions are required"),
		strings.Contains(lower, "missing required parameter") && strings.Contains(lower, "instructions"):
		return "请求缺少 instructions 参数"
	case strings.Contains(lower, "missing scopes"),
		strings.Contains(lower, "insufficient permissions"):
		return "上游账号缺少调用该接口所需的权限"
	case strings.Contains(lower, "an error occurred while processing your request"):
		return "上游处理请求时发生临时错误，请稍后重试"
	default:
		return ""
	}
}

func clientErrorLocaleForAcceptLanguage(acceptLanguage string) string {
	raw := strings.ToLower(strings.TrimSpace(acceptLanguage))
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if semi := strings.IndexByte(tag, ';'); semi >= 0 {
			tag = strings.TrimSpace(tag[:semi])
		}
		if tag == "zh" || strings.HasPrefix(tag, "zh-") || strings.HasPrefix(tag, "zh_") || tag == "cn" {
			return clientErrorLocaleChinese
		}
		if tag == "en" || strings.HasPrefix(tag, "en-") || strings.HasPrefix(tag, "en_") {
			return clientErrorLocaleEnglish
		}
	}
	return ""
}
