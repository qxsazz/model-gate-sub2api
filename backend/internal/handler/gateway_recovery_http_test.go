//go:build unit

package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func (r *grokCredentialHandlerRepo) ListModelAvailabilityCandidates(_ context.Context, _ *int64, platforms []string, _ bool) ([]service.Account, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []service.Account
	for _, a := range r.accounts {
		for _, p := range platforms {
			if a.Platform == p && a.IsActive() && a.Schedulable {
				out = append(out, a)
				break
			}
		}
	}
	return out, nil
}

func recoveryHTTPRequest(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	return w
}

func TestGatewayRecoveryHTTPFirstRequestTransientEmptyPoolThenHealthy(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/openai/v1/responses", `{"model":"grok","input":"hello","stream":false}`},
		{"/openai/v1/chat/completions", `{"model":"grok","messages":[{"role":"user","content":"hello"}],"stream":false}`},
		{"/openai/v1/messages", `{"model":"grok","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			h, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "revoked")
			defer cleanup()
			h.cfg.Gateway.Recovery = recoveryTestConfig().Gateway.Recovery
			repo.unavailableSelections = 1
			w := recoveryHTTPRequest(router, tc.path, tc.body)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, []int64{802}, upstream.accountHits(), "revoked account must be refreshed/isolated, then a healthy account serves the same request")
			require.NotContains(t, w.Body.String(), "503")
		})
	}
}

func TestGatewayRecoveryHTTPUnsupportedModelDoesNotQueue(t *testing.T) {
	h, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "revoked")
	defer cleanup()
	h.cfg.Gateway.Recovery = recoveryTestConfig().Gateway.Recovery
	for i := range repo.accounts {
		repo.accounts[i].Credentials["model_mapping"] = map[string]any{"another-model": "another-model"}
	}
	w := recoveryHTTPRequest(router, "/openai/v1/responses", `{"model":"grok","input":"hello","stream":false}`)
	require.Equal(t, 404, w.Code, w.Body.String())
	require.Empty(t, upstream.accountHits())
	require.Equal(t, 1, repo.selectorCalls())
}

func TestGatewayRecoveryHTTP503SwitchesHealthyBeforeReturningError(t *testing.T) {
	h, _, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "first_429")
	defer cleanup()
	h.cfg.Gateway.Recovery = recoveryTestConfig().Gateway.Recovery
	h.maxAccountSwitches = 0
	upstream.rateLimitIDs = nil
	upstream.failureStatus = map[int64]int{801: 503}
	w := recoveryHTTPRequest(router, "/openai/v1/responses", `{"model":"grok","input":"hello","stream":false}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	hits := upstream.accountHits()
	require.Contains(t, hits, int64(801))
	require.Equal(t, int64(802), hits[len(hits)-1])
	require.LessOrEqual(t, len(hits), 12)
}

func TestGatewayRecoveryWebSocketFirstSelectionAnd503Recover(t *testing.T) {
	for _, scenario := range []string{"first selection busy", "upstream 503"} {
		t.Run(scenario, func(t *testing.T) {
			h, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "first_429")
			defer cleanup()
			h.cfg.Gateway.Recovery = recoveryTestConfig().Gateway.Recovery
			h.maxAccountSwitches = 0
			upstream.rateLimitIDs = nil
			if scenario == "first selection busy" {
				repo.unavailableSelections = 1
			} else {
				upstream.failureStatus = map[int64]int{801: 503}
			}
			server := httptest.NewServer(router)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/openai/v1/responses", nil)
			require.NoError(t, err)
			defer conn.CloseNow()
			require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"grok","input":"hello","stream":false}`)))
			_, payload, err := conn.Read(ctx)
			require.NoError(t, err, "the gateway must recover before closing the connection")
			require.Contains(t, string(payload), "resp_healthy")
			if scenario == "first selection busy" {
				require.GreaterOrEqual(t, repo.selectorCalls(), 2)
			} else {
				hits := upstream.accountHits()
				require.Contains(t, hits, int64(801))
				require.Equal(t, int64(802), hits[len(hits)-1])
			}
		})
	}
}
