package response

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	errorLocaleChinese = "zh"
	errorLocaleEnglish = "en"
)

var localizedErrorMessages = map[string]map[string]string{
	errorLocaleChinese: {
		"AUTH_REQUIRED":                                "请先登录后再操作",
		"UNAUTHORIZED":                                 "请先登录后再操作",
		"INVALID_USER":                                 "用户不存在或登录状态无效",
		"BACKEND_MODE_ADMIN_ONLY":                      "后端模式已启用，仅允许管理员登录",
		"SERVICE_UNAVAILABLE":                          "服务暂时不可用，请稍后重试",
		"CONFIG_NOT_READY":                             "系统配置尚未就绪",
		"VALIDATION_ERROR":                             "请求参数校验失败",
		"INVALID_REQUEST":                              "请求参数无效",
		"OAUTH_DISABLED":                               "第三方登录未启用",
		"PENDING_AUTH_NOT_READY":                       "授权服务暂时不可用，请稍后重试",
		"PENDING_AUTH_SESSION_INVALID":                 "授权会话无效或已过期，请重新发起授权",
		"PENDING_AUTH_TARGET_USER_MISMATCH":            "授权会话必须由指定用户完成",
		"USER_EMAIL_CONFLICT":                          "该邮箱匹配到多个用户，请联系管理员处理",
		"AUTH_IDENTITY_OWNERSHIP_CONFLICT":             "该第三方身份已绑定到其他用户",
		"AUTH_IDENTITY_CHANNEL_OWNERSHIP_CONFLICT":     "该第三方身份通道已绑定到其他用户",
		"SUBSCRIPTION_NOT_FOUND":                       "订阅不存在",
		"SUBSCRIPTION_EXPIRED":                         "订阅已过期",
		"SUBSCRIPTION_SUSPENDED":                       "订阅已暂停",
		"SUBSCRIPTION_ALREADY_EXISTS":                  "该用户已拥有此套餐订阅",
		"SUBSCRIPTION_ASSIGN_CONFLICT":                 "该用户已有订阅，且本次分配内容与现有订阅不一致",
		"SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED": "该用户已拥有此套餐订阅，确认后将覆盖旧订阅",
		"SUBSCRIPTION_NOT_REVOKED":                     "订阅未撤销，无法恢复",
		"SUBSCRIPTION_RESTORE_CONFLICT":                "该用户已存在同套餐订阅，无法恢复",
		"GROUP_NOT_SUBSCRIPTION_TYPE":                  "目标分组不是订阅套餐类型",
		"DAILY_LIMIT_EXCEEDED":                         "今日用量已达到上限",
		"WEEKLY_LIMIT_EXCEEDED":                        "本周用量已达到上限",
		"MONTHLY_LIMIT_EXCEEDED":                       "本月用量已达到上限",
		"TOTAL_LIMIT_EXCEEDED":                         "总用量已达到上限",
		"SUBSCRIPTION_NIL_INPUT":                       "订阅参数不能为空",
		"ADJUST_WOULD_EXPIRE":                          "调整后订阅会立即过期，请输入更大的天数",
		"CANNOT_SHORTEN_EXPIRED":                       "已过期订阅不能缩短有效期",
		"TROUBLESHOOTING_EMPTY_REPORT":                 "请粘贴需要排查的报错信息",
		"TROUBLESHOOTING_ONLY":                         "这里只能排查请求失败原因，请粘贴报错信息、状态码、接口 URL 或 request id",
		"TROUBLESHOOTING_RATE_LIMITED":                 "故障排查请求过于频繁，请稍后再试",
		"TROUBLESHOOTING_DAILY_LIMITED":                "今日故障排查次数已达到限制，如仍无法解决请联系管理员",
		"TROUBLESHOOTING_NOTIFY_EMPTY":                 "缺少需要通知管理员的排查内容",
		"TROUBLESHOOTING_NOTIFY_UNAVAILABLE":           "管理员通知暂不可用",
		"TROUBLESHOOTING_NOTIFY_EMAILS_EMPTY":          "管理员通知邮箱未配置",
		"OPS_REPO_UNAVAILABLE":                         "运维日志服务暂时不可用",
		"OPS_FILTER_REQUIRED":                          "缺少查询条件",
		"OPS_TIME_RANGE_REQUIRED":                      "必须提供开始时间和结束时间",
		"OPS_TIME_RANGE_INVALID":                       "开始时间不能晚于结束时间",
		"OPS_ERROR_NOT_FOUND":                          "未找到对应错误日志",
		"OPS_ERROR_INVALID_ID":                         "错误日志 ID 无效",
		"OPS_ERROR_LOAD_FAILED":                        "读取错误日志失败",
		"TURNSTILE_VERIFICATION_FAILED":                "人机验证失败，请重试",
		"TURNSTILE_NOT_CONFIGURED":                     "人机验证尚未配置",
		"TURNSTILE_INVALID_SECRET_KEY":                 "人机验证密钥无效",
		"TOTP_NOT_ENABLED":                             "两步验证功能未启用",
		"TOTP_ALREADY_ENABLED":                         "该账号已启用两步验证",
		"TOTP_NOT_SETUP":                               "该账号尚未设置两步验证",
		"TOTP_INVALID_CODE":                            "两步验证码无效",
		"TOTP_SETUP_EXPIRED":                           "两步验证设置会话已过期",
		"TOTP_TOO_MANY_ATTEMPTS":                       "验证尝试次数过多，请稍后再试",
		"VERIFY_CODE_REQUIRED":                         "请输入邮箱验证码",
		"PASSWORD_REQUIRED":                            "请输入密码",
		"EMAIL_VERIFY_NOT_ENABLED":                     "邮箱验证功能未启用",
		"REDEEM_CODE_EXPIRY_CONFLICT":                  "不能同时设置固定过期时间和有效天数",
		"REDEEM_CODE_EXPIRES_IN_DAYS_INVALID":          "有效天数必须大于 0",
		"REDEEM_CODE_EXPIRES_AT_INVALID":               "过期时间必须晚于当前时间",
		"REDEEM_CODE_CONFLICT":                         "兑换码冲突或已被其他用户使用",
	},
	errorLocaleEnglish: {
		"AUTH_REQUIRED":                                "Please sign in before continuing.",
		"UNAUTHORIZED":                                 "Please sign in before continuing.",
		"INVALID_USER":                                 "The user does not exist or the session is invalid.",
		"BACKEND_MODE_ADMIN_ONLY":                      "Backend mode is active. Only admin login is allowed.",
		"SERVICE_UNAVAILABLE":                          "The service is temporarily unavailable. Please try again later.",
		"CONFIG_NOT_READY":                             "System configuration is not ready.",
		"VALIDATION_ERROR":                             "Request validation failed.",
		"INVALID_REQUEST":                              "Invalid request.",
		"OAUTH_DISABLED":                               "OAuth login is disabled.",
		"PENDING_AUTH_NOT_READY":                       "The authorization service is temporarily unavailable. Please try again later.",
		"PENDING_AUTH_SESSION_INVALID":                 "The authorization session is invalid or expired. Please start again.",
		"PENDING_AUTH_TARGET_USER_MISMATCH":            "The authorization session must be completed by the targeted user.",
		"USER_EMAIL_CONFLICT":                          "This email matches multiple users. Please contact an administrator.",
		"AUTH_IDENTITY_OWNERSHIP_CONFLICT":             "This third-party identity is already bound to another user.",
		"AUTH_IDENTITY_CHANNEL_OWNERSHIP_CONFLICT":     "This third-party identity channel is already bound to another user.",
		"SUBSCRIPTION_NOT_FOUND":                       "Subscription not found.",
		"SUBSCRIPTION_EXPIRED":                         "The subscription has expired.",
		"SUBSCRIPTION_SUSPENDED":                       "The subscription is suspended.",
		"SUBSCRIPTION_ALREADY_EXISTS":                  "This user already has a subscription for this plan.",
		"SUBSCRIPTION_ASSIGN_CONFLICT":                 "This user already has a subscription, and the new assignment conflicts with it.",
		"SUBSCRIPTION_DUPLICATE_CONFIRMATION_REQUIRED": "This user already has a subscription for this plan. Confirm to overwrite the existing subscription.",
		"SUBSCRIPTION_NOT_REVOKED":                     "The subscription is not revoked and cannot be restored.",
		"SUBSCRIPTION_RESTORE_CONFLICT":                "This user already has an active subscription for the same plan.",
		"GROUP_NOT_SUBSCRIPTION_TYPE":                  "The target group is not a subscription plan.",
		"DAILY_LIMIT_EXCEEDED":                         "Daily usage limit exceeded.",
		"WEEKLY_LIMIT_EXCEEDED":                        "Weekly usage limit exceeded.",
		"MONTHLY_LIMIT_EXCEEDED":                       "Monthly usage limit exceeded.",
		"TOTAL_LIMIT_EXCEEDED":                         "Total usage limit exceeded.",
		"SUBSCRIPTION_NIL_INPUT":                       "Subscription input cannot be empty.",
		"ADJUST_WOULD_EXPIRE":                          "The adjustment would make the subscription expire immediately.",
		"CANNOT_SHORTEN_EXPIRED":                       "Expired subscriptions cannot be shortened.",
		"TROUBLESHOOTING_EMPTY_REPORT":                 "Paste the error information that needs diagnosis.",
		"TROUBLESHOOTING_ONLY":                         "This assistant can only diagnose request failures. Paste an error message, status code, API URL, or request id.",
		"TROUBLESHOOTING_RATE_LIMITED":                 "Troubleshooting requests are too frequent. Please try again later.",
		"TROUBLESHOOTING_DAILY_LIMITED":                "Today's troubleshooting limit has been reached. Contact an administrator if the issue is still unresolved.",
		"TROUBLESHOOTING_NOTIFY_EMPTY":                 "Troubleshooting details are required before notifying an administrator.",
		"TROUBLESHOOTING_NOTIFY_UNAVAILABLE":           "Administrator notification is temporarily unavailable.",
		"TROUBLESHOOTING_NOTIFY_EMAILS_EMPTY":          "Administrator notification emails are not configured.",
		"OPS_REPO_UNAVAILABLE":                         "The operations log service is temporarily unavailable.",
		"OPS_FILTER_REQUIRED":                          "A filter is required.",
		"OPS_TIME_RANGE_REQUIRED":                      "Start time and end time are required.",
		"OPS_TIME_RANGE_INVALID":                       "Start time must not be later than end time.",
		"OPS_ERROR_NOT_FOUND":                          "Error log not found.",
		"OPS_ERROR_INVALID_ID":                         "Invalid error log ID.",
		"OPS_ERROR_LOAD_FAILED":                        "Failed to load the error log.",
		"TURNSTILE_VERIFICATION_FAILED":                "Human verification failed. Please try again.",
		"TURNSTILE_NOT_CONFIGURED":                     "Human verification is not configured.",
		"TURNSTILE_INVALID_SECRET_KEY":                 "Invalid human verification secret key.",
		"TOTP_NOT_ENABLED":                             "Two-factor authentication is not enabled.",
		"TOTP_ALREADY_ENABLED":                         "Two-factor authentication is already enabled for this account.",
		"TOTP_NOT_SETUP":                               "Two-factor authentication is not set up for this account.",
		"TOTP_INVALID_CODE":                            "Invalid two-factor authentication code.",
		"TOTP_SETUP_EXPIRED":                           "The two-factor setup session has expired.",
		"TOTP_TOO_MANY_ATTEMPTS":                       "Too many verification attempts. Please try again later.",
		"VERIFY_CODE_REQUIRED":                         "Email verification code is required.",
		"PASSWORD_REQUIRED":                            "Password is required.",
		"EMAIL_VERIFY_NOT_ENABLED":                     "Email verification is not enabled.",
		"REDEEM_CODE_EXPIRY_CONFLICT":                  "expires_at and expires_in_days cannot both be set.",
		"REDEEM_CODE_EXPIRES_IN_DAYS_INVALID":          "expires_in_days must be greater than zero.",
		"REDEEM_CODE_EXPIRES_AT_INVALID":               "expires_at must be in the future.",
		"REDEEM_CODE_CONFLICT":                         "Redeem code conflict or already used by another user.",
	},
}

