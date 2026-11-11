package config

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestBotIntegrationConfigRequiresExplicitCredentialScope(t *testing.T) {
	valid := BotIntegrationClient{ID: "local-astrbot", TokenSHA256: strings.Repeat("ab", 32), Scopes: []BotIntegrationScope{{Platform: "qq_official", BotID: "bot", GroupIDs: []string{"group"}}}}
	config := func(clients []BotIntegrationClient) BotIntegrationConfig {
		raw, err := json.Marshal(clients)
		require.NoError(t, err)
		return BotIntegrationConfig{Enabled: true, PublicURL: "https://staging.model-gate.cc", ClientsJSON: string(raw)}
	}
	require.NoError(t, config([]BotIntegrationClient{valid}).Validate())
	require.NoError(t, (BotIntegrationConfig{}).Validate())
	for _, origin := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com/path", "https://example.com?x=y", "https://example.com/#x", " https://example.com"} {
		c := config([]BotIntegrationClient{valid})
		c.PublicURL = origin
		require.Error(t, c.Validate())
	}
	require.Error(t, config([]BotIntegrationClient{valid, valid}).Validate())
	for _, mutate := range []func(*BotIntegrationClient){
		func(c *BotIntegrationClient) { c.TokenSHA256 = "plaintext-token" },
		func(c *BotIntegrationClient) { c.Scopes = nil },
		func(c *BotIntegrationClient) {
			c.Scopes = []BotIntegrationScope{{Platform: "qq_official", BotID: "*", GroupIDs: []string{"group"}}}
		},
		func(c *BotIntegrationClient) {
			c.Scopes = []BotIntegrationScope{{Platform: "qq_official", BotID: "bot", GroupIDs: []string{"*"}}}
		},
		func(c *BotIntegrationClient) {
			c.Scopes = []BotIntegrationScope{{Platform: "qq", BotID: "bot", GroupIDs: []string{"group"}}}
		},
	} {
		bad := valid
		mutate(&bad)
		require.Error(t, config([]BotIntegrationClient{bad}).Validate())
	}
	c := config([]BotIntegrationClient{valid})
	c.ClientsJSON = strings.Replace(c.ClientsJSON, `"id":`, `"typo":`, 1)
	require.Error(t, c.Validate())
}
