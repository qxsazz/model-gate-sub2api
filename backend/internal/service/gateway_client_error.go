package service

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const preserveGatewayClientMessageKey = "preserve_gateway_client_error_message"
const gatewayClientReasonKey = "gateway_client_error_reason"

// SetGatewayClientErrorReason carries an existing local error reason to the
// presentation boundary when a protocol envelope has no place for that reason.
func SetGatewayClientErrorReason(c *gin.Context, reason string) {
	if c != nil {
		c.Set(gatewayClientReasonKey, strings.ToLower(reason))
	}
}

// PreserveGatewayClientErrorMessage protects an explicitly configured admin
// message. It affects presentation only, never recovery or monitoring.
func PreserveGatewayClientErrorMessage(c *gin.Context) {
	if c != nil {
		c.Set(preserveGatewayClientMessageKey, true)
	}
}

type gatewayClientHint struct{ label, text string }

var gatewayClientHints = map[string]gatewayClientHint{
	"auth":           {"认证失败", "当前 API Key 无效或已失效。请检查客户端填写的 API Key，必要时重新生成并更新。"},
	"upstream_auth":  {"上游认证异常", "上游服务认证或权限异常。请稍后重试；持续失败请联系管理员。"},
	"permission":     {"访问受限", "当前账号或分组无权使用此模型或接口。请检查访问权限，或联系管理员。"},
	"balance":        {"余额不足", "当前可用余额不足。请充值或检查账户余额后重试。"},
	"plan":           {"套餐额度不足", "当前套餐可用额度不足。请检查套餐额度及重置时间，或联系管理员。"},
	"upstream_quota": {"上游额度不足", "上游服务可用额度不足。请稍后重试；持续失败请联系管理员。"},
	"rate":           {"请求过于频繁", "当前请求频率或并发已达限制。请减少同时进行的请求，等待片刻后重试。"},
	"upstream_rate":  {"上游请求受限", "上游服务暂时限流。请等待片刻后在当前会话重试；持续失败请联系管理员。"},
	"expired":        {"账户或套餐已到期", "当前账户或套餐已到期。请检查有效期并续订，或联系管理员。"},
	"model":          {"模型不可用", "当前模型不可用或不受此入口支持。请检查模型名称、调用接口及分组支持范围。"},
	"route":          {"接口不存在", "当前请求接口不存在。请检查客户端的服务地址和接口路径。"},
	"feature":        {"接口不支持", "当前分组不支持此接口或功能。请检查调用接口及分组支持范围，或联系管理员。"},
	"capacity":       {"暂无可用服务", "当前没有符合条件的可用服务。请稍后重试；持续失败请联系管理员。"},
	"busy":           {"服务繁忙", "上游服务当前繁忙。请稍后在当前会话重试；持续失败可尝试新建会话并补充必要上下文。"},
	"server":         {"服务暂时不可用", "服务暂时不可用。请稍后在当前会话重试，可重新发送上一条消息或输入“请重试上一条请求”；若持续失败，可尝试新建会话并补充必要上下文。"},
	"timeout":        {"请求超时", "本次请求等待超时。请稍后在当前会话重试；持续失败可适当缩短本次请求内容，或联系管理员。"},
	"context":        {"会话内容过长", "当前会话超过模型上下文限制。请压缩历史内容，或新建会话并提供必要背景。"},
	"size":           {"请求内容过大", "本次请求超过大小限制。请减少附件或单次输入内容后重试。"},
	"policy":         {"请求受策略限制", "当前请求受到服务内容或安全策略限制。请调整请求内容；如认为判断有误，请联系管理员。"},
	"parameter":      {"请求参数错误", "请求参数不符合接口要求。请检查参数格式及模型支持的功能。"},
	"denied":         {"请求被拒绝", "当前请求被拒绝。请检查账户状态和访问权限；如仍无法解决，请联系管理员。"},
	"unknown":        {"请求失败", "本次请求未能完成。请根据错误码检查请求配置，或联系管理员。"},
}

