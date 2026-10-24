package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayRecoveryDefaultsAndDisable(t *testing.T) {
	resetViperWithJWTSecret(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.Gateway.Recovery.Enabled)
	require.Equal(t, 300, cfg.Gateway.Recovery.BudgetSeconds)
	require.Equal(t, 120, cfg.Gateway.Recovery.SelectionWaitSeconds)
	require.Equal(t, 12, cfg.Gateway.Recovery.MaxAttempts)
	t.Setenv("GATEWAY_RECOVERY_ENABLED", "false")
	cfg, err = Load()
	require.NoError(t, err)
	require.False(t, cfg.Gateway.Recovery.Enabled)
}

func TestGatewayRecoveryRejectsUnboundedConfiguration(t *testing.T) {
	for _, mutate := range []func(*GatewayRecoveryConfig){
		func(r *GatewayRecoveryConfig) { r.BudgetSeconds = 0 },
		func(r *GatewayRecoveryConfig) { r.SelectionWaitSeconds = r.BudgetSeconds + 1 },
		func(r *GatewayRecoveryConfig) { r.MaxAttempts = 0 },
		func(r *GatewayRecoveryConfig) { r.MaxAttempts = 31 },
		func(r *GatewayRecoveryConfig) { r.MaxWaiters = 0 },
		func(r *GatewayRecoveryConfig) { r.MaxWaitersPerGroup = 0 },
		func(r *GatewayRecoveryConfig) { r.MaxWaitersPerUser = 0 },
	} {
		resetViperWithJWTSecret(t)
		cfg, err := Load()
		require.NoError(t, err)
		mutate(&cfg.Gateway.Recovery)
		require.ErrorContains(t, cfg.Validate(), "gateway.recovery")
	}
}
