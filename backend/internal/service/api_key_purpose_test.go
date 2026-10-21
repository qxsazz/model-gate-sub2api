//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type purposeUserRepository struct {
	UserRepository
	VIPRepository
	user  User
	state VIPSnapshot
}

func (r *purposeUserRepository) GetByID(context.Context, int64) (*User, error) {
	copy := r.user
	copy.AllowedGroups = append([]int64{}, r.user.AllowedGroups...)
	return &copy, nil
}
func (r *purposeUserRepository) VIPSnapshot(context.Context, int64) (*VIPSnapshot, error) {
	return &r.state, nil
}

type purposeGroupRepository struct {
	GroupRepository
	groups []Group
}

func (r *purposeGroupRepository) ListActive(context.Context) ([]Group, error) { return r.groups, nil }
func (r *purposeGroupRepository) GetByID(_ context.Context, id int64) (*Group, error) {
	for i := range r.groups {
		if r.groups[i].ID == id {
			return &r.groups[i], nil
		}
	}
	return nil, ErrGroupNotFound
}

type purposeSubscriptionRepository struct{ UserSubscriptionRepository }

func (*purposeSubscriptionRepository) ListActiveByUserID(context.Context, int64) ([]UserSubscription, error) {
	return nil, nil
}
func TestKeyPurposeDoesNotBypassExclusiveGroupGrant(t *testing.T) {
	group := Group{ID: 13, Status: "active", Platform: PlatformOpenAI, IsExclusive: true, SubscriptionType: "standard", AllowImageGeneration: true}
	for _, tc := range []struct {
		name   string
		manual bool
		vip    bool
	}{{"ordinary", false, false}, {"manual", true, false}, {"recharge eligible", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &purposeUserRepository{user: User{ID: 1}, state: VIPSnapshot{Enabled: true, Groups: []VIPGroupView{{ID: 13, Exclusive: true, Granted: tc.vip}}}}
			if tc.manual {
				repo.user.AllowedGroups = []int64{13}
			}
			svc := &APIKeyService{userRepo: repo, groupRepo: &purposeGroupRepository{groups: []Group{group}}, userSubRepo: &purposeSubscriptionRepository{}}
			available, err := svc.GetAvailableGroups(context.Background(), 1)
			require.NoError(t, err)
			if tc.manual || tc.vip {
				require.Len(t, available, 1)
				require.True(t, svc.canUserBindGroup(context.Background(), &repo.user, &group))
			} else {
				require.Empty(t, available)
				_, err = svc.Create(context.Background(), 1, CreateAPIKeyRequest{Name: "forged", GroupID: &group.ID, Purpose: "image"})
				require.ErrorIs(t, err, ErrGroupNotAllowed)
			}
		})
	}
}
func TestCreateKeyRejectsWrongPurposeBeforeCreatingCredential(t *testing.T) {
	group := Group{ID: 7, Status: "active", Platform: PlatformGrok, SubscriptionType: "standard", AllowImageGeneration: false}
	svc := &APIKeyService{userRepo: &purposeUserRepository{user: User{ID: 1}}, groupRepo: &purposeGroupRepository{groups: []Group{group}}}
	for _, purpose := range []string{"image", "video", "invalid"} {
		_, err := svc.Create(context.Background(), 1, CreateAPIKeyRequest{Name: "forged", GroupID: &group.ID, Purpose: purpose})
		require.Error(t, err)
		require.Contains(t, err.Error(), "用途")
	}
}
