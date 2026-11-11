package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode"
)

type BotIntegrationConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	PublicURL   string `mapstructure:"public_url"`
	ClientsJSON string `mapstructure:"clients_json"`
}

type BotIntegrationScope struct {
	Platform string   `json:"platform"`
	BotID    string   `json:"bot_id"`
	GroupIDs []string `json:"group_ids"`
}

type BotIntegrationClient struct {
	ID          string                `json:"id"`
	TokenSHA256 string                `json:"token_sha256"`
	Scopes      []BotIntegrationScope `json:"scopes"`
}

func validBotConfigID(value string) bool {
	return value != "" && len(value) <= 200 && !strings.ContainsFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.In(r, unicode.Cf) })
}

func (c BotIntegrationConfig) Clients() ([]BotIntegrationClient, error) {
	var clients []BotIntegrationClient
	decoder := json.NewDecoder(strings.NewReader(c.ClientsJSON))
	decoder.DisallowUnknownFields()
	if len(c.ClientsJSON) > 65536 || decoder.Decode(&clients) != nil {
		return nil, fmt.Errorf("bot_integration.clients_json must contain a valid client array")
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || len(clients) == 0 || len(clients) > 16 {
		return nil, fmt.Errorf("bot_integration requires 1 to 16 clients")
	}
	seenID, seenToken := map[string]bool{}, map[string]bool{}
	for _, client := range clients {
		digest, err := hex.DecodeString(client.TokenSHA256)
		if !validBotConfigID(client.ID) || seenID[client.ID] || err != nil || len(digest) != 32 || seenToken[strings.ToLower(client.TokenSHA256)] || len(client.Scopes) == 0 || len(client.Scopes) > 20 {
			return nil, fmt.Errorf("bot_integration client IDs and SHA-256 credentials must be unique with explicit scopes")
		}
		seenID[client.ID], seenToken[strings.ToLower(client.TokenSHA256)] = true, true
		for _, scope := range client.Scopes {
			if (scope.Platform != "qq_official" && scope.Platform != "wechat_native") || !validBotConfigID(scope.BotID) || len(scope.GroupIDs) == 0 || len(scope.GroupIDs) > 100 {
				return nil, fmt.Errorf("bot_integration scopes require an exact supported platform, bot and group IDs")
			}
			for _, group := range scope.GroupIDs {
				if !validBotConfigID(group) || group == "*" {
					return nil, fmt.Errorf("bot_integration group IDs must be explicit")
				}
			}
			if scope.BotID == "*" {
				return nil, fmt.Errorf("bot_integration bot IDs must be explicit")
			}
		}
	}
	return clients, nil
}

func (c BotIntegrationConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || strings.TrimSpace(c.PublicURL) != c.PublicURL {
		return fmt.Errorf("bot_integration.public_url must be an HTTPS origin without credentials, path, query or fragment")
	}
	_, err = c.Clients()
	return err
}
