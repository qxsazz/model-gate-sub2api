package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGatewayClientErrorWebSocketFrameKeepsCloseReason(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := coderws.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.CloseNow() }()
		writeContentModerationWSError(r.Context(), conn, &service.ContentModerationDecision{Blocked: true, StatusCode: 403, Message: "private original diagnostic"})
		_ = conn.Close(coderws.StatusPolicyViolation, "original short close reason")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	defer func() { _ = conn.CloseNow() }()
	typ, payload, err := conn.Read(ctx)
	require.NoError(t, err)
	require.Equal(t, coderws.MessageText, typ)
	require.Equal(t, "evt_content_moderation_blocked", gjson.GetBytes(payload, "event_id").String())
	require.Equal(t, "content_policy_violation", gjson.GetBytes(payload, "error.code").String())
	require.Contains(t, gjson.GetBytes(payload, "error.message").String(), "请求受策略限制")
	require.NotContains(t, string(payload), "private original diagnostic")
	_, _, err = conn.Read(ctx)
	require.Equal(t, coderws.StatusPolicyViolation, coderws.CloseStatus(err))
	require.Contains(t, err.Error(), "original short close reason")
}