var localizedDirectErrorMessages = map[string]map[string]string{
	errorLocaleChinese: {
		"API key not found":                                                  "API Key 不存在",
		"API key service is not configured":                                  "API Key 服务未配置",
		"API key service not available":                                      "API Key 服务暂时不可用",
		"Auth service not configured":                                        "认证服务未配置",
		"Error requests view is disabled":                                    "错误请求查看功能未启用",
		"Failed to parse request body":                                       "请求体解析失败",
		"Failed to read request body":                                        "读取请求体失败",
		"Invalid API key ID":                                                 "API Key ID 无效",
		"Invalid account ID":                                                 "账号 ID 无效",
		"Invalid announcement ID":                                            "公告 ID 无效",
		"Invalid api_key_id":                                                 "API Key ID 无效",
		"Invalid billing_mode":                                               "计费模式无效",
		"Invalid billing_type":                                               "计费类型无效",
		"Invalid end_date format, use YYYY-MM-DD":                            "结束日期格式无效，请使用 YYYY-MM-DD",
		"Invalid group ID":                                                   "分组 ID 无效",
		"Invalid group_id":                                                   "分组 ID 无效",
		"Invalid id":                                                         "ID 无效",
		"Invalid key ID":                                                     "API Key ID 无效",
		"Invalid model_source, user usage only supports requested":           "模型来源无效，用户用量只支持 requested",
		"Invalid request":                                                    "请求参数无效",
		"Invalid start_date format, use YYYY-MM-DD":                          "开始日期格式无效，请使用 YYYY-MM-DD",
		"Invalid status_code":                                                "状态码无效",
		"Invalid stream value, use true or false":                            "stream 参数无效，请使用 true 或 false",
		"Invalid subscription ID":                                            "订阅 ID 无效",
		"Invalid usage ID":                                                   "用量记录 ID 无效",
		"Invalid user ID":                                                    "用户 ID 无效",
		"Invalid user_id":                                                    "用户 ID 无效",
		"Image generation is not enabled for this group":                     "当前分组未启用图片生成",
		"Not authorized to access this API key's usage":                      "无权访问该 API Key 的用量",
		"Not authorized to access this API key's usage records":              "无权访问该 API Key 的用量记录",
		"Not authorized to access this record":                               "无权访问该记录",
		"Ops service not available":                                          "运维服务暂时不可用",
		"Pending oauth session provider mismatch":                            "待处理授权会话的提供方不匹配",
		"Rate limit service unavailable":                                     "限流服务暂时不可用",
		"Request body is empty":                                              "请求体不能为空",
		"Subscription not found":                                             "订阅不存在",
		"Too many API key IDs (maximum 100 allowed)":                         "API Key ID 数量过多，最多允许 100 个",
		"Troubleshooting service not available":                              "故障排查服务暂时不可用",
		"User not authenticated":                                             "请先登录",
		"User not found in context":                                          "请先登录",
		"account_ids is required":                                            "请选择账号",
		"account_ids or filters is required":                                 "请选择账号或筛选条件",
		"backup ID is required":                                              "备份 ID 不能为空",
		"failed to parse request body":                                       "请求体解析失败",
		"image file is required":                                             "必须上传图片文件",
		"images endpoint requires an image model":                            "图片接口需要使用图片模型",
		"images[].file_id is not supported (use images[].image_url instead)": "images[].file_id 不支持，请改用 images[].image_url",
		"images[].image_url is required":                                     "images[].image_url 不能为空",
		"incorrect admin password":                                           "管理员密码不正确",
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
		"notification email service is not configured":                       "通知邮件服务未配置",
		"password is required for restore operation":                         "恢复操作需要输入密码",
		"refresh_token is required":                                          "refresh_token 不能为空",
		"request body is empty":                                              "请求体不能为空",
		"token is required":                                                  "token 不能为空",
		"unauthorized":                                                       "请先登录",
	},
}

