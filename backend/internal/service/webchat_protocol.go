package service

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/Wei-Shaw/sub2api/internal/util/logredact"
	"github.com/tidwall/gjson"
)

// WebchatCatalogScope includes public IDs exposed on any supported conversational endpoint.
const WebchatCatalogScope = "webchat"

var webchatEndpoints = []string{CompositeRouteEndpointResponses, CompositeRouteEndpointMessages, CompositeRouteEndpointGemini, CompositeRouteEndpointChatCompletions}

func webchatPlatformEndpoint(platform, model string) string {
	switch platform {
	case PlatformOpenAI:
		return CompositeRouteEndpointResponses
	case PlatformAnthropic:
		return CompositeRouteEndpointMessages
	case PlatformAntigravity:
		if family, _ := DetectModelPlatform(model); family != PlatformGemini {
			return CompositeRouteEndpointMessages
		}
		fallthrough
	case PlatformGemini:
		if IsSafeGeminiModelPathSegment(model) {
			return CompositeRouteEndpointGemini
		}
	case PlatformTypeSafe:
		return ""
	}
	// Chat-compatible providers and Gemini aliases that require a JSON model field.
	return CompositeRouteEndpointChatCompletions
}

// Exact endpoint rules and aliases use the same precedence as gateway dispatch.
func matchWebchatRoute(routes []CompositeModelRoute, model string) (string, bool) {
	var chosen string
	var best CompositeModelRoute
	var matched, preferred bool
	for _, endpoint := range webchatEndpoints {
		route, ok := matchCompositeRoute(routes, model, endpoint)
		if !ok {
			continue
		}
		matched = true
		if route.TargetPlatform == PlatformTypeSafe || (endpoint == CompositeRouteEndpointGemini &&
			(!IsSafeGeminiModelPathSegment(model) || (route.TargetPlatform != PlatformGemini && route.TargetPlatform != PlatformAntigravity))) {
			continue
		}
		target := route.UpstreamModel
		if target == "" {
			target = model
		}
		native := webchatPlatformEndpoint(route.TargetPlatform, target)
		if native == CompositeRouteEndpointGemini && !IsSafeGeminiModelPathSegment(model) {
			native = CompositeRouteEndpointChatCompletions
		}
		explicit := route.Endpoint != CompositeRouteEndpointAny
		// Within each endpoint the gateway already selected its winning rule.
		// Across endpoints prefer the most specific model/endpoint rule, then
		// its configured priority. Use the provider's native dialect to break ties.
		better := chosen == ""
		switch {
		case chosen == "":
		case route.MatchType != best.MatchType:
			better = route.MatchType == CompositeRouteMatchExact
		case explicit != (best.Endpoint != CompositeRouteEndpointAny):
			better = explicit
		case len(route.PublicModel) != len(best.PublicModel):
			better = len(route.PublicModel) > len(best.PublicModel)
		case route.Priority != best.Priority:
			better = route.Priority < best.Priority
		default:
			better = !preferred && endpoint == native
		}
		if better {
			chosen, best, preferred = endpoint, route, endpoint == native
		}
	}
	return chosen, matched
}

func (r *CompositeRouteResolver) webchatEndpoint(ctx context.Context, group *Group, model string) (string, error) {
	if group == nil {
		platform, _ := DetectModelPlatform(model)
		return webchatPlatformEndpoint(platform, model), nil
	}
	if group.Platform != PlatformComposite {
		return webchatPlatformEndpoint(group.Platform, model), nil
	}
	if r != nil && r.repo != nil {
		routes, err := r.repo.ListByGroup(ctx, group.ID, false)
		if err != nil {
			return "", err
		}
		if endpoint, matched := matchWebchatRoute(routes, model); matched {
			return endpoint, nil
		}
	}
	decision, err := r.resolveUnroutedModel(ctx, CompositeRouteDecision{GroupID: group.ID, PublicModel: model, Endpoint: CompositeRouteEndpointAny})
	if err != nil {
		return "", err
	}
	// Leave ambiguous accounts to the gateway; it owns the client-facing diagnostic.
	return webchatPlatformEndpoint(decision.TargetPlatform, model), nil
}

var webchatBearerSecret = regexp.MustCompile(`(?i)\b(?:Bearer\s+[a-z0-9._~+/=-]+|(?:sk|rk|sess)-[a-z0-9_-]{10,}|eyJ[a-z0-9_-]+\.[a-z0-9_-]+\.[a-z0-9_-]+)\b`)

func webchatGatewayMessage(data []byte, key *APIKey) string {
	if !gjson.ValidBytes(data) {
		return ""
	}
	message := ExtractUpstreamErrorMessage(data)
	if message == "" {
		if field := gjson.GetBytes(data, "error"); field.Type == gjson.String {
			message = field.Str
		}
	}
	if key != nil && key.Key != "" {
		message = strings.ReplaceAll(message, key.Key, "[redacted]")
	}
	message = webchatBearerSecret.ReplaceAllString(message, "[redacted]")
	message = logredact.RedactText(SanitizeUpstreamErrorMessage(message), "api_key", "x-api-key", "apikey", "authorization", "cookie", "token", "secret")
	return trimRunes(message, 1600)
}

