package service

import (
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// applyModelGateGeminiThinkingConfig keeps the deployed compatibility contract:
// summaries are available in both buffered and streaming responses, and Gemini
// 3 budget tiers retain their existing billing meaning. New model families and
// explicit disable intent use the upstream capability-aware parser.
func applyModelGateGeminiThinkingConfig(body, claudeBody []byte, model string) []byte {
	out := applyGeminiCompatThinkingConfig(body, claudeBody, model, true)
	name := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(model)), "models/")
	intent := parseClaudeThinkingIntent(claudeBody)
	if strings.HasPrefix(name, "gemini-3") && intent.thinkingType != "disabled" && !geminiModelNameCarriesThinkingTier(model) {
		level := geminiThinkingLevelFromClaudeBody(claudeBody)
		if level == "" && intent.tier == "" && intent.budgetTokens > 0 {
			level = geminiThinkingLevelForBudget(intent.budgetTokens)
		}
		if level != "" {
			out = setGeminiThinkingField(out, "thinkingLevel", level)
		}
	}
	// The tier belongs to the actual forwarded model, never the public alias.
	if level := geminiThinkingLevel(name); level != "" && geminiThinkingParamForModel(name) == geminiThinkingParamLevel {
		out = setGeminiThinkingField(out, "thinkingLevel", level)
	}
	if geminiThinkingParamForModel(name) == geminiThinkingParamLevel &&
		(intent.thinkingType == "disabled" || strings.EqualFold(gjson.GetBytes(claudeBody, "output_config.effort").String(), "none")) {
		out, _ = sjson.DeleteBytes(out, "generationConfig.thinkingConfig.includeThoughts")
	}
	if budget := gjson.GetBytes(out, "generationConfig.thinkingConfig.thinkingBudget"); budget.Exists() && budget.Int() == 0 {
		out = setGeminiThinkingField(out, "includeThoughts", false)
	}
	return out
}

func hasGeminiHistorySignature(block map[string]any) bool {
	signature, _ := block["signature"].(string)
	return strings.TrimSpace(signature) != ""
}
