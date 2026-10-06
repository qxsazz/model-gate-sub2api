//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

type refundBalanceCache struct {
	BillingCache
	check func(context.Context, int64)
}

func (c *refundBalanceCache) InvalidateUserBalance(ctx context.Context, id int64) error {
	c.check(ctx, id)
	return nil
}

func TestRefundInvalidatesCachesAfterCommittedSuccess(t *testing.T) {
	for _, async := range []bool{false, true} {
		t.Run(map[bool]string{false: "immediate", true: "pending confirmation"}[async], func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPendingRefundOrderForTest(t, ctx, client, "cache-success")
			calls := 0
			auth := &authCacheInvalidatorStub{}
			cache := &refundBalanceCache{check: func(cacheCtx context.Context, id int64) {
				require.NoError(t, cacheCtx.Err())
				require.Nil(t, dbent.TxFromContext(cacheCtx))
				require.Equal(t, order.UserID, id)
				current, err := client.PaymentOrder.Get(context.Background(), order.ID)
				require.NoError(t, err)
				require.Equal(t, OrderStatusRefunded, current.Status)
				calls++
			}}
			svc := &PaymentService{entClient: client, redeemService: &RedeemService{
				authCacheInvalidator: auth, billingCacheService: &BillingCacheService{cache: cache},
			}, userRepo: &mockUserRepo{deductAvailableBalanceFn: func(context.Context, int64, float64) (float64, error) { return 0, nil }}}
			plan := svc.refundFinalizePlan(order)
			var err error
			if async {
				_, err = svc.finalizePendingRefundSuccess(ctx, plan)
			} else {
				_, err = svc.markRefundOk(ctx, plan)
			}
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, []int64{order.UserID}, auth.userIDs)
		})
	}
}

func TestRefundDoesNotInvalidateUncommittedFailure(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "cache-failed")
	auth := &authCacheInvalidatorStub{}
	cache := &refundBalanceCache{check: func(context.Context, int64) { t.Fatal("cache invalidated before refund commit") }}
	svc := &PaymentService{entClient: client, redeemService: &RedeemService{
		authCacheInvalidator: auth, billingCacheService: &BillingCacheService{cache: cache},
	}, userRepo: &mockUserRepo{deductAvailableBalanceFn: func(context.Context, int64, float64) (float64, error) { return 0, errors.New("deduction failed") }}}
	_, err := svc.finalizePendingRefundSuccess(ctx, svc.refundFinalizePlan(order))
	require.Error(t, err)
	require.Empty(t, auth.userIDs)
}

func TestRefundRollbackInvalidatesRestoredBalance(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	order := createPendingRefundOrderForTest(t, ctx, client, "cache-rollback")
	auth := &authCacheInvalidatorStub{}
	calls := 0
	cache := &refundBalanceCache{check: func(context.Context, int64) { calls++ }}
	svc := &PaymentService{entClient: client, redeemService: &RedeemService{
		authCacheInvalidator: auth, billingCacheService: &BillingCacheService{cache: cache},
	}, userRepo: &mockUserRepo{}}
	plan := svc.refundFinalizePlan(order)
	plan.BalanceToDeduct = 10
	require.True(t, svc.RollbackRefund(ctx, plan, errors.New("provider failed")))
	require.Equal(t, 1, calls)
	require.Equal(t, []int64{order.UserID}, auth.userIDs)
}
