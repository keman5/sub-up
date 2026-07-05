package service

import (
	"fmt"
	"strings"
)

const (
	clientErrorLocaleChinese = "zh"
)

var localizedClientErrorMessages = map[string]string{
	"API key group platform is not gemini":                               "API Key 所属分组不是 Gemini 平台",
	"Empty upstream response":                                            "上游返回为空",
	"Failed to parse request body":                                       "请求体解析失败",
	"Failed to read request body":                                        "读取请求体失败",
	"Image generation is not enabled for this group":                     "当前分组未启用图片生成",
	"Invalid API key":                                                    "API Key 无效",
	"Missing model in URL":                                               "URL 中缺少模型",
	"No available Gemini accounts":                                       "暂无可用 Gemini 账号",
	"OpenAI codex passthrough requires a non-empty instructions field":   "OpenAI Codex 透传请求必须包含非空 instructions 字段",
	"Request body is empty":                                              "请求体不能为空",
	"Too many pending requests, please retry later":                      "等待中的请求过多，请稍后重试",
	"Upstream access forbidden, please contact administrator":            "上游拒绝访问，请联系管理员",
	"Upstream gateway error":                                             "上游网关错误",
	"Upstream rate limit exceeded, please retry later":                   "上游限流，请稍后重试",
	"Upstream request failed":                                            "上游请求失败",
	"Upstream service overloaded, please retry later":                    "上游服务过载，请稍后重试",
	"Upstream service temporarily unavailable":                           "上游服务暂时不可用",
	"User context not found":                                             "用户上下文不存在",
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
}

// ClientErrorMessageForAcceptLanguage localizes locally generated gateway-compatible
// error messages while preserving upstream messages and the response shape.
func ClientErrorMessageForAcceptLanguage(acceptLanguage string, message string) string {
	message = strings.TrimSpace(message)
	if message == "" || clientErrorLocaleForAcceptLanguage(acceptLanguage) != clientErrorLocaleChinese {
		return message
	}
	if translated, ok := localizedClientErrorMessages[message]; ok && strings.TrimSpace(translated) != "" {
		return translated
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
	}
	return ""
}
