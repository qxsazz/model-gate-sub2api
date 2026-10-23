//go:build unit

package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Compare actual acquisition calls and deadlines, including a full recovery
// queue. A healthy request must never consult that queue or extend its WaitPlan.
func TestGatewayRecoveryInactivePreservesNormalAccountWait(t *testing.T) {
	for _, scenario := range []string{"immediate", "wait then acquire", "normal timeout"} {
		t.Run(scenario, func(t *testing.T) {
			var baselineCalls int
			for _, enabled := range []bool{false, true} {
				c, r, _ := newRecoveryTest(t, false)
				if !enabled {
					c.Request = c.Request.WithContext(context.Background())
				}
				r.cfg.MaxWaiters = 0 // A full recovery queue must not reject normal traffic.
				r.cfg.MaxWaitersPerUser = 0
				wait := time.Second
				if scenario == "normal timeout" {
					wait = 30 * time.Millisecond
				}
				calls := 0
				cache := &concurrencyCacheMock{acquireAccountSlotFn: func(ctx context.Context, id int64, limit int, _ string) (bool, error) {
					calls++
					require.Equal(t, int64(42), id)
					require.Equal(t, 3, limit)
					deadline, ok := ctx.Deadline()
					require.True(t, ok, "use the original WaitPlan context even for the immediate attempt")
					require.WithinDuration(t, time.Now().Add(wait), deadline, 250*time.Millisecond)
					return scenario == "immediate" || (scenario == "wait then acquire" && calls == 2), nil
				}}
				helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)
				start := time.Now()
				release, err := helper.AcquireAccountSlotWithWaitTimeout(c, 42, 3, wait, false, nil)
				if scenario == "normal timeout" {
					var timeout *ConcurrencyError
					require.ErrorAs(t, err, &timeout)
					require.True(t, timeout.IsTimeout)
					require.Less(t, time.Since(start), time.Second)
				} else {
					require.NoError(t, err)
					require.NotNil(t, release)
					release()
				}
				if enabled {
					require.Equal(t, baselineCalls, calls)
				} else {
					baselineCalls = calls
				}
				require.False(t, r.active())
				require.False(t, r.admitted)
				require.Zero(t, r.selectionWait)
				require.Zero(t, r.failures)
			}
		})
	}
}

func TestGatewayRecoveryStartsBudgetAtFailureNotRequestStart(t *testing.T) {
	c, r, _ := newRecoveryTest(t, false)
	require.False(t, r.active())
	require.True(t, r.sleep(c.Request.Context(), time.Hour))
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, &service.UpstreamFailoverError{StatusCode: 502}, 3))
	require.True(t, r.available())
	require.Equal(t, r.now().Add(300*time.Second), r.deadline)
	deadline := r.deadline
	require.True(t, r.sleep(c.Request.Context(), time.Second))
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 2, &service.UpstreamFailoverError{StatusCode: 503}, 3))
	require.Equal(t, deadline, r.deadline, "a second failure must not replenish recovery time")
}

func TestGatewayRecoveryDoesNotActivateForUnsafeOrUnknownFailure(t *testing.T) {
	for _, e := range []*service.UpstreamFailoverError{
		{StatusCode: 503, NextAccountAction: service.NextAccountStop},
		{StatusCode: 503, BoundUpstreamPassthrough: true},
		{StatusCode: 503, Scope: service.GatewayFailureScopeRequest},
		{StatusCode: 400}, {StatusCode: 402}, {StatusCode: 418},
	} {
		c, r, _ := newRecoveryTest(t, false)
		require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, e, 3))
		require.False(t, r.active())
		require.Zero(t, r.failures)
		require.False(t, gatewayRecoveryCanRetry(c.Request.Context(), e))
	}
}

func TestGatewayRecoveryHealthyHTTPUsesSamePriorityWithSwitchesDisabled(t *testing.T) {
	for _, tc := range []struct{ path, body string }{
		{"/openai/v1/responses", `{"model":"grok","input":"hello","stream":false}`},
		{"/openai/v1/chat/completions", `{"model":"grok","messages":[{"role":"user","content":"hello"}],"stream":false}`},
		{"/openai/v1/messages", `{"model":"grok","max_tokens":16,"messages":[{"role":"user","content":"hello"}]}`},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, enabled := range []bool{false, true} {
				h, repo, upstream, router, cleanup := newGrokCredentialFailoverHandler(t, "first_429")
				defer cleanup()
				h.cfg.Gateway.Recovery = recoveryTestConfig().Gateway.Recovery
				h.cfg.Gateway.Recovery.Enabled = enabled
				h.cfg.Gateway.Recovery.MaxWaiters = 0
				h.maxAccountSwitches = 0
				upstream.rateLimitIDs = nil
				w := recoveryHTTPRequest(router, tc.path, tc.body)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, []int64{801}, upstream.accountHits())
				require.Equal(t, 1, repo.selectorCalls())
			}
		})
	}
}
