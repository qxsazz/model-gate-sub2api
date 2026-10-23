//go:build unit

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func recoveryTestConfig() *config.Config {
	return &config.Config{Gateway: config.GatewayConfig{Recovery: config.GatewayRecoveryConfig{
		Enabled: true, BudgetSeconds: 300, SelectionWaitSeconds: 120, MaxAttempts: 12,
		MaxWaiters: 256, MaxWaitersPerGroup: 64, MaxWaitersPerUser: 2,
	}}}
}

func newRecoveryTest(t *testing.T, stream bool) (*gin.Context, *gatewayRecovery, *service.APIKey) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("recovery_test_recorder", recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	key := &service.APIKey{UserID: 10001, GroupID: ptrInt64(10002)}
	started := false
	t.Cleanup(beginGatewayRecovery(c, recoveryTestConfig(), key, stream, &started))
	r := recoveryFromContext(c.Request.Context())
	now := time.Now()
	r.now = func() time.Time { return now }
	r.sleep = func(ctx context.Context, d time.Duration) bool {
		if ctx.Err() != nil {
			return false
		}
		now = now.Add(d)
		return true
	}
	return c, r, key
}

func TestGatewayRecoveryFirstSelectionWaitsWithoutWritingError(t *testing.T) {
	c, r, key := newRecoveryTest(t, false)
	require.False(t, r.active())
	require.True(t, r.sleep(c.Request.Context(), time.Hour))
	diag := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true, RecoveryAccountIDs: []int64{1}}}
	require.True(t, tryGatewaySelectionRecovery(c, service.ErrNoAvailableAccounts, diag, key, "model", service.PlatformOpenAI, nil))
	require.True(t, r.active())
	require.Equal(t, r.now().Add(300*time.Second-r.selectionWait), r.deadline)
	require.Equal(t, 1, r.waits)
	require.False(t, c.Writer.Written(), "a transient first selection failure must not emit a 503")
	require.Equal(t, "model", diag.calls[0].Model)
}

func TestGatewayRecovery503ThenSuccessPreservesPermanentExclusions(t *testing.T) {
	c, r, key := newRecoveryTest(t, false)
	diag := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{RecoveryAccountIDs: []int64{1, 2, 3}}}
	excluded := map[int64]struct{}{2: {}, 3: {}}
	// Invalid credentials and model/profit exclusions never gain a retry timer.
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 2, &service.UpstreamFailoverError{StatusCode: 401, Stage: service.GatewayFailureStageAccountAuth}, 3))
	fs := NewFailoverState(0, false)
	for attempt := 0; attempt < 2; attempt++ {
		e := &service.UpstreamFailoverError{StatusCode: 503}
		require.Equal(t, FailoverContinue, fs.HandleFailoverError(c.Request.Context(), nil, 1, service.PlatformOpenAI, 2, e))
		excluded[1] = struct{}{}
		for i := 0; i < 8; i++ {
			if _, blocked := excluded[1]; !blocked {
				break
			}
			require.True(t, tryGatewaySelectionRecovery(c, service.ErrNoAvailableAccounts, diag, key, "model", service.PlatformOpenAI, excluded))
		}
		require.NotContains(t, excluded, int64(1))
		require.Contains(t, excluded, int64(2))
		require.Contains(t, excluded, int64(3))
	}
	c.JSON(200, gin.H{"result": "recovered"})
	require.Equal(t, 200, c.Writer.Status())
	require.Equal(t, 3, r.failures)
}

func TestGatewayRecoveryImmediateFailuresDoNotWait(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		diag service.ModelAvailabilityDiagnosis
	}{
		{"empty pool", service.ErrNoAvailableAccounts, service.ModelAvailabilityDiagnosis{}},
		{"unsupported model", service.ErrNoAvailableAccounts, service.ModelAvailabilityDiagnosis{HasAccountsInPool: true}},
		{"unknown diagnostic", service.ErrNoAvailableAccounts, service.ModelAvailabilityDiagnosis{HasAccountsInPool: true, HasModelSupport: true}},
		{"user balance", errors.New("insufficient balance"), service.ModelAvailabilityDiagnosis{RecoveryAccountIDs: []int64{1}}},
		{"database", errors.New("database unavailable"), service.ModelAvailabilityDiagnosis{RecoveryAccountIDs: []int64{1}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, r, key := newRecoveryTest(t, false)
			require.False(t, tryGatewaySelectionRecovery(c, tc.err, &fakeDiagnoser{resp: tc.diag}, key, "model", service.PlatformOpenAI, nil))
			require.Zero(t, r.waits)
			require.False(t, r.active())
		})
	}
}

func TestGatewayRecoveryBoundedAndCancelSafe(t *testing.T) {
	c, r, key := newRecoveryTest(t, false)
	diag := &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{RecoveryAccountIDs: []int64{1}}}
	r.cfg.SelectionWaitSeconds = 2
	for i := 0; i < 10; i++ {
		if !tryGatewaySelectionRecovery(c, service.ErrNoAvailableAccounts, diag, key, "model", service.PlatformOpenAI, nil) {
			break
		}
	}
	require.LessOrEqual(t, r.selectionWait, 2*time.Second)
	require.LessOrEqual(t, r.waits, 2)
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	require.False(t, tryGatewaySelectionRecovery(c, service.ErrNoAvailableAccounts, diag, key, "model", service.PlatformOpenAI, nil))
}

