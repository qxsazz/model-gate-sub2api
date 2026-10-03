package service

import "testing"

func TestAchievementImmediateRulesAllowAllWithoutBudgetQueue(t *testing.T) {
	c := AchievementConfig{CashEnabled: true, MilestoneCashEnabled: true, CashScope: "all", BudgetEnabled: false, DailyBudget: 0, MonthlyBudget: 0}
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	c.CashScope = "allowlist"
	if c.Validate() == nil {
		t.Fatal("empty allowlist accepted")
	}
	c.CashAllowlist = []int64{1}
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	c.BudgetEnabled = true
	if c.Validate() == nil {
		t.Fatal("enabled zero budget accepted")
	}
	c.DailyBudget = 10
	c.MonthlyBudget = 5
	if c.Validate() == nil {
		t.Fatal("monthly less than daily")
	}
}
func TestAchievementAdminMutationValidation(t *testing.T) {
	good := AchievementAdminCommand{Key: "T01", Reason: "核验后授予", RequestKey: "9049586f-2ea3-4931-9106-25d13e1224be"}
	if e := good.Validate("grant"); e != nil {
		t.Fatal(e)
	}
	bad := good
	bad.Reason = " "
	if bad.Validate("grant") == nil {
		t.Fatal("missing reason")
	}
	bad = good
	bad.RequestKey = ""
	if bad.Validate("grant") == nil {
		t.Fatal("missing idempotency")
	}
	bad = good
	bad.Date = "2026-02-30"
	if bad.Validate("backfill") == nil {
		t.Fatal("invalid date")
	}
	bad = good
	bad.GrantReward = true
	if bad.Validate("revoke") == nil {
		t.Fatal("grant flag on revoke")
	}
	if good.Validate("unknown") == nil {
		t.Fatal("unknown action")
	}
}
