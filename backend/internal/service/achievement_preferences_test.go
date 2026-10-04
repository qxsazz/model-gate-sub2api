package service

import "testing"

func TestAchievementZodiacValidation(t *testing.T) {
	for _, key := range []string{"", "aries", "taurus", "gemini", "cancer", "leo", "virgo", "libra", "scorpio", "sagittarius", "capricorn", "aquarius", "pisces"} {
		if err := ValidateAchievementZodiac(key); err != nil {
			t.Fatal(key, err)
		}
	}
	for _, key := range []string{"Libra", "unknown", "libra ", "<script>"} {
		if ValidateAchievementZodiac(key) == nil {
			t.Fatal("invalid preference accepted", key)
		}
	}
}