func (b *WebchatBridge) forwardWebchat(ctx context.Context, key *APIKey, ip, model string, messages json.RawMessage) (webchatCompletion, *WebchatUpstreamError) {
	var empty webchatCompletion
	endpoint, err := b.resolver.webchatEndpoint(ctx, key.Group, model)
	if err != nil {
		return empty, webchatError(http.StatusServiceUnavailable)
	}
	if endpoint == "" {
		return empty, &WebchatUpstreamError{Status: http.StatusForbidden, Message: "当前分组未开放此模型的对话接口"}
	}
	path, payload, err := webchatNativeRequest(endpoint, model, messages)
	if err != nil {
		return empty, webchatError(http.StatusBadRequest)
	}
	if endpoint == CompositeRouteEndpointGemini && key.Group != nil && key.Group.Platform == PlatformAntigravity {
		path = "/antigravity" + path
	}
	out, gatewayErr := b.dispatch(ctx, key, ip, http.MethodPost, path, payload)
	if gatewayErr != nil {
		return empty, gatewayErr
	}
	// Some native APIs return a failed response object even with HTTP 200.
	if failure := gjson.GetBytes(out, "error"); failure.Exists() && failure.Type != gjson.Null {
		return empty, &WebchatUpstreamError{Status: http.StatusBadGateway, Message: webchatGatewayMessage(out, key)}
	}
	result, err := webchatNativeReply(endpoint, model, out)
	if err != nil || result.text() == "" {
		return empty, webchatError(http.StatusBadGateway)
	}
	return result, nil
}

func webchatNativeRequest(endpoint, model string, messages json.RawMessage) (string, []byte, error) {
	if endpoint == CompositeRouteEndpointChatCompletions {
		body, err := json.Marshal(map[string]any{"model": model, "messages": messages, "stream": false})
		return "/v1/chat/completions", body, err
	}
	request := apicompat.ChatCompletionsRequest{Model: model}
	if err := json.Unmarshal(messages, &request.Messages); err != nil {
		return "", nil, err
	}
	responses, err := apicompat.ChatCompletionsToResponses(&request)
	if err != nil {
		return "", nil, err
	}
	responses.Stream = false
	responses.Include = nil
	if endpoint == CompositeRouteEndpointResponses {
		body, err := json.Marshal(responses)
		return "/v1/responses", body, err
	}
	anthropic, err := apicompat.ResponsesToAnthropicRequest(responses)
	if err != nil {
		return "", nil, err
	}
	body, err := json.Marshal(anthropic)
	if err != nil || endpoint == CompositeRouteEndpointMessages {
		return "/v1/messages", body, err
	}
	if endpoint != CompositeRouteEndpointGemini || !IsSafeGeminiModelPathSegment(model) {
		return "", nil, ErrWebchatModelUnavailable
	}
	body, err = convertClaudeMessagesToGeminiGenerateContent(body, model)
	return "/v1beta/models/" + model + ":generateContent", body, err
}

func webchatNativeReply(endpoint, model string, data []byte) (webchatCompletion, error) {
	var chat *apicompat.ChatCompletionsResponse
	switch endpoint {
	case CompositeRouteEndpointResponses:
		var response apicompat.ResponsesResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return webchatCompletion{}, err
		}
		chat = apicompat.ResponsesToChatCompletions(&response, model)
	case CompositeRouteEndpointMessages:
		var response apicompat.AnthropicResponse
		if err := json.Unmarshal(data, &response); err != nil {
			return webchatCompletion{}, err
		}
		chat = apicompat.ResponsesToChatCompletions(apicompat.AnthropicToResponsesResponse(&response), model)
	case CompositeRouteEndpointGemini:
		var response map[string]any
		if err := json.Unmarshal(data, &response); err != nil {
			return webchatCompletion{}, err
		}
		var err error
		chat, _, err = geminiResponseToChatCompletions(response, model, data, nil)
		if err != nil {
			return webchatCompletion{}, err
		}
	default:
		chat = new(apicompat.ChatCompletionsResponse)
		if err := json.Unmarshal(data, chat); err != nil {
			return webchatCompletion{}, err
		}
	}
	var result webchatCompletion
	if len(chat.Choices) == 1 {
		msg := chat.Choices[0].Message
		text := msg.Refusal
		if len(msg.Content) > 0 && string(msg.Content) != "null" {
			if err := json.Unmarshal(msg.Content, &text); err != nil {
				return result, err
			}
		}
		result.Choices = []webchatChoice{{Message: webchatReply{Role: msg.Role, Content: text}}}
	}
	return result, nil
}
