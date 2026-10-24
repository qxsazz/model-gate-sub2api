package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type gatewayRecoveryKey struct{}

// Recovery counters belong to one request or WebSocket turn.
// Timers control recovery admission, not an already-running successful stream.
type gatewayRecovery struct {
	c               *gin.Context
	cfg             config.GatewayRecoveryConfig
	deadline        time.Time
	selectionWait   time.Duration
	failures        int
	accountFailures map[int64]int
	waits           int
	retryAt         map[int64]time.Time
	stream          bool
	streamStarted   *bool
	heartbeatAt     time.Time
	userID, groupID int64
	admitted        bool
	nextTurn        bool
	now             func() time.Time
	sleep           func(context.Context, time.Duration) bool
}

var recoveryWaiters = struct {
	sync.Mutex
	total  int
	users  map[int64]int
	groups map[int64]int
}{users: make(map[int64]int), groups: make(map[int64]int)}

func beginGatewayRecovery(c *gin.Context, cfg *config.Config, key *service.APIKey, stream bool, started *bool) func() {
	if c == nil || c.Request == nil || cfg == nil || !cfg.Gateway.Recovery.Enabled || key == nil {
		return func() {}
	}
	r := &gatewayRecovery{c: c, cfg: cfg.Gateway.Recovery, stream: stream, streamStarted: started,
		retryAt: make(map[int64]time.Time), accountFailures: make(map[int64]int), userID: key.UserID, now: time.Now, sleep: sleepWithContext}
	if key.GroupID != nil {
		r.groupID = *key.GroupID
	}
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), gatewayRecoveryKey{}, r))
	return r.close
}

func recoveryFromContext(ctx context.Context) *gatewayRecovery {
	if ctx == nil {
		return nil
	}
	r, _ := ctx.Value(gatewayRecoveryKey{}).(*gatewayRecovery)
	return r
}

// Merely enabling recovery must not change normal scheduling or its WaitPlan.
// Start the window only after a confirmed recoverable failure, once per turn.
func (r *gatewayRecovery) active() bool {
	return r != nil && !r.deadline.IsZero()
}

func (r *gatewayRecovery) activate() {
	if !r.active() {
		now := r.now()
		r.deadline = now.Add(time.Duration(r.cfg.BudgetSeconds) * time.Second)
		r.heartbeatAt = now.Add(10 * time.Second)
	}
}

func (r *gatewayRecovery) available() bool {
	return r.active() && r.c.Request.Context().Err() == nil && r.now().Before(r.deadline) && r.failures < r.cfg.MaxAttempts
}

func (r *gatewayRecovery) admit() bool {
	if r.admitted {
		return true
	}
	recoveryWaiters.Lock()
	defer recoveryWaiters.Unlock()
	if recoveryWaiters.total >= r.cfg.MaxWaiters || recoveryWaiters.users[r.userID] >= r.cfg.MaxWaitersPerUser || recoveryWaiters.groups[r.groupID] >= r.cfg.MaxWaitersPerGroup {
		return false
	}
	recoveryWaiters.total++
	recoveryWaiters.users[r.userID]++
	recoveryWaiters.groups[r.groupID]++
	r.admitted = true
	return true
}

func (r *gatewayRecovery) close() {
	if !r.admitted {
		return
	}
	recoveryWaiters.Lock()
	recoveryWaiters.total--
	recoveryWaiters.users[r.userID]--
	recoveryWaiters.groups[r.groupID]--
	if recoveryWaiters.users[r.userID] == 0 {
		delete(recoveryWaiters.users, r.userID)
	}
	if recoveryWaiters.groups[r.groupID] == 0 {
		delete(recoveryWaiters.groups, r.groupID)
	}
	r.admitted = false
	recoveryWaiters.Unlock()
	slog.Info("gateway.recovery_finished", "failures", r.failures, "waits", r.waits, "selection_wait_ms", r.selectionWait.Milliseconds())
}

