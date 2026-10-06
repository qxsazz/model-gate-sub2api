package service

import (
	"net/http"
	"strings"

	apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// principal is the recharge amount before balance bonuses and payment fees.
func applyVIPPaymentSnapshot(snapshot map[string]any, currency string, principal float64, rules VIPRules) (map[string]any, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	rate, configured := rules.ExchangeRates[currency]
	if !configured {
		if !rules.Enabled {
			return snapshot, nil
		}
		return nil, apperrors.New(http.StatusServiceUnavailable, "VIP_EXCHANGE_RATE_NOT_CONFIGURED", "VIP 充值成长换算尚未配置，请联系管理员").WithMetadata(map[string]string{"currency": currency})
	}
	if !finiteVIP(rate) || rate <= 0 {
		return nil, apperrors.New(http.StatusServiceUnavailable, "VIP_EXCHANGE_RATE_INVALID", "VIP 充值成长换算配置无效，请联系管理员").WithMetadata(map[string]string{"currency": currency})
	}
	if !finiteVIP(principal) || principal < 0 {
		return nil, apperrors.BadRequest("VIP_RECHARGE_AMOUNT_INVALID", "充值成长本金无效")
	}
	amount := decimal.NewFromFloat(principal).Mul(decimal.NewFromFloat(rate)).Round(8)
	if snapshot == nil {
		snapshot = map[string]any{}
	}
	snapshot["vip_principal_usd"] = amount.InexactFloat64()
	snapshot["vip_fx"] = rate
	snapshot["vip_currency"] = currency
	return snapshot, nil
}
