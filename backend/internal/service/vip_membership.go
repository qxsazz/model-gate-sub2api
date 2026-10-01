package service

import (
	"context"
	"fmt"
	"strings"
)

type VIPReward struct {
	Level     int     `json:"level"`
	Threshold float64 `json:"threshold"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
}
type VIPHonorSeat struct {
	Name  string `json:"name"`
	Level int    `json:"level"`
}
type VIPMembership struct {
	Rewards           []VIPReward       `json:"rewards"`
	Seats             []VIPHonorSeat    `json:"seats"`
	Debt              float64           `json:"debt"`
	Claimed           float64           `json:"claimed"`
	DiscountSummaries map[int][]float64 `json:"discount_summaries"`
}
type VIPMembershipRepository interface {
	VIPMembership(context.Context, int64) (*VIPMembership, error)
	VIPClaimReward(context.Context, int64, int) (float64, error)
}

// Never publish full usernames or any part of an email domain.
func VIPPrivateName(name string) string {
	name = strings.TrimSpace(strings.SplitN(name, "@", 2)[0])
	r := []rune(name)
	if len(r) == 0 {
		return "会员***"
	}
	n := 2
	if len(r) <= 2 {
		n = 1
	}
	return string(r[:n]) + "***"
}
func (s *UserService) GetVIPMembership(ctx context.Context, id int64) (*VIPMembership, error) {
	r, ok := s.userRepo.(VIPMembershipRepository)
	if !ok {
		return nil, fmt.Errorf("VIP membership unavailable")
	}
	return r.VIPMembership(ctx, id)
}
func (s *UserService) ClaimVIPReward(ctx context.Context, id int64, level int) (float64, error) {
	if level < 1 || level > 5 {
		return 0, fmt.Errorf("invalid milestone")
	}
	r, ok := s.userRepo.(VIPMembershipRepository)
	if !ok {
		return 0, fmt.Errorf("VIP membership unavailable")
	}
	amount, err := r.VIPClaimReward(ctx, id, level)
	if err == nil {
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
		}
		if s.billingCache != nil {
			_ = s.billingCache.InvalidateUserBalance(ctx, id)
		}
	}
	return amount, err
}