// Only explicit transient failures may re-enter a previously failed account.
// Credentials, model mismatches, policy denials and unknown failures stay excluded.
func recoverableGatewayFailure(e *service.UpstreamFailoverError) bool {
	if e == nil || !e.ShouldRetryNextAccount() || e.BoundUpstreamPassthrough || e.IsCredentialFailure() {
		return false
	}
	if e.Scope == service.GatewayFailureScopeRequest && !e.RequestScopedTransient {
		return false
	}
	switch e.StatusCode {
	case 429, 500, 502, 503, 504:
		return true
	default:
		return e.RequestScopedTransient
	}
}

func observeGatewayRecoveryFailure(ctx context.Context, id int64, e *service.UpstreamFailoverError, sameAccountLimit int) bool {
	r := recoveryFromContext(ctx)
	if r == nil || !eligibleGatewayRecoveryFailure(e) {
		return true
	}
	r.activate()
	r.failures++
	r.accountFailures[id]++
	delete(r.retryAt, id)
	if recoverableGatewayFailure(e) && (sameAccountLimit > 0 && r.accountFailures[id] <= sameAccountLimit) && (e.SameAccountRetryMax <= 0 || r.accountFailures[id] <= e.SameAccountRetryMax) {
		delay := time.Duration(1<<min(r.failures, 3)) * time.Second
		if e.SameAccountRetryDelay > delay {
			delay = e.SameAccountRetryDelay
		}
		if raw := e.ResponseHeaders.Get("Retry-After"); raw != "" {
			if n, err := strconv.ParseInt(raw, 10, 32); err == nil && n > 0 {
				delay = max(delay, time.Duration(n)*time.Second)
			} else if at, err := http.ParseTime(raw); err == nil {
				delay = max(delay, at.Sub(r.now()))
			}
		}
		at := r.now().Add(delay)
		if e.SameAccountRetryDeadline.IsZero() || at.Before(e.SameAccountRetryDeadline) {
			r.retryAt[id] = at
		}
	}
	return r.available()
}

// Called only at existing safe failover points; it never authorizes replay after
// semantic output or overrides provider-specific continuation / OAuth guards.
func gatewayRecoveryCanRetry(ctx context.Context, e *service.UpstreamFailoverError) bool {
	r := recoveryFromContext(ctx)
	return r.available() && eligibleGatewayRecoveryFailure(e)
}

func eligibleGatewayRecoveryFailure(e *service.UpstreamFailoverError) bool {
	return e != nil && e.ShouldRetryNextAccount() && !e.BoundUpstreamPassthrough &&
		(recoverableGatewayFailure(e) || e.IsCredentialFailure())
}

