package service

import (
	"fmt"
	"github.com/shopspring/decimal"
	"math"
)

type VIPTier struct {
	Level                int     `json:"level"`
	Threshold            float64 `json:"threshold"`
	Concurrency          int     `json:"concurrency"`
	RPM                  int     `json:"rpm"`
	RebatePercent        float64 `json:"rebate_percent"`
	RechargeBonusPercent float64 `json:"recharge_bonus_percent"`
}
type VIPGroupRule struct {
	GroupID   int64     `json:"group_id"`
	Access    bool      `json:"access"`
	Private   bool      `json:"private"`
	Floor     float64   `json:"floor"`
	Discounts []float64 `json:"discounts"`
}
type VIPRules struct {
	RechargeBonusEnabled bool               `json:"recharge_bonus_enabled"`
	ExchangeRates        map[string]float64 `json:"exchange_rates"`
	Enabled              bool               `json:"enabled"`
	Currency             string             `json:"currency"`
	AccessThreshold      float64            `json:"access_threshold"`
	Tiers                []VIPTier          `json:"tiers"`
	Groups               []VIPGroupRule     `json:"groups"`
}

func DefaultVIPRules() VIPRules {
	return VIPRules{RechargeBonusEnabled: true, Currency: "USD", ExchangeRates: map[string]float64{"USD": 1, "CNY": 1}, AccessThreshold: 100, Groups: []VIPGroupRule{}, Tiers: []VIPTier{
		{1, 100, 8, 0, 2, 1}, {2, 300, 12, 0, 4, 2}, {3, 600, 16, 0, 6, 3}, {4, 1500, 20, 0, 8, 4}, {5, 3000, 30, 0, 10, 5},
	}}
}
func (r VIPRules) Validate() error {
	for currency, rate := range r.ExchangeRates {
		if len(currency) != 3 || !finiteVIP(rate) || rate <= 0 {
			return fmt.Errorf("invalid exchange rate")
		}
	}
	if r.Currency != "USD" || !finiteVIP(r.AccessThreshold) || r.AccessThreshold < 100 || len(r.Tiers) != 5 {
		return fmt.Errorf("invalid currency, access threshold or tier count")
	}
	previous := 0.0
	for i, t := range r.Tiers {
		if !finiteVIP(t.RechargeBonusPercent) || t.RechargeBonusPercent < 0 || t.RechargeBonusPercent > 5 || (i > 0 && t.RechargeBonusPercent < r.Tiers[i-1].RechargeBonusPercent) {
			return fmt.Errorf("invalid recharge bonus tier %d", i+1)
		}
		if t.Level != i+1 || !finiteVIP(t.Threshold) || t.Threshold <= previous || (i == 0 && t.Threshold != 100) || t.Concurrency < 1 || t.Concurrency > 1000 || t.RPM < 0 || !finiteVIP(t.RebatePercent) || t.RebatePercent < 0 || t.RebatePercent > 10 {
			return fmt.Errorf("invalid tier %d", i+1)
		}
		if i > 0 && (t.RebatePercent < r.Tiers[i-1].RebatePercent || t.Concurrency < r.Tiers[i-1].Concurrency) {
			return fmt.Errorf("tier benefits must increase")
		}
		previous = t.Threshold
	}
	seen := map[int64]bool{}
	for _, g := range r.Groups {
		if g.GroupID <= 0 || seen[g.GroupID] || !finiteVIP(g.Floor) || g.Floor < 0 || len(g.Discounts) != 5 {
			return fmt.Errorf("invalid group rule")
		}
		seen[g.GroupID] = true
		if g.Private && g.Access {
			return fmt.Errorf("private groups cannot be VIP groups")
		}
		prev := 0.0
		for _, d := range g.Discounts {
			if !finiteVIP(d) || d < prev || d > 0.075 {
				return fmt.Errorf("invalid group discount")
			}
			prev = d
		}
		if g.Access {
			for _, d := range g.Discounts {
				if d != 0 {
					return fmt.Errorf("exclusive groups cannot stack discounts")
				}
			}
		}
	}
	return nil
}
func finiteVIP(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (r VIPRules) Tier(total float64) VIPTier {
	var result VIPTier
	if !r.Enabled {
		return result
	}
	for _, t := range r.Tiers {
		if decimal.NewFromFloat(total).GreaterThanOrEqual(decimal.NewFromFloat(t.Threshold)) {
			result = t
		}
	}
	return result
}
func VIPDiscountedRate(base, floor, discount float64) float64 {
	discount = math.Min(discount, .075)
	if floor > base {
		floor = base
	}
	v := decimal.NewFromFloat(base).Sub(decimal.NewFromFloat(discount))
	return decimal.Max(decimal.NewFromFloat(floor), v).InexactFloat64()
}

func (r VIPRules) RechargeMultiplier(total, legacy float64) (float64, error) {
	if !r.Enabled || !r.RechargeBonusEnabled {
		return legacy, nil
	}
	if !finiteVIP(total) {
		return 0, ErrVIPRateUnavailable
	}
	bonus := r.Tier(total).RechargeBonusPercent
	if !finiteVIP(bonus) || bonus < 0 || bonus > 5 {
		return 0, ErrVIPRateUnavailable
	}
	return decimal.NewFromInt(1).Add(decimal.NewFromFloat(bonus).Div(decimal.NewFromInt(100))).InexactFloat64(), nil
}
