package service

import (
	"context"
	"errors"
	"testing"
)

type vipAuthTestRepository struct {
	UserRepository
	VIPRepository
	state    *VIPSnapshot
	original *User
	err      error
	calls    int
	rulesErr error
}

func (r *vipAuthTestRepository) VIPRules(context.Context) (VIPRules, error) {
	return VIPRules{Enabled: r.state != nil && r.state.Enabled}, r.rulesErr
}

func (r *vipAuthTestRepository) VIPAuthSnapshot(ctx context.Context, id int64) (*VIPSnapshot, error) {
	return r.VIPSnapshot(ctx, id)
}

func (r *vipAuthTestRepository) VIPSnapshot(context.Context, int64) (*VIPSnapshot, error) {
	r.calls++
	return r.state, r.err
}

func TestVIPDisabledAuthSkipsSnapshotFailure(t *testing.T) {
	repo := &vipAuthTestRepository{state: &VIPSnapshot{Enabled: false}, err: errors.New("snapshot unavailable")}
	svc := &APIKeyService{userRepo: repo}
	key := &APIKey{UserID: 1, User: &User{ID: 1, Concurrency: 5}}
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 0 {
		t.Fatal("disabled VIP queried the user snapshot")
	}
}

func TestVIPEnabledAuthFailureIsUnavailable(t *testing.T) {
	repo := &vipAuthTestRepository{state: &VIPSnapshot{Enabled: true}, err: errors.New("database unavailable")}
	svc := &APIKeyService{userRepo: repo}
	if err := svc.applyVIP(context.Background(), &APIKey{UserID: 1, User: &User{ID: 1}}); !errors.Is(err, ErrVIPUnavailable) {
		t.Fatalf("expected availability error, got %v", err)
	}
}

func TestVIPDisabledCacheIsBoundedAndInvalidatedOnConfigChange(t *testing.T) {
	base := &User{ID: 1, Concurrency: 5, AllowedGroups: []int64{21}}
	repo := &vipAuthTestRepository{state: &VIPSnapshot{Enabled: false}, original: base}
	svc := &APIKeyService{userRepo: repo}
	key := &APIKey{UserID: 1, User: base}
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	repo.rulesErr = errors.New("configuration query failed")
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal("fresh confirmed-disabled cache was ignored", err)
	}
	svc.InvalidateVIPConfig()
	if err := svc.applyVIP(context.Background(), key); !errors.Is(err, ErrVIPUnavailable) {
		t.Fatal("unknown config treated as disabled", err)
	}
	repo.rulesErr = nil
	repo.state = &VIPSnapshot{Enabled: true, Concurrency: 8, Groups: []VIPGroupView{{ID: 32, Rate: .2, Exclusive: true, Granted: true}}}
	if err := svc.applyVIP(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if key.User.Concurrency != 8 || !key.User.CanBindGroup(32, true) {
		t.Fatal("configuration invalidation did not activate rights")
	}
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
