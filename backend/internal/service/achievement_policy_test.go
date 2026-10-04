package service

import (
	"testing"
	"time"
)

func TestCheckinDayUsesShanghaiBoundary(t *testing.T) {
	now := time.Date(2026, 10, 3, 16, 0, 0, 0, time.UTC)
	if got := CheckinDate(now); got != "2026-10-04" {
		t.Fatalf("got %s", got)
	}
	if got := CheckinDate(now.Add(-time.Second)); got != "2026-10-03" {
		t.Fatalf("got %s", got)
	}
}

func TestTrustedGrowthExcludesUnbilledAndMedia(t *testing.T) {
	c := UsageBillingCommand{BalanceCost: .01, InputTokens: 100, OutputTokens: 20, CacheReadTokens: 30, CacheCreationTokens: 40}
	if got := c.AchievementTokens(); got != 190 {
		t.Fatalf("got %d", got)
	}
	c.MediaType = "image"
	if c.AchievementTokens() != 0 {
		t.Fatal("image counted")
	}
	c.MediaType = ""
	c.BalanceCost = 0
	c.APIKeyRateLimitCost = 1
	if c.AchievementTokens() != 0 {
		t.Fatal("rate-only simple request counted")
	}
	c.SubscriptionCost = .1
	if c.AchievementTokens() != 190 {
		t.Fatal("subscription excluded")
	}
	c.InputTokens = -1
	if c.AchievementTokens() != 0 {
		t.Fatal("invalid tokens counted")
	}
}