func TestGatewayRecoveryFailureBudgetIsPerRequest(t *testing.T) {
	c, r, _ := newRecoveryTest(t, false)
	r.cfg.MaxAttempts = 2
	e := &service.UpstreamFailoverError{StatusCode: 503}
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, e, 3))
	require.False(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, e, 3))
	c2, r2, _ := newRecoveryTest(t, false)
	require.True(t, observeGatewayRecoveryFailure(c2.Request.Context(), 1, e, 3))
	require.Equal(t, 1, r2.failures)
}

func TestGatewayRecoveryRespectsRetryAfterAndStopDecisions(t *testing.T) {
	c, r, key := newRecoveryTest(t, false)
	e := &service.UpstreamFailoverError{StatusCode: 503, ResponseHeaders: http.Header{"Retry-After": []string{"900"}}}
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, e, 3))
	require.False(t, tryGatewaySelectionRecovery(c, service.ErrNoAvailableAccounts, &fakeDiagnoser{resp: service.ModelAvailabilityDiagnosis{RecoveryAccountIDs: []int64{1}}}, key, "model", service.PlatformOpenAI, map[int64]struct{}{1: {}}))
	for _, e := range []*service.UpstreamFailoverError{
		{StatusCode: 503, NextAccountAction: service.NextAccountStop},
		{StatusCode: 503, BoundUpstreamPassthrough: true},
		{StatusCode: 403, Stage: service.GatewayFailureStageAccountAuth},
		{StatusCode: 503, Scope: service.GatewayFailureScopeRequest},
	} {
		require.False(t, recoverableGatewayFailure(e))
	}
	require.Zero(t, r.waits)
}

func TestGatewayRecoveryHeartbeatPreservesProtocolAndReplayBoundary(t *testing.T) {
	for _, protocol := range []string{"chat", "responses", "google"} {
		t.Run(protocol, func(t *testing.T) {
			c, r, _ := newRecoveryTest(t, true)
			if protocol == "responses" {
				c.Request.URL.Path = "/v1/responses"
			}
			r.activate()
			require.True(t, r.wait(11*time.Second))
			require.True(t, gatewayRecoveryOnlyHeartbeat(c))
			if protocol == "responses" {
				(&GatewayHandler{}).responsesErrorResponse(c, 503, "server_error", "exhausted")
			} else if protocol == "google" {
				googleError(c, 503, "exhausted")
			} else {
				(&GatewayHandler{}).chatCompletionsErrorResponse(c, 503, "server_error", "exhausted")
			}
			recorder := c.Writer.(interface{ Status() int })
			require.Equal(t, 200, recorder.Status())
			require.False(t, gatewayRecoveryOnlyHeartbeat(c))
			body := c.MustGet("recovery_test_recorder").(*httptest.ResponseRecorder).Body.String()
			require.Contains(t, body, ": ping\n\n")
			require.Contains(t, body, "exhausted")
			if protocol == "responses" {
				require.Contains(t, body, "event: response.failed\n")
			} else if protocol == "google" {
				// Keep the gateway's existing Google status mapping on SSE errors.
				require.Contains(t, body, `"code":503`)
				require.Contains(t, body, `"status":"INTERNAL"`)
			} else {
				require.Contains(t, body, `"type":"error"`)
			}
		})
	}
	c, r, _ := newRecoveryTest(t, true)
	r.activate()
	require.True(t, r.wait(11*time.Second))
	before := c.Writer.Size()
	_, err := fmt.Fprint(c.Writer, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
	require.NoError(t, err)
	require.False(t, gatewayRecoveryOnlyHeartbeat(c))
	require.False(t, openAIForwardMayFailover(c, before, &service.UpstreamFailoverError{StatusCode: 503}))
}

func TestGatewayRecoveryWaiterAdmissionAndRelease(t *testing.T) {
	_, r, _ := newRecoveryTest(t, false)
	r.cfg.MaxWaitersPerUser = 1
	require.True(t, r.admit())
	_, other, _ := newRecoveryTest(t, false)
	other.cfg.MaxWaitersPerUser = 1
	require.False(t, other.admit())
	r.close()
	require.True(t, other.admit())
	other.close()
	require.NotContains(t, recoveryWaiters.users, r.userID)
}

func TestGatewayRecoverySameAccountBackoffUsesWaiterAdmission(t *testing.T) {
	_, busy, _ := newRecoveryTest(t, false)
	busy.cfg.MaxWaitersPerUser = 1
	require.True(t, busy.admit())
	c, waiting, _ := newRecoveryTest(t, false)
	waiting.cfg.MaxWaitersPerUser = 1
	waiting.activate()
	require.False(t, waitGatewayRecovery(c.Request.Context(), time.Second))
	busy.close()
	require.True(t, waitGatewayRecovery(c.Request.Context(), time.Second))
	require.False(t, waiting.admitted, "same-account backoff must release its admission before retrying upstream")
}

func TestGatewayRecoveryWebSocketBudgetResetsOnlyForNewCompletedTurn(t *testing.T) {
	c, r, _ := newRecoveryTest(t, false)
	ctx := c.Request.Context()
	r.activate()
	r.failures = r.cfg.MaxAttempts
	r.selectionWait = 120 * time.Second
	r.retryAt[1] = r.now().Add(time.Second)
	require.False(t, startNewGatewayRecoveryTurn(ctx), "retrying the same turn must not reset its budget")
	require.False(t, r.available())
	markGatewayRecoveryTurnComplete(ctx)
	// A long idle conversation may outlive the previous turn's deadline.
	r.deadline = r.now().Add(-time.Hour)
	require.True(t, startNewGatewayRecoveryTurn(ctx))
	require.False(t, r.active(), "a new healthy turn must use normal scheduling")
	require.False(t, r.available())
	require.Zero(t, r.failures)
	require.Zero(t, r.selectionWait)
	require.Empty(t, r.retryAt)
	require.True(t, r.deadline.IsZero())
	require.True(t, r.sleep(ctx, time.Hour))
	require.True(t, observeGatewayRecoveryFailure(ctx, 1, &service.UpstreamFailoverError{StatusCode: 503}, 3))
	require.Equal(t, r.now().Add(300*time.Second), r.deadline)
	require.False(t, startNewGatewayRecoveryTurn(ctx))
}

func TestGatewayRecoveryExplicitZeroRetryAndPermanentFailureStayExcluded(t *testing.T) {
	c, r, _ := newRecoveryTest(t, false)
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, &service.UpstreamFailoverError{StatusCode: 503}, 0))
	require.NotContains(t, r.retryAt, int64(1))
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 2, &service.UpstreamFailoverError{StatusCode: 401, Stage: service.GatewayFailureStageAccountAuth}, 3))
	require.NotContains(t, r.retryAt, int64(2))
	for attempt := 0; attempt < 3; attempt++ {
		require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 3, &service.UpstreamFailoverError{StatusCode: 503}, 2))
	}
	require.NotContains(t, r.retryAt, int64(3), "selection polling must not bypass the account retry cap")
}

