package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/google/uuid"
)

type AchievementCardCommand struct {
	Date          string   `json:"date"`
	RequestKey    string   `json:"request_key"`
	ExpectedGross *float64 `json:"expected_gross"`
	ExpectedTier  *int     `json:"expected_tier"`
}

func (c AchievementCardCommand) Validate() error {
	if _, err := time.Parse("2006-01-02", c.Date); err != nil {
		return infraerrors.BadRequest("ACHIEVEMENT_CARD_DATE_INVALID", "无效的补签日期")
	}
	if _, err := uuid.Parse(c.RequestKey); err != nil {
		return infraerrors.BadRequest("ACHIEVEMENT_REQUEST_INVALID", "无效的请求编号")
	}
	if c.ExpectedGross == nil || c.ExpectedTier == nil || math.IsNaN(*c.ExpectedGross) || math.IsInf(*c.ExpectedGross, 0) || *c.ExpectedGross < 0 || *c.ExpectedTier < 0 || *c.ExpectedTier > 5 {
		return infraerrors.BadRequest("ACHIEVEMENT_HISTORY_CHANGED", "请先核验补签日的历史权益")
	}
	return nil
}

type AchievementCardRepository interface {
	AchievementCardPreview(context.Context, int64, string) (json.RawMessage, error)
	UseAchievementCard(context.Context, int64, AchievementCardCommand) (json.RawMessage, error)
}

func (s *UserService) PreviewAchievementCard(ctx context.Context, id int64, date string) (json.RawMessage, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, infraerrors.BadRequest("ACHIEVEMENT_CARD_DATE_INVALID", "无效的补签日期")
	}
	r, ok := s.userRepo.(AchievementCardRepository)
	if !ok {
		return nil, fmt.Errorf("achievement cards unavailable")
	}
	return r.AchievementCardPreview(ctx, id, date)
}
func (s *UserService) UseAchievementCard(ctx context.Context, id int64, c AchievementCardCommand) (json.RawMessage, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	r, ok := s.userRepo.(AchievementCardRepository)
	if !ok {
		return nil, fmt.Errorf("achievement cards unavailable")
	}
	result, err := r.UseAchievementCard(ctx, id, c)
	if err == nil {
		if s.authCacheInvalidator != nil {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
		}
		if s.billingCache != nil {
			cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_ = s.billingCache.InvalidateUserBalance(cacheCtx, id)
		}
	}
	return result, err
}
