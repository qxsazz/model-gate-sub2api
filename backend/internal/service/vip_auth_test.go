package service

import (
	"context"
	"testing"
)

type vipAuthTestRepository struct {
	UserRepository
	VIPRepository
	state    *VIPSnapshot
	original *User
}

func (r *vipAuthTestRepository) VIPSnapshot(context.Context, int64) (*VIPSnapshot, error) {
	return r.state, nil
}
func (r *vipAuthTestRepository) GetByID(context.Context, int64) (*User, error) {
	copy := *r.original
	return &copy, nil
}
func TestVIPAuthPreservesManualGrantsAndDoesNotMutateCache(t *testing.T) {
	base := &User{ID: 1, Concurrency: 5, AllowedGroups: []int64{21}}
	repo := &vipAuthTestRepository{original: base, state: &VIPSnapshot{Enabled: true, Concurrency: 8, Groups: []VIPGroupView{{ID: 32, Exclusive: true, Granted: true}}}}
	svc := &APIKeyService{userRepo: repo}
	key := &APIKey{UserID: 1, User: base}
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if key.User.Concurrency != 8 || !key.User.CanBindGroup(21, true) || !key.User.CanBindGroup(32, true) {
		t.Fatal("effective rights missing")
	}
	if base.Concurrency != 5 || base.CanBindGroup(32, true) {
		t.Fatal("shared auth cache was mutated")
	}
	repo.state.Groups = nil
	repo.state.Concurrency = 5
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if key.User.CanBindGroup(32, true) || !key.User.CanBindGroup(21, true) {
		t.Fatal("refund failed to revoke VIP or revoked private grant")
	}
}