func TestGatewayRecoveryAccountSlotWaitIsBoundedAndReleasesAdmission(t *testing.T) {
	c, r, _ := newRecoveryTest(t, false)
	require.True(t, observeGatewayRecoveryFailure(c.Request.Context(), 1, &service.UpstreamFailoverError{StatusCode: 502}, 3))
	// Exercise the real concurrency helper after a recoverable upstream failure.
	cache := &helperConcurrencyCacheStub{accountSeq: []bool{false, false, true}}
	helper := NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second)
	release, err := helper.AcquireAccountSlotWithWaitTimeout(c, 1, 1, time.Millisecond, false, nil)
	require.NoError(t, err, "recovery should tolerate the legacy one millisecond timeout")
	require.NotNil(t, release)
	release()
	require.False(t, r.admitted)
	r.selectionWait = time.Duration(r.cfg.SelectionWaitSeconds) * time.Second
	_, err = helper.AcquireAccountSlotWithWaitTimeout(c, 1, 1, time.Hour, false, nil)
	var timeout *ConcurrencyError
	require.ErrorAs(t, err, &timeout)
	require.True(t, timeout.IsTimeout)
	require.False(t, r.admitted)
}

func TestGatewayRecoveryHeartbeatAfterContentDoesNotResetReplayBoundary(t *testing.T) {
	c, r, _ := newRecoveryTest(t, true)
	_, err := fmt.Fprint(c.Writer, "data: assistant-content\n\n")
	require.NoError(t, err)
	r.activate()
	require.True(t, r.wait(11*time.Second))
	require.False(t, gatewayRecoveryOnlyHeartbeat(c))
}

func TestGatewayRecoveryTextAdmissionSkipsImageTasksAndDisabledConfig(t *testing.T) {
	for _, tc := range []struct {
		name, path, model, body string
		disabled                bool
	}{
		{"image endpoint", "/v1/images/generations", "gpt-image-1", `{"prompt":"draw"}`, false},
		{"image tool", "/v1/responses", "gpt-4.1", `{"tools":[{"type":"image_generation"}],"tool_choice":{"type":"image_generation"}}`, false},
		{"gemini image", "/v1beta/models/gemini:generateContent", "gemini-2.5-flash-image", `{"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`, false},
		{"disabled", "/v1/chat/completions", "gpt-4.1", `{}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			cfg := recoveryTestConfig()
			cfg.Gateway.Recovery.Enabled = !tc.disabled
			defer beginTextGatewayRecovery(c, cfg, &service.APIKey{UserID: 1}, false, nil, tc.model, []byte(tc.body))()
			require.Nil(t, recoveryFromContext(c.Request.Context()))
		})
	}
}
