package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"math"
	"testing"
)

func TestCheckoutVIPGrowthRatesExposeOnlyOfferedCurrencyQuotes(t *testing.T) {
	rates := map[string]float64{"USD": 1, "CNY": 1, "SECRET-CURRENCY": 10, "EUR": math.Inf(1)}
	methods := map[string]service.MethodLimits{"alipay": {Currency: "CNY"}, "card": {Currency: "EUR"}}
	require.Equal(t, map[string]float64{"CNY": 1}, checkoutVIPGrowthRates(rates, methods))
	require.Empty(t, checkoutVIPGrowthRates(nil, methods))
}
