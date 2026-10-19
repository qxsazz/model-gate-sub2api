package service

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestGatewayClientErrorMessage(t *testing.T) {
	for _, tt := range []struct {
		name        string
		status      int
		body, label string
		upstream    int
	}{
		{"local key", 401, `{"code":"INVALID_API_KEY","message":"bad key"}`, "认证失败", 0},
		{"upstream key", 502, `{"error":{"type":"upstream_error","message":"rejected"}}`, "上游认证异常", 401},
		{"local permission", 403, `{"code":"GROUP_NOT_ALLOWED","message":"denied"}`, "访问受限", 0},
		{"unknown permission", 403, `{"error":{"message":"private internal detail"}}`, "请求被拒绝", 0},
		{"unknown upstream permission", 403, `{"error":{"message":"private internal detail"}}`, "请求被拒绝", 403},
		{"balance", 403, `{"code":"INSUFFICIENT_BALANCE","message":"balance"}`, "余额不足", 0},
		{"plan", 403, `{"code":"SUBSCRIPTION_QUOTA_EXCEEDED","message":"quota"}`, "套餐额度不足", 0},
		{"provider quota", 429, `{"error":{"code":"insufficient_quota","message":"quota"}}`, "上游额度不足", 429},
		{"user quota", 429, `{"error":{"code":"insufficient_quota","message":"quota"}}`, "套餐额度不足", 0},
		{"user concurrency", 429, `{"error":{"code":"gateway_concurrency_limit","message":"limit"}}`, "请求过于频繁", 0},
		{"provider rate", 429, `{"error":{"type":"rate_limit_error","message":"limited"}}`, "上游请求受限", 429},
		{"expired", 403, `{"code":"API_KEY_EXPIRED","message":"expired"}`, "账户或套餐已到期", 0},
		{"model", 404, `{"error":{"code":"model_not_found","message":"missing"}}`, "模型不可用", 0},
		{"route", 404, `{"error":{"message":"missing"}}`, "接口不存在", 0},
		{"context", 400, `{"error":{"code":"context_length_exceeded","message":"long"}}`, "会话内容过长", 0},
		{"payload", 413, `{"error":{"message":"big"}}`, "请求内容过大", 0},
		{"policy", 403, `{"error":{"code":"content_policy_violation","message":"blocked"}}`, "请求受策略限制", 0},
		{"timeout", 504, `{"error":{"message":"private proxy https://secret/?key=abc"}}`, "请求超时", 0},
		{"bad parameter", 422, `{"error":{"message":"bad"}}`, "请求参数错误", 0},
		{"server", 503, `{"error":{"type":"api_error","message":"private internal detail"}}`, "服务暂时不可用", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tt.upstream != 0 {
				SetOpsUpstreamError(c, tt.upstream, "original provider message", "")
			}
			original := []byte(tt.body)
			got := RewriteGatewayClientError(c, original, tt.status, "")
			path := "error.message"
			if !gjson.GetBytes(original, path).Exists() {
				path = "message"
			}
			message := gjson.GetBytes(got, path).String()
			require.Contains(t, message, "【"+tt.label+"｜")
			require.NotContains(t, message, "private")
			before, err := sjson.DeleteBytes(original, path)
			require.NoError(t, err)
			after, err := sjson.DeleteBytes(got, path)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after), "only message may change")
			require.Equal(t, tt.body, string(original), "input is immutable")
		})
	}
}

func TestGatewayClientErrorCustomRuleAndTurnReset(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	custom := "管理员明确指定的提示"
	svc := &ErrorPassthroughService{localCache: []*cachedPassthroughRule{{ErrorPassthroughRule: &model.ErrorPassthroughRule{ID: 1, Enabled: true, ErrorCodes: []int{503}, MatchMode: model.MatchModeAny, CustomMessage: &custom}, errorCodeSet: map[int]struct{}{503: {}}}}}
	BindErrorPassthroughService(c, svc)
	status, typ, message, matched := applyErrorPassthroughRule(c, "openai", 503, []byte(`{"error":{"message":"internal"}}`), 502, "api_error", "default")
	require.True(t, matched)
	require.Equal(t, 503, status)
	require.Equal(t, "upstream_error", typ)
	require.Equal(t, custom, message)
	body := []byte(`{"error":{"message":"管理员明确指定的提示"}}`)
	require.Equal(t, string(body), string(RewriteGatewayClientError(c, body, status, "")))
	BeginOpsStreamTurn(c, 2)
	require.Contains(t, string(RewriteGatewayClientError(c, body, status, "")), "【服务暂时不可用｜503】")
}

func TestGatewayClientErrorLocalReasonSurvivesRecoveredUpstream(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetOpsUpstreamError(c, 503, "earlier recovered failure", "")
	SetGatewayClientErrorReason(c, "INSUFFICIENT_BALANCE")
	got := RewriteGatewayClientError(c, []byte(`{"error":{"type":"billing_error","message":"internal"}}`), 403, "")
	require.Contains(t, string(got), "【余额不足｜403】")
	require.Equal(t, 503, c.GetInt(OpsUpstreamStatusCodeKey))
}

func TestGatewayClientErrorSpecificReasonPrecedesCapacity(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	SetGatewayClientErrorReason(c, "capacity")
	body := []byte(`{"error":{"code":"model_not_found","message":"No available accounts"}}`)
	got := RewriteGatewayClientError(c, body, 404, "")
	require.Contains(t, string(got), "【模型不可用｜404】")
	require.Equal(t, "model_not_found", gjson.GetBytes(got, "error.code").String())
}

func TestGatewayClientErrorStreamStatusAndBypass(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte(`{"type":"response.failed","sequence_number":9007199254740993,"response":{"error":{"code":"server_error","message":"internal"}}}`)
	got := RewriteGatewayClientError(c, body, 200, "response.failed")
	require.NotContains(t, string(got), "503", "server_error alone does not prove a numeric status")
	require.Contains(t, string(got), "server_error")
	MarkOpsStreamFailure(c, "server_error", "server_error", "internal", 503)
	got = RewriteGatewayClientError(c, body, 200, "response.failed")
	require.Contains(t, string(got), "流内错误 503")
	require.Contains(t, string(got), "9007199254740993")
	require.Equal(t, "internal", gjson.GetBytes(body, "response.error.message").String())
	for _, raw := range []string{`not json`, `{"type":"response.output_text.delta","delta":"error: server_error"}`, `{"type":"response.completed","response":{"output":[{"error":{"message":"fictional"}}]}}`, `{"error":{"code":"session_blocked_by_cyber_policy","message":"专用安全说明"}}`} {
		require.Equal(t, raw, string(RewriteGatewayClientError(c, []byte(raw), 200, "")))
	}
	PreserveGatewayClientErrorMessage(c)
	require.Equal(t, string(body), string(RewriteGatewayClientError(c, body, 200, "response.failed")))
}

func TestGatewayClientErrorGoogleAndRetryAfter(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Header("Retry-After", "7")
	body := []byte(`{"error":{"code":503,"status":"UNAVAILABLE","message":"internal","details":[{"reason":"keep"}]}}`)
	got := RewriteGatewayClientError(c, body, 200, "")
	require.Contains(t, string(got), "流内错误 503")
	require.Equal(t, "UNAVAILABLE", gjson.GetBytes(got, "error.status").String())
	require.Equal(t, "keep", gjson.GetBytes(got, "error.details.0.reason").String())
	require.Equal(t, "7", c.Writer.Header().Get("Retry-After"))
	require.False(t, strings.Contains(string(got), "充值"))
}
