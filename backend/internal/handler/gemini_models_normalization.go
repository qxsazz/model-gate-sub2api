package handler

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// normalizeGeminiModelsBody only recognizes model-list envelopes. Unknown and
// malformed responses retain the existing passthrough behavior, never becoming
// a fabricated successful empty list. The caller still applies its allowlist.
func normalizeGeminiModelsBody(body []byte) ([]byte, bool) {
	var envelope map[string]json.RawMessage
	if json.Unmarshal(body, &envelope) != nil || envelope == nil {
		return body, false
	}
	var object string
	_ = json.Unmarshal(envelope["object"], &object)
	var entries []json.RawMessage
	dataArray := isGeminiModelsJSONArray(envelope["data"]) && json.Unmarshal(envelope["data"], &entries) == nil
	openAIList := object == "list" && dataArray

	if raw := envelope["models"]; isGeminiModelsJSONArray(raw) {
		var models []struct {
			Name string `json:"name"`
		}
		valid := json.Unmarshal(raw, &models) == nil
		for _, model := range models {
			valid = valid && strings.TrimSpace(model.Name) != ""
		}
		if valid {
			duplicateCatalogue := openAIList
			for _, raw := range entries {
				var entry struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(raw, &entry) == nil && strings.TrimSpace(entry.ID) != "" {
					duplicateCatalogue = true
					break
				}
			}
			if !duplicateCatalogue {
				return body, true
			}
			// Preserve native metadata and pagination, but never leak a second
			// model catalogue that the Google allowlist would not inspect.
			if object == "list" {
				delete(envelope, "object")
			}
			delete(envelope, "data")
			out, err := json.Marshal(envelope)
			return out, err == nil
		}
	}
	if !openAIList {
		return body, false
	}
	models := make([]map[string]string, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, raw := range entries {
		var entry struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		}
		if json.Unmarshal(raw, &entry) != nil || (entry.Object != "" && entry.Object != "model") {
			continue
		}
		id := strings.TrimPrefix(entry.ID, "models/")
		if id != strings.TrimSpace(id) || !service.IsSafeGeminiModelPathSegment(id) {
			continue
		}
		name := "models/" + id
		if !seen[name] {
			models = append(models, map[string]string{"name": name})
			seen[name] = true
		}
	}
	if len(entries) > 0 && len(models) == 0 {
		return body, false
	}
	// Do not copy arbitrary upstream fields or invent generation capabilities.
	out, err := json.Marshal(map[string]any{"models": models})
	return out, err == nil
}

func isGeminiModelsJSONArray(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	return len(raw) > 0 && raw[0] == '['
}
