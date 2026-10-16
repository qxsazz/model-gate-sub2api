package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestModelGateGeminiThinkingKeepsLegacyBudgetAndSummaryContract(t *testing.T) {
	for _, tc := range []struct {
		budget int
		body   string
		level  string
	}{
		{1024, `{"thinking":{"type":"enabled","budget_tokens":1024}}`, "minimal"},
		{4096, `{"thinking":{"type":"enabled","budget_tokens":4096}}`, "low"},
		{8000, `{"thinking":{"type":"enabled","budget_tokens":8000}}`, "medium"},
		{24576, `{"thinking":{"type":"enabled","budget_tokens":24576}}`, "high"},
	} {
		out := applyModelGateGeminiThinkingConfig([]byte(`{"contents":[]}`), []byte(tc.body), "gemini-3.8-flash")
		cfg := gjson.GetBytes(out, "generationConfig.thinkingConfig")
		require.Equal(t, tc.level, cfg.Get("thinkingLevel").String(), "budget %d", tc.budget)
		require.True(t, cfg.Get("includeThoughts").Bool())
		require.False(t, cfg.Get("thinkingBudget").Exists())
	}
}

func TestModelGateGeminiThinkingHonorsDisableAndModelSuffix(t *testing.T) {
	base := []byte(`{"generationConfig":{"thinkingConfig":{"thinkingBudget":8000,"includeThoughts":true}}}`)
	out := applyModelGateGeminiThinkingConfig(base, []byte(`{"thinking":{"type":"disabled"}}`), "gemini-2.5-flash")
	require.Zero(t, gjson.GetBytes(out, "generationConfig.thinkingConfig.thinkingBudget").Int())
	require.False(t, gjson.GetBytes(out, "generationConfig.thinkingConfig.includeThoughts").Bool())
	out = applyModelGateGeminiThinkingConfig(base, []byte(`{"output_config":{"effort":"low"}}`), "gemini-3.8-flash-high")
	require.Equal(t, "high", gjson.GetBytes(out, "generationConfig.thinkingConfig.thinkingLevel").String())
	require.False(t, gjson.GetBytes(out, "generationConfig.thinkingConfig.thinkingBudget").Exists())
}
