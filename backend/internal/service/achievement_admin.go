package service

import (
	"context"
	"encoding/json"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
	"math"
	"strings"
	"time"
)

type AchievementAdminCommand struct {
	Key           string   `json:"key"`
	Date          string   `json:"date"`
	Reason        string   `json:"reason"`
	RequestKey    string   `json:"request_key"`
	GrantReward   bool     `json:"grant_reward"`
	ReclaimReward bool     `json:"reclaim_reward"`
	ExpectedGross *float64 `json:"expected_gross,omitempty"`
	ExpectedTier  *int     `json:"expected_tier,omitempty"`
}

func (c AchievementAdminCommand) Validate(action string) error {
	if strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 1500 {
		return infraerrors.BadRequest("ACHIEVEMENT_ADMIN_REASON", "请填写操作原因，最多 500 个汉字")
	}
	if _, e := uuid.Parse(c.RequestKey); e != nil {
		return infraerrors.BadRequest("ACHIEVEMENT_ADMIN_REQUEST", "无效的请求编号")
	}
	switch action {
	case "backfill":
		if _, e := time.Parse("2006-01-02", c.Date); e != nil || c.GrantReward || c.ReclaimReward || c.ExpectedGross == nil || c.ExpectedTier == nil || *c.ExpectedGross < 0 || math.IsNaN(*c.ExpectedGross) || math.IsInf(*c.ExpectedGross, 0) || *c.ExpectedTier < 0 || *c.ExpectedTier > 5 {
			return infraerrors.BadRequest("ACHIEVEMENT_ADMIN_DATE", "无效的补签日期或选项")
		}
	case "grant", "revoke", "restore":
		if c.Key == "" || len(c.Key) > 20 || c.Date != "" || (action != "grant" && c.GrantReward) || (action != "revoke" && c.ReclaimReward) {
			return infraerrors.BadRequest("ACHIEVEMENT_ADMIN_ACTION", "无效的徽章或操作选项")
		}
	default:
		return infraerrors.BadRequest("ACHIEVEMENT_ADMIN_ACTION", "操作不存在")
	}
	return nil
}

type AchievementAdminRepository interface {
	AdminAchievementSnapshot(context.Context, int64) (json.RawMessage, error)
	AchievementBackfillPreview(context.Context, int64, string) (json.RawMessage, error)
	AdminAchievementMutation(context.Context, int64, int64, string, AchievementAdminCommand) (json.RawMessage, error)
	AchievementAudit(context.Context) (json.RawMessage, error)
}

func (s *UserService) achievementAdminRepo() (AchievementAdminRepository, error) {
	r, ok := s.userRepo.(AchievementAdminRepository)
	if !ok {
		return nil, fmt.Errorf("achievement administration unavailable")
	}
	return r, nil
}
func (s *UserService) GetAdminAchievements(ctx context.Context, id int64) (json.RawMessage, error) {
	r, e := s.achievementAdminRepo()
	if e != nil {
		return nil, e
	}
	return r.AdminAchievementSnapshot(ctx, id)
}
func (s *UserService) PreviewAchievementBackfill(ctx context.Context, id int64, date string) (json.RawMessage, error) {
	if _, e := time.Parse("2006-01-02", date); e != nil {
		return nil, infraerrors.BadRequest("ACHIEVEMENT_ADMIN_DATE", "无效的补签日期")
	}
	r, e := s.achievementAdminRepo()
	if e != nil {
		return nil, e
	}
	return r.AchievementBackfillPreview(ctx, id, date)
}
func (s *UserService) ControlAdminAchievement(ctx context.Context, actor, id int64, action string, c AchievementAdminCommand) (json.RawMessage, error) {
	if e := c.Validate(action); e != nil {
		return nil, e
	}
	r, e := s.achievementAdminRepo()
	if e != nil {
		return nil, e
	}
	result, e := r.AdminAchievementMutation(ctx, actor, id, action, c)
	if e == nil {
		s.invalidateAchievementBalanceCaches(ctx, id)
	}
	return result, e
}
func (s *UserService) GetAchievementAudit(ctx context.Context) (json.RawMessage, error) {
	r, e := s.achievementAdminRepo()
	if e != nil {
		return nil, e
	}
	return r.AchievementAudit(ctx)
}
