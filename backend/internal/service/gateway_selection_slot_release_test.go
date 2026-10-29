//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type gatewaySelectionSlotCache struct {
	ConcurrencyCache
	slots        map[string]int64
	releaseCalls int
	afterAcquire func()
}

func (c *gatewaySelectionSlotCache) AcquireAccountSlot(ctx context.Context, id int64, limit int, requestID string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if len(c.slots) >= limit {
		return false, nil
	}
	c.slots[requestID] = id
	if c.afterAcquire != nil {
		c.afterAcquire()
	}
	return true, nil
}

func (c *gatewaySelectionSlotCache) ReleaseAccountSlot(ctx context.Context, id int64, requestID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.releaseCalls++
	if c.slots[requestID] == id {
		delete(c.slots, requestID)
	}
	return nil
}

func (c *gatewaySelectionSlotCache) GetAccountsLoadBatch(_ context.Context, accounts []AccountWithConcurrency) (map[int64]*AccountLoadInfo, error) {
	loads := make(map[int64]*AccountLoadInfo, len(accounts))
	for _, account := range accounts {
		loads[account.ID] = &AccountLoadInfo{AccountID: account.ID}
	}
	return loads, nil
}

func TestGatewaySelectionSlotOwnershipDuringHydration(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformGemini} {
		for _, outcome := range []string{"missing_account", "canceled_after_acquire", "success"} {
			t.Run(platform+"/"+outcome, func(t *testing.T) {
				account := &Account{
					ID: 9401, Platform: platform, Type: AccountTypeAPIKey,
					Status: StatusActive, Schedulable: true, Concurrency: 1,
					Credentials: map[string]any{"api_key": "test-key"},
				}
				cache := &snapshotHydrationCache{snapshot: []*Account{account}, accounts: map[int64]*Account{}}
				if outcome != "missing_account" {
					cache.accounts[account.ID] = account
				}
				ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.ForcePlatform, platform))
				defer cancel()
				slots := &gatewaySelectionSlotCache{slots: make(map[string]int64)}
				if outcome == "canceled_after_acquire" {
					slots.afterAcquire = cancel
				}
				concurrency := NewConcurrencyService(slots)
				cfg := testConfig()
				cfg.Gateway.Scheduling.LoadBatchEnabled = true
				svc := &GatewayService{
					cfg: cfg, concurrencyService: concurrency,
					schedulerSnapshot: NewSchedulerSnapshotService(cache, nil, stubOpenAIAccountRepo{}, nil, nil),
				}
				model := "claude-sonnet-4-6"
				if platform == PlatformGemini {
					model = "gemini-3.5-flash"
				}

				selection, err := svc.SelectAccountWithLoadAwareness(ctx, nil, "", model, nil, "", 0)
				if outcome == "success" {
					require.NoError(t, err)
					require.NotNil(t, selection)
					require.True(t, selection.Acquired)
					require.Len(t, slots.slots, 1, "successful selection transfers slot ownership to the caller")
					require.Zero(t, slots.releaseCalls)
					selection.ReleaseFunc()
				} else {
					require.Error(t, err)
					require.Nil(t, selection)
					if outcome == "canceled_after_acquire" {
						require.ErrorIs(t, err, context.Canceled)
					}
				}
				require.Empty(t, slots.slots, "failed hydration or caller cleanup must return the acquired slot")
				require.Equal(t, 1, slots.releaseCalls)

				// Capacity must be immediately reusable, without waiting for Redis TTL.
				slots.afterAcquire = nil
				next, err := concurrency.AcquireAccountSlot(context.Background(), account.ID, account.Concurrency)
				require.NoError(t, err)
				require.True(t, next.Acquired)
				next.ReleaseFunc()
				require.Empty(t, slots.slots)
			})
		}
	}
}

func TestGatewaySelectionHydrationFailureWithoutOwnedSlot(t *testing.T) {
	svc := &GatewayService{schedulerSnapshot: NewSchedulerSnapshotService(&snapshotHydrationCache{}, nil, stubOpenAIAccountRepo{}, nil, nil)}
	releases := 0
	selection, err := svc.newSelectionResult(context.Background(), &Account{ID: 9402}, false, func() { releases++ }, &AccountWaitPlan{AccountID: 9402})
	require.Error(t, err)
	require.Nil(t, selection)
	require.Zero(t, releases, "a wait plan has not acquired a slot")

	selection, err = svc.newSelectionResult(context.Background(), &Account{ID: 9402}, true, nil, nil)
	require.Error(t, err)
	require.Nil(t, selection)
}
