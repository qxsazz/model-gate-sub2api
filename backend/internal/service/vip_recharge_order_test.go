//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type vipRechargeOrderRepo struct {
	UserRepository
	VIPRepository
	rules VIPRules
	total float64
}

func (r *vipRechargeOrderRepo) VIPRules(context.Context) (VIPRules, error) { return r.rules, nil }
func (r *vipRechargeOrderRepo) VIPAuthSnapshot(context.Context, int64) (*VIPSnapshot, error) {
	return &VIPSnapshot{Enabled: r.rules.Enabled, Rules: r.rules, Total: r.total, BadgeLevel: 5}, nil
}

func TestVIPRechargeQuoteMatchesOrderAndOldSnapshotDoesNotChange(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	user, err := client.User.Create().SetEmail("vip-bonus-order@example.invalid").SetPasswordHash("fixture").Save(ctx)
	require.NoError(t, err)
	rules := DefaultVIPRules()
	rules.Enabled = true
	repo := &vipRechargeOrderRepo{rules: rules, total: 300}
	svc := &PaymentService{entClient: client, userRepo: repo}
	quote, err := svc.GetVIPRechargeQuote(ctx, int64(user.ID), 1.05)
	require.NoError(t, err)
	require.Equal(t, 2, quote.Level)
	require.Equal(t, 1.02, quote.Multiplier)
	order, err := svc.createOrderInTx(ctx, CreateOrderRequest{UserID: int64(user.ID), Amount: 100, OrderType: payment.OrderTypeBalance}, &User{ID: int64(user.ID)}, nil, &PaymentConfig{BalanceRechargeMultiplier: 1.05}, 105, 100, 0, 100, nil)
	require.NoError(t, err)
	require.Equal(t, 102.0, order.Amount)
	require.Equal(t, 100.0, order.ProviderSnapshot["vip_principal_usd"])
	require.Equal(t, 1.02, order.ProviderSnapshot["balance_recharge_multiplier"])
	require.Equal(t, 2.0, order.ProviderSnapshot["recharge_bonus_amount"])
	repo.total = 3000
	quote, err = svc.GetVIPRechargeQuote(ctx, int64(user.ID), 1.05)
	require.NoError(t, err)
	require.Equal(t, 1.05, quote.Multiplier)
	stored, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, 102.0, stored.Amount)
	require.Equal(t, 1.02, stored.ProviderSnapshot["balance_recharge_multiplier"])
}
