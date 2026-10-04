package service

import (
	"math"
	"testing"
)

func TestAchievementCardCommandRequiresVerifiedHistory(t *testing.T) {
	amount := .05
	tier := 1
	good := AchievementCardCommand{Date: "2026-10-02", RequestKey: "9049586f-2ea3-4931-9106-25d13e1224be", ExpectedGross: &amount, ExpectedTier: &tier}
	if err := good.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*AchievementCardCommand){
		func(c *AchievementCardCommand) { c.ExpectedGross = nil },
		func(c *AchievementCardCommand) { c.ExpectedTier = nil },
		func(c *AchievementCardCommand) { v := math.NaN(); c.ExpectedGross = &v },
		func(c *AchievementCardCommand) { v := -1.; c.ExpectedGross = &v },
		func(c *AchievementCardCommand) { v := 6; c.ExpectedTier = &v },
		func(c *AchievementCardCommand) { c.Date = "2026-02-30" },
		func(c *AchievementCardCommand) { c.RequestKey = "bad" },
	} {
		bad := good
		mutate(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid history command accepted")
		}
	}
}
