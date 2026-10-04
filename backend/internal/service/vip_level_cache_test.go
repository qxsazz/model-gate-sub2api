//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type vipLevelCacheRepo struct {
	UserRepository
	calls int
}

func (r *vipLevelCacheRepo) VIPSetLevel(context.Context, int64, int64, *VIPLevelCommand, string) error {
	r.calls++
	return nil
}
func TestVIPLevelChangeInvalidatesTargetAuth(t *testing.T) {
	repo := &vipLevelCacheRepo{}
	cache := &mockAuthCacheInvalidator{}
	svc := &UserService{userRepo: repo, authCacheInvalidator: cache}
	require.NoError(t, svc.SetVIPLevel(context.Background(), 1, 7, VIPLevelCommand{Level: 3, Reason: "设置完整权益"}))
	require.NoError(t, svc.RestoreVIPLevel(context.Background(), 1, 7, "恢复自动权益"))
	require.Equal(t, []int64{7, 7}, cache.invalidatedUserIDs)
	require.Equal(t, 2, repo.calls)
}
