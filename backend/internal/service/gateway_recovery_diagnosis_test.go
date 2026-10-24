//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayRecoveryDiagnosisDistinguishesCapacityFromPermanentFailures(t *testing.T) {
	until := time.Now().Add(time.Minute)
	expired := time.Now().Add(-time.Minute)
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, OverloadUntil: &until},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, RateLimitResetAt: &until},
		{ID: 3, Platform: PlatformOpenAI, Status: StatusError, Schedulable: true},
		{ID: 4, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, AutoPauseOnExpired: true, ExpiresAt: &expired},
		{ID: 5, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Type: AccountTypeAPIKey, Extra: map[string]any{"quota_limit": 10.0, "quota_used": 10.0}},
		{ID: 6, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": map[string]any{"other-model": "other-model"}}},
	}
	repo := &mockAccountRepoForPlatform{accounts: accounts, accountsByID: map[int64]*Account{}}
	for i := range repo.accounts {
		repo.accountsByID[repo.accounts[i].ID] = &repo.accounts[i]
	}
	gateway := &GatewayService{accountRepo: repo, cfg: testConfig()}
	d := gateway.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "requested-model", PlatformOpenAI)
	require.True(t, d.HasModelSupport, "expired/quota state must not be presented as an unsupported model")
	require.ElementsMatch(t, []int64{1, 2}, d.RecoveryAccountIDs)

	openai := &OpenAIGatewayService{accountRepo: repo, cfg: testConfig()}
	d = openai.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "requested-model", PlatformOpenAI)
	require.ElementsMatch(t, []int64{1, 2}, d.RecoveryAccountIDs)
}

func TestGatewayRecoveryUnknownDiagnosisNeverQueues(t *testing.T) {
	var generic *GatewayService
	var openai *OpenAIGatewayService
	for _, d := range []ModelAvailabilityDiagnosis{
		generic.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "model", PlatformOpenAI),
		openai.DiagnoseModelAvailabilityForPlatform(context.Background(), nil, "model", PlatformOpenAI),
	} {
		require.True(t, d.HasModelSupport)
		require.Empty(t, d.RecoveryAccountIDs)
	}
}