// Only known machine identifiers may be echoed. Provider messages and unknown
// codes can contain credentials, URLs or user input, and are never interpolated.
var gatewayClientCodeCategories = map[string]string{
	"invalid_api_key": "auth", "api_key_required": "auth", "api_key_disabled": "auth", "user_not_found": "auth", "user_inactive": "denied",
	"authentication_error": "auth", "invalid_token": "auth", "unauthenticated": "auth",
	"access_denied": "permission", "group_not_allowed": "permission", "group_required": "permission",
	"insufficient_balance": "balance", "insufficient_quota": "plan", "api_key_quota_exhausted": "plan",
	"subscription_quota_exceeded": "plan", "subscription_daily_limit_exceeded": "plan", "subscription_weekly_limit_exceeded": "plan", "subscription_monthly_limit_exceeded": "plan",
	"subscription_not_found": "plan", "subscription_expired": "expired", "api_key_expired": "expired", "account_expired": "expired",
	"daily_limit_exceeded": "plan", "weekly_limit_exceeded": "plan", "monthly_limit_exceeded": "plan",
	"api_key_rate_5h_exceeded": "plan", "api_key_rate_1d_exceeded": "plan", "api_key_rate_7d_exceeded": "plan",
	"group_rpm_exceeded": "rate", "user_rpm_exceeded": "rate",
	"user_platform_daily_quota_exhausted": "plan", "user_platform_weekly_quota_exhausted": "plan", "user_platform_monthly_quota_exhausted": "plan",
	"gateway_queue_full": "rate", "gateway_concurrency_limit": "rate", "rate_limit_exceeded": "rate", "rate_limit_error": "rate", "rate_limit_reached": "rate", "resource_exhausted": "rate",
	"model_not_found": "model", "model_not_supported": "model", "compact_not_supported": "model",
	"context_length_exceeded": "context", "context_window_exceeded": "context", "context_too_large": "context",
	"request_too_large": "size", "request_body_too_large": "size",
	"content_policy_violation": "policy", "cyber_policy": "policy", "content_filter": "policy", "content_filter_error": "policy",
	"prompt_guard_blocked": "policy",
	"server_is_overloaded": "busy", "slow_down": "busy", "overloaded_error": "busy", "model_capacity_exhausted": "busy",
	"timeout": "timeout", "request_timeout": "timeout", "gateway_timeout": "timeout", "deadline_exceeded": "timeout",
	"server_error": "server", "internal_error": "server", "unavailable": "server",
}