func localizeErrorMessage(c *gin.Context, reason string, fallback string) string {
	reason = strings.TrimSpace(reason)
	locale := requestErrorLocale(c)
	if locale == "" {
		return fallback
	}
	if reason != "" {
		if byReason, ok := localizedErrorMessages[locale]; ok {
			if message, ok := byReason[reason]; ok && strings.TrimSpace(message) != "" {
				return message
			}
		}
	}
	return localizeDirectErrorMessage(locale, fallback)
}

func localizeDirectErrorMessage(locale string, fallback string) string {
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return fallback
	}
	if byMessage, ok := localizedDirectErrorMessages[locale]; ok {
		if message, ok := byMessage[fallback]; ok && strings.TrimSpace(message) != "" {
			return message
		}
	}
	if locale != errorLocaleChinese {
		return fallback
	}

	if strings.HasPrefix(fallback, "Invalid request: ") {
		return "请求参数无效：" + strings.TrimPrefix(fallback, "Invalid request: ")
	}
	if strings.HasPrefix(fallback, "images endpoint requires an image model, got ") {
		return "图片接口需要使用图片模型，当前模型为 " + strings.TrimPrefix(fallback, "images endpoint requires an image model, got ")
	}
	if strings.HasPrefix(fallback, "Invalid ") && strings.HasSuffix(fallback, " ID") {
		field := strings.TrimSuffix(strings.TrimPrefix(fallback, "Invalid "), " ID")
		if label := localizedIDFieldName(field); label != "" {
			return label + " ID 无效"
		}
		return field + " ID 无效"
	}
	if strings.HasPrefix(fallback, "Invalid ") && strings.Contains(fallback, " value, use true or false") {
		field := strings.TrimSuffix(strings.TrimPrefix(fallback, "Invalid "), " value, use true or false")
		return field + " 参数无效，请使用 true 或 false"
	}
	if strings.HasPrefix(fallback, "Failed to ") {
		return "操作失败：" + strings.TrimPrefix(fallback, "Failed to ")
	}
	if strings.HasSuffix(strings.ToLower(fallback), " not found") {
		subject := strings.TrimSpace(fallback[:len(fallback)-len(" not found")])
		if label := localizedIDFieldName(subject); label != "" {
			return label + "不存在"
		}
	}
	return fallback
}

func localizedIDFieldName(field string) string {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "account":
		return "账号"
	case "api key", "key":
		return "API Key"
	case "announcement":
		return "公告"
	case "backup":
		return "备份"
	case "channel":
		return "渠道"
	case "error log":
		return "错误日志"
	case "group":
		return "分组"
	case "profile":
		return "配置"
	case "promo code":
		return "优惠码"
	case "rule":
		return "规则"
	case "subscription":
		return "订阅"
	case "usage":
		return "用量记录"
	case "user":
		return "用户"
	}
	return ""
}

func requestErrorLocale(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	raw := strings.ToLower(strings.TrimSpace(c.GetHeader("Accept-Language")))
	for _, part := range strings.Split(raw, ",") {
		tag := strings.TrimSpace(part)
		if semi := strings.IndexByte(tag, ';'); semi >= 0 {
			tag = strings.TrimSpace(tag[:semi])
		}
		switch {
		case tag == "zh" || strings.HasPrefix(tag, "zh-") || strings.HasPrefix(tag, "zh_"):
			return errorLocaleChinese
		case tag == "en" || strings.HasPrefix(tag, "en-") || strings.HasPrefix(tag, "en_"):
			return errorLocaleEnglish
		}
	}
	return ""
}
