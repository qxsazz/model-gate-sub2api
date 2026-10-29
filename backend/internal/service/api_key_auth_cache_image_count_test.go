package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthSnapshotImageCountBillingPreservesMGPolicy(t *testing.T) {
	svc := &APIKeyService{}
	key := profitAuthTestAPIKey()
	key.Group.AllowImageGeneration = true
	key.Group.ImageCountBilling = true
	key.Group.ForceOpenAIFast = true
	key.Group.FreeOpenAIFast = true
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var cached APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &cached))
	materialized, used, err := svc.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.True(t, used)
	require.True(t, imageCountBillingApplies(materialized, true))
	require.False(t, imageCountBillingApplies(materialized, false))
	require.True(t, materialized.Group.ProfitControlEnabled)
	require.Equal(t, key.Group.RateMultiplier, materialized.Group.RateMultiplier)
	require.True(t, materialized.Group.ForceOpenAIFast)
	require.True(t, materialized.Group.FreeOpenAIFast)

	// MG 0.2.18 used v25 for a different policy change. Reusing it would
	// silently disable image billing until an old Redis entry expired.
	cached.Snapshot.Version = 25
	cached.Snapshot.Group.ImageCountBilling = false
	materialized, used, err = svc.applyAuthCacheEntry(key.Key, &cached)
	require.NoError(t, err)
	require.False(t, used)
	require.Nil(t, materialized)
}
