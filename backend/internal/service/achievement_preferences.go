package service

import (
	"context"
	"fmt"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Zodiac is an optional entertainment preference, never a birthday or reward input.
func ValidateAchievementZodiac(zodiac string) error {
	switch zodiac {
	case "", "aries", "taurus", "gemini", "cancer", "leo", "virgo", "libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces":
		return nil
	default:
		return infraerrors.BadRequest("ACHIEVEMENT_PREFERENCE_INVALID", "请选择有效星座")
	}
}

type achievementPreferenceRepository interface {
	SaveAchievementZodiac(context.Context, int64, string) error
}

func (s *UserService) SaveAchievementZodiac(ctx context.Context, id int64, zodiac string) error {
	if err := ValidateAchievementZodiac(zodiac); err != nil {
		return err
	}
	r, ok := s.userRepo.(achievementPreferenceRepository)
	if !ok {
		return fmt.Errorf("achievement preference repository unavailable")
	}
	return r.SaveAchievementZodiac(ctx, id, zodiac)
}
