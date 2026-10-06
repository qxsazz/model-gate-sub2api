package service

import "time"

// The platform day is fixed, independent of browser and server local timezones.
func CheckinDate(now time.Time) string {
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Format("2006-01-02")
}

// Only committed, charged text requests contribute. Providers normalize the
// four mutually exclusive token buckets before building the billing command.
func (c *UsageBillingCommand) AchievementTokens() int64 {
	if c == nil || (c.BalanceCost <= 0 && c.SubscriptionCost <= 0) || c.ImageCount > 0 || c.MediaType != "" || c.AchievementMedia {
		return 0
	}
	counts := []int{c.InputTokens, c.OutputTokens, c.CacheReadTokens, c.CacheCreationTokens}
	var total int64
	for _, v := range counts {
		if v < 0 {
			return 0
		}
		total += int64(v)
	}
	return total
}
