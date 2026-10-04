package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AchievementConfig struct {
	DailyRewards         []float64 `json:"daily_rewards,omitempty"`
	CashScope            string    `json:"cash_scope"`
	BudgetEnabled        bool      `json:"budget_enabled"`
	CashEnabled          bool      `json:"cash_enabled"`
	MilestoneCashEnabled bool      `json:"milestone_cash_enabled"`
	CashAllowlist        []int64   `json:"cash_allowlist"`
	DailyBudget          float64   `json:"daily_budget"`
	MonthlyBudget        float64   `json:"monthly_budget"`
}

func (c AchievementConfig) Validate() error {
	if len(c.DailyRewards) > 0 {
		if len(c.DailyRewards) != 6 {
			return infraerrors.BadRequest("VIP_DAILY_POLICY_INVALID", "请填写普通会员及五个VIP等级的签到金额")
		}
		for _, amount := range c.DailyRewards {
			if !finiteVIP(amount) || amount < 0 || amount > 1000 {
				return infraerrors.BadRequest("VIP_DAILY_POLICY_INVALID", "签到金额须在0至1000之间")
			}
		}
	}
	if math.IsNaN(c.DailyBudget) || math.IsInf(c.DailyBudget, 0) || math.IsNaN(c.MonthlyBudget) || math.IsInf(c.MonthlyBudget, 0) || c.DailyBudget < 0 || c.DailyBudget > 1000 || c.MonthlyBudget < c.DailyBudget || c.MonthlyBudget > 10000 || len(c.CashAllowlist) > 100 {
		return infraerrors.BadRequest("ACHIEVEMENT_CONFIG_INVALID", "试点每日预算 0–1000，月预算不少于日预算且不超过 10000，最多 100 个账户")
	}
	seen := map[int64]bool{}
	for _, id := range c.CashAllowlist {
		if id <= 0 || seen[id] {
			return infraerrors.BadRequest("ACHIEVEMENT_CONFIG_INVALID", "账户 ID 必须为不重复的正整数")
		}
		seen[id] = true
	}
	if c.CashScope != "" && c.CashScope != "all" && c.CashScope != "allowlist" {
		return infraerrors.BadRequest("ACHIEVEMENT_CONFIG_INVALID", "无效的奖励开放范围")
	}
	if c.CashEnabled && ((c.CashScope != "all" && len(c.CashAllowlist) == 0) || (c.BudgetEnabled && c.DailyBudget == 0)) {
		return infraerrors.BadRequest("ACHIEVEMENT_CONFIG_INVALID", "指定账户模式须填写名单，启用金额上限时须设置正数上限")
	}
	return nil
}

type AchievementRepository interface {
	AchievementSnapshot(context.Context, int64) (json.RawMessage, error)
	AchievementMutation(context.Context, int64, string, string, string, string) (json.RawMessage, error)
	AchievementConfig(context.Context) (*AchievementConfig, error)
	SaveAchievementConfig(context.Context, int64, AchievementConfig) error
}

func (s *UserService) achievementRepo() (AchievementRepository, error) {
	r, ok := s.userRepo.(AchievementRepository)
	if !ok {
		return nil, fmt.Errorf("achievement repository unavailable")
	}
	return r, nil
}
func (s *UserService) GetAchievements(ctx context.Context, id int64) (json.RawMessage, error) {
	r, e := s.achievementRepo()
	if e != nil {
		return nil, e
	}
	return r.AchievementSnapshot(ctx, id)
}
func (s *UserService) ChangeAchievement(ctx context.Context, id int64, action, key, date, idem string) (json.RawMessage, error) {
	if action == "checkin" {
		if _, e := time.Parse("2006-01-02", date); e != nil {
			return nil, infraerrors.BadRequest("ACHIEVEMENT_DATE_INVALID", "请刷新页面后签到")
		}
		if len(idem) < 8 || len(idem) > 80 || strings.TrimSpace(idem) != idem {
			return nil, infraerrors.BadRequest("ACHIEVEMENT_REQUEST_INVALID", "无效的请求编号")
		}
	}
	if len(key) > 20 {
		return nil, infraerrors.BadRequest("ACHIEVEMENT_KEY_INVALID", "无效的徽章编号")
	}
	r, e := s.achievementRepo()
	if e != nil {
		return nil, e
	}
	result, e := r.AchievementMutation(ctx, id, action, key, date, idem)
	if e == nil && (action == "checkin" || action == "claim" || action == "claim_series") {
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
		}
		if s.billingCache != nil {
			cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = s.billingCache.InvalidateUserBalance(cacheCtx, id)
		}
	}
	return result, e
}
func (s *UserService) GetAchievementConfig(ctx context.Context) (*AchievementConfig, error) {
	r, e := s.achievementRepo()
	if e != nil {
		return nil, e
	}
	return r.AchievementConfig(ctx)
}
func (s *UserService) SetAchievementConfig(ctx context.Context, actor int64, c AchievementConfig) error {
	if c.CashScope == "" {
		c.CashScope = "allowlist"
	}
	if c.CashAllowlist == nil {
		c.CashAllowlist = []int64{}
	}
	if e := c.Validate(); e != nil {
		return e
	}
	r, e := s.achievementRepo()
	if e != nil {
		return e
	}
	return r.SaveAchievementConfig(ctx, actor, c)
}
