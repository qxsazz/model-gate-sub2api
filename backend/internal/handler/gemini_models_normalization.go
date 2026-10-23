package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// combineGeminiModelsSources unions genuine catalogues in source priority order.
// Keep the first source's full model metadata for duplicates. Failures and
// malformed success bodies cannot hide another source's valid catalogue.
func combineGeminiModelsSources(sources []service.GeminiModelsSource) *service.UpstreamHTTPResult {
	var fallback, first *service.UpstreamHTTPResult
	var envelope map[string]json.RawMessage
	var models []json.RawMessage
	seen := make(map[string]bool)
	validSources := 0
	for _, source := range sources {
		res := source.Response
		if res == nil {
			continue
		}
		if fallback == nil {
			fallback = res
		}
		if res.StatusCode != http.StatusOK {
			continue
		}
		body, ok := normalizeGeminiModelsBody(res.Body)
		if !ok {
			continue
		}
		var current map[string]json.RawMessage
		if json.Unmarshal(body, &current) != nil {
			continue
		}
		var entries []json.RawMessage
		if json.Unmarshal(current["models"], &entries) != nil {
			continue
		}
		validSources++
		if first == nil {
			copy := *res
			copy.Body = body
			first = &copy
			envelope = current
		}
		for _, entry := range entries {
			var model struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(entry, &model) != nil {
				continue
			}
			name := strings.TrimPrefix(model.Name, "models/")
			if !seen[name] {
				models = append(models, entry)
				seen[name] = true
			}
		}
	}
	if first == nil {
		return fallback
	}
	if validSources > 1 {
		if models == nil {
			models = []json.RawMessage{}
		}
		envelope["models"], _ = json.Marshal(models)
		// A page token belongs to one upstream, not to an aggregated catalogue.
		delete(envelope, "nextPageToken")
		first.Body, _ = json.Marshal(envelope)
	}
	// Normalization or aggregation changes response validators and byte length.
	first.Headers = first.Headers.Clone()
	if first.Headers == nil {
		first.Headers = make(http.Header)
	}
	if fallback == nil || !bytes.Equal(first.Body, fallback.Body) || validSources > 1 {
		first.Headers.Set("Content-Type", "application/json")
		for _, name := range []string{"Content-Length", "Content-Encoding", "ETag", "Content-MD5"} {
			first.Headers.Del(name)
		}
	}
	return first
}

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
