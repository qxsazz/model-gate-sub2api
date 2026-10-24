//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type pendingRefreshDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c pendingRefreshDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }

func TestOAuthRefreshRejectsElapsedDeadlineBeforeTimerSignals(t *testing.T) {
	ctx := pendingRefreshDeadlineContext{Context: context.Background(), deadline: time.Now().Add(-time.Hour)}
	require.NoError(t, ctx.Err(), "simulate a delayed context timer")
	require.ErrorIs(t, oauthRefreshContextError(ctx), context.DeadlineExceeded)
	future := pendingRefreshDeadlineContext{Context: context.Background(), deadline: time.Now().Add(time.Hour)}
	require.NoError(t, oauthRefreshContextError(future))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, oauthRefreshContextError(canceled), context.Canceled, "parent cancellation retains priority")
}