// RewriteGatewayClientError is a last-mile, copy-on-write transformation. Call
// only on gateway responses AFTER existing sanitization, compatibility-code
// conversion and Ops capture. It must never feed inference error classification.
// status is the actual HTTP status (0 for WS); event is the SSE/WS event type.
func RewriteGatewayClientError(c *gin.Context, payload []byte, status int, event string) []byte {
	if c != nil && c.GetBool(preserveGatewayClientMessageKey) {
		return payload
	}
	if !gjson.ValidBytes(payload) {
		return payload
	}
	root := gjson.ParseBytes(payload)
	if event == "" {
		event = root.Get("type").String()
	}
	stream := status < 400
	if stream && event != "" && event != "error" && event != "response.failed" {
		return payload
	}
	if stream && event != "error" && event != "response.failed" && !root.Get("error").IsObject() {
		return payload
	}
	path := ""
	for _, p := range []string{"response.error", "error"} {
		if e := root.Get(p); e.IsObject() && e.Get("message").Type == gjson.String {
			path = p
			break
		}
	}
	if path == "" {
		if root.Get("message").Type != gjson.String || (stream && event != "error") {
			return payload
		}
		if !stream && !root.Get("code").Exists() && event != "error" {
			return payload
		}
	}
	field := func(key string) gjson.Result {
		if path != "" {
			return root.Get(path + "." + key)
		}
		return root.Get(key)
	}
	code := strings.ToLower(strings.TrimSpace(field("code").String()))
	typ := strings.ToLower(strings.TrimSpace(field("type").String()))
	// Google errors expose a numeric code; preserve their existing internal
	// type/reason for presentation without adding protocol fields.
	if gatewayClientCodeCategories[code] == "" {
		if reason := strings.ToLower(field("details.0.reason").String()); gatewayClientCodeCategories[reason] != "" {
			code = reason
		}
		if c != nil {
			localType, localCode := OpsLocalErrorType(c)
			if gatewayClientCodeCategories[strings.ToLower(localCode)] != "" {
				code = strings.ToLower(localCode)
			}
			if typ == "" {
				typ = strings.ToLower(localType)
			}
		}
	}
	if code == "session_blocked_by_cyber_policy" {
		return payload
	}
	messagePath := "message"
	if path != "" {
		messagePath = path + ".message"
	}
	original := root.Get(messagePath).String()
	semantic := 0
	if stream {
		for _, key := range []string{"status_code", "status", "code"} {
			n, err := strconv.Atoi(field(key).String())
			if err == nil && n >= 400 && n <= 599 {
				semantic = n
				break
			}
		}
		if semantic == 0 && c != nil {
			if se, ok := GetOpsStreamError(c); ok && se.IntendedStatus >= 400 && se.IntendedStatus <= 599 {
				semantic = se.IntendedStatus
			}
		}
	}
	upstream := stream
	upstreamStatus := 0
	if c != nil {
		upstreamStatus = c.GetInt(OpsUpstreamStatusCodeKey)
		upstream = upstream || upstreamStatus > 0
	}
	if typ == "upstream_error" {
		upstream = true
	}
	category := gatewayClientCodeCategories[code]
	safeCode := code
	if category == "" {
		safeCode = ""
		category = gatewayClientCodeCategories[typ]
		if category != "" {
			safeCode = typ
		}
	}
	if category == "" {
		category = gatewayClientCodeCategories[strings.ToLower(field("status").String())]
	}
	// Reuse existing provider classifiers, scoped to the error object so model
	// output and echoed request fields cannot change the presentation category.
	errorBody := payload
	if path != "" {
		errorBody = []byte(`{"error":` + root.Get(path).Raw + `}`)
	}
	if isOpenAIContextWindowError(original, errorBody) {
		category = "context"
	}
	if isOpenAIRequestScopedCapacityShed(original, errorBody) {
		category = "busy"
		if stream && semantic == 0 {
			semantic = 503
		}
	}
	// These are exact existing gateway literals, not an arbitrary upstream
	// keyword inference system. They compensate for legacy writers without codes.
	switch original {
	case "No available accounts", "No available Gemini accounts", "No available OpenAI accounts", profitControlNoAccountsMessage:
		if category == "" || category == "server" {
			category = "capacity"
		}
	case "Upstream authentication failed, please contact administrator", "Upstream access forbidden, please contact administrator":
		category = "upstream_auth"
	}
	if c != nil {
		if reason := OpsClientBusinessLimitedReason(c); reason == OpsClientBusinessLimitedReasonLocalModelConfiguration {
			category = "model"
		} else if reason == OpsClientBusinessLimitedReasonLocalFeatureGate {
			category = "feature"
		} else if reason == OpsClientBusinessLimitedReasonAPIKeyGroupUnassigned || reason == OpsClientBusinessLimitedReasonAPIKeyGroupUnavailable || reason == OpsClientBusinessLimitedReasonIPRestriction {
			category = "permission"
		}
	}
	if upstream && category == "auth" {
		category = "upstream_auth"
	}
	if upstream && (category == "balance" || category == "plan") {
		category = "upstream_quota"
	}
	if upstream && category == "rate" {
		category = "upstream_rate"
	}
	if upstream && category == "permission" {
		category = "denied"
	}
	if category == "" && upstreamStatus == 401 {
		category = "upstream_auth"
	}
	if c != nil {
		value, _ := c.Get(OpsUpstreamErrorsKey)
		if events, ok := value.([]*OpsUpstreamErrorEvent); ok && len(events) > 0 && events[len(events)-1] != nil && events[len(events)-1].Stage == string(GatewayFailureStageAccountAuth) && category != "policy" && category != "context" && category != "size" && category != "model" {
			category = "upstream_auth"
		}
	}
	// Explicit local codes must not inherit an earlier recovered upstream error.
	if code == "gateway_concurrency_limit" || code == "gateway_queue_full" || !upstream && strings.ToUpper(field("code").String()) == field("code").String() && code != "" && gatewayClientCodeCategories[code] != "" {
		category = gatewayClientCodeCategories[code]
	}
	if c != nil {
		reason := c.GetString(gatewayClientReasonKey)
		// A generic capacity marker must not hide a specific model/permission/
		// policy reason already identified by the existing admission logic.
		if reason != "capacity" || category == "" || category == "server" || category == "capacity" {
			if hint, ok := gatewayClientCodeCategories[reason]; ok {
				category = hint
			} else if _, ok := gatewayClientHints[reason]; ok {
				category = reason
			}
		}
	}
	n := status
	if stream {
		n = semantic
	}
	if category == "" {
		if typ == "invalid_request_error" && stream && n == 0 {
			category = "parameter"
		}
	}
	if category == "" {
		switch n {
		case 401:
			if upstream {
				category = "upstream_auth"
			} else {
				category = "auth"
			}
		case 403:
			category = "denied"
		case 404:
			category = "route"
		case 408, 504:
			category = "timeout"
		case 413:
			category = "size"
		case 400, 422:
			category = "parameter"
		case 429:
			if upstream {
				category = "upstream_rate"
			} else {
				category = "rate"
			}
		case 500, 502, 503:
			category = "server"
		case 529:
			category = "busy"
		default:
			category = "unknown"
		}
	}
	hint := gatewayClientHints[category]
	label := hint.label
	if n >= 400 && n <= 599 {
		if stream {
			label += fmt.Sprintf("｜流内错误 %d", n)
		} else {
			label += fmt.Sprintf("｜%d", n)
		}
	} else if safeCode != "" {
		label += "｜" + safeCode
	}
	message := "【" + label + "】" + hint.text
	if category == "rate" || category == "upstream_rate" {
		if c != nil {
			if seconds, err := strconv.Atoi(c.Writer.Header().Get("Retry-After")); err == nil && seconds > 0 {
				message += fmt.Sprintf("建议等待 %d 秒后重试。", seconds)
			}
		}
	}
	updated, err := sjson.SetBytes(payload, messagePath, message)
	if err != nil {
		return payload
	}
	return updated
}

// Keep this literal aligned with the existing handler profit-veto response.
const profitControlNoAccountsMessage = "No available accounts: all candidates rejected by group profit control"
