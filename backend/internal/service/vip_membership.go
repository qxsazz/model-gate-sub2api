package service

import (
	"context"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"net/http"
	"strings"
)

var (
	ErrVIPRewardAccountUnavailable = infraerrors.New(http.StatusForbidden, "VIP_REWARD_ACCOUNT_UNAVAILABLE", "账号已停用或不可用，无法领取奖励。")
	ErrVIPRewardDisabled           = infraerrors.New(http.StatusConflict, "VIP_REWARD_DISABLED", "VIP 权益尚未启用，暂不能领取奖励。")
	ErrVIPRewardInvalidLevel       = infraerrors.New(http.StatusBadRequest, "VIP_REWARD_INVALID_LEVEL", "奖励档位无效，请刷新会员中心。")
	ErrVIPRewardThreshold          = infraerrors.New(http.StatusBadRequest, "VIP_REWARD_THRESHOLD_NOT_REACHED", "累计有效充值未达到该档奖励门槛。")
	ErrVIPRewardConfig             = infraerrors.New(http.StatusInternalServerError, "VIP_REWARD_CONFIG_INVALID", "奖励配置异常，请联系管理员。")
	ErrVIPRewardUnavailable        = infraerrors.New(http.StatusInternalServerError, "VIP_REWARD_UNAVAILABLE", "奖励服务暂时不可用，请稍后重试。")
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
		return 0, ErrVIPRewardInvalidLevel
	}
	r, ok := s.userRepo.(VIPMembershipRepository)
	if !ok {
		return 0, ErrVIPRewardUnavailable
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