func tryGatewaySelectionRecovery(c *gin.Context, err error, diag service.ModelAvailabilityDiagnoser, key *service.APIKey, model, platform string, excluded map[int64]struct{}) bool {
	r := recoveryFromContext(c.Request.Context())
	if r == nil || c.Request.Context().Err() != nil || (r.active() && !r.available()) || !errors.Is(err, service.ErrNoAvailableAccounts) || diag == nil || key == nil {
		return false
	}
	remaining := time.Duration(r.cfg.SelectionWaitSeconds)*time.Second - r.selectionWait
	if r.active() {
		remaining = min(remaining, r.deadline.Sub(r.now()))
	} else {
		remaining = min(remaining, time.Duration(r.cfg.BudgetSeconds)*time.Second)
	}
	if remaining <= 0 {
		return false
	}
	// A failed diagnostic lookup produces no RecoveryAccountIDs. Never queue on
	// an empty pool, unsupported model, exhausted total quota or unknown DB error.
	d := diag.DiagnoseModelAvailabilityForPlatform(c.Request.Context(), key.GroupID, model, platform)
	var candidates []int64
	for _, id := range d.RecoveryAccountIDs {
		if _, failed := excluded[id]; failed {
			at, transient := r.retryAt[id]
			if !transient || !at.Before(r.now().Add(remaining)) {
				continue
			}
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return false
	}
	r.activate()
	remaining = min(remaining, r.deadline.Sub(r.now()))
	if remaining <= 0 || !r.available() || !r.admit() {
		return false
	}
	delay := min(time.Duration(1<<min(r.waits, 3))*time.Second, remaining)
	// Jitter prevents a recovered provider from being hit by synchronized queues.
	delay = min(delay+time.Duration(rand.IntN(250))*time.Millisecond, remaining)
	r.waits++
	slog.Info("gateway.recovery_wait", "round", r.waits, "delay_ms", delay.Milliseconds(), "failures", r.failures)
	start := r.now()
	if !r.wait(delay) {
		return false
	}
	r.selectionWait += r.now().Sub(start)
	for _, id := range candidates {
		if at, ok := r.retryAt[id]; ok && !r.now().Before(at) {
			delete(excluded, id)
		}
	}
	return r.available()
}

// Heartbeats are emitted by the handler goroutine only, between upstream
// attempts. They neither invent assistant content nor authorize unsafe replays.
func (r *gatewayRecovery) wait(delay time.Duration) bool {
	until := r.now().Add(delay)
	for r.now().Before(until) {
		step := min(until.Sub(r.now()), time.Second)
		if !r.sleep(r.c.Request.Context(), step) {
			return false
		}
		if r.stream && !r.now().Before(r.heartbeatAt) {
			r.c.Header("Content-Type", "text/event-stream")
			r.c.Header("Cache-Control", "no-cache")
			r.c.Header("X-Accel-Buffering", "no")
			if written, err := fmt.Fprint(r.c.Writer, ": ping\n\n"); err != nil {
				return false
			} else {
				recordGatewayStreamHeartbeat(r.c, written)
			}
			r.c.Writer.Flush()
			if r.streamStarted != nil {
				*r.streamStarted = true
			}
			r.heartbeatAt = r.now().Add(10 * time.Second)
		}
	}
	return true
}

func gatewayRecoveryOnlyHeartbeat(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return recoveryFromContext(c.Request.Context()) != nil && gatewayStreamHasOnlyHeartbeats(c)
}

// Same-account sleeps share the same recovery window and heartbeat path.
func waitGatewayRecovery(ctx context.Context, delay time.Duration) bool {
	r := recoveryFromContext(ctx)
	if !r.active() {
		return sleepWithContext(ctx, delay)
	}
	if !r.available() {
		return false
	}
	alreadyAdmitted := r.admitted
	if !r.admit() {
		return false
	}
	if !alreadyAdmitted {
		defer r.close()
	}
	return r.wait(min(delay, r.deadline.Sub(r.now()))) && r.available()
}

func releaseGatewayRecoveryWait(c *gin.Context) {
	if r := recoveryFromContext(c.Request.Context()); r != nil {
		r.close()
	}
}

func beginTextGatewayRecovery(c *gin.Context, cfg *config.Config, key *service.APIKey, stream bool, started *bool, model string, body []byte) func() {
	if service.IsExplicitImageGenerationIntent(c.Request.URL.Path, model, body) || service.IsGeminiImageGenerationIntent(body, model) {
		return func() {}
	}
	return beginGatewayRecovery(c, cfg, key, stream, started)
}

func markGatewayRecoveryTurnComplete(ctx context.Context) {
	if r := recoveryFromContext(ctx); r != nil {
		r.nextTurn = true
	}
}

func startNewGatewayRecoveryTurn(ctx context.Context) bool {
	r := recoveryFromContext(ctx)
	if r == nil || !r.nextTurn {
		return false
	}
	r.close()
	r.nextTurn = false
	r.deadline = time.Time{}
	r.heartbeatAt = time.Time{}
	r.selectionWait = 0
	r.failures = 0
	r.waits = 0
	r.retryAt = make(map[int64]time.Time)
	r.accountFailures = make(map[int64]int)
	return true
}
