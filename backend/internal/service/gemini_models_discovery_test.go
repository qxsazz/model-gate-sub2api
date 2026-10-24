//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type discoveryBoundedUpstream struct {
	HTTPUpstream
	active, peak, calls atomic.Int32
	blocked             bool
}

func (u *discoveryBoundedUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls.Add(1)
	active := u.active.Add(1)
	defer u.active.Add(-1)
	for peak := u.peak.Load(); active > peak; peak = u.peak.Load() {
		if u.peak.CompareAndSwap(peak, active) {
			break
		}
	}
	if u.blocked {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	select {
	case <-req.Context().Done():
		return nil, req.Context().Err()
	case <-time.After(5 * time.Millisecond):
	}
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"models":[]}`))}, nil
}

func TestGeminiModelsDiscoveryBoundsFanoutAndHonorsCancellation(t *testing.T) {
	accounts := make([]Account, 15)
	for i := range accounts {
		accounts[i] = Account{ID: int64(i + 1), Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}
	}
	groupID := int64(26)
	upstream := &discoveryBoundedUpstream{}
	svc := newGeminiModelsEndpointsTestService(accounts, upstream)
	results, err := svc.ReadModelsLists(context.Background(), &groupID)
	require.NoError(t, err)
	require.Len(t, results, 12)
	require.Equal(t, int32(12), upstream.calls.Load())
	require.LessOrEqual(t, upstream.peak.Load(), int32(3))
	for _, result := range results {
		require.NoError(t, result.Err)
		require.Equal(t, 200, result.Response.StatusCode)
	}
	upstream.blocked = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	results, err = svc.ReadModelsLists(ctx, &groupID)
	require.NoError(t, err)
	for _, result := range results {
		require.ErrorIs(t, result.Err, context.Canceled)
		require.Nil(t, result.Response)
	}
}
