package service

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func geminiThoughtAndAnswerResponse() map[string]any {
	return map[string]any{
		"candidates": []any{map[string]any{
			"content": map[string]any{"parts": []any{
				map[string]any{"text": "reasoning summary", "thought": true, "thoughtSignature": "sig-1"},
				map[string]any{"text": "final answer"},
			}},
			"finishReason": "STOP",
		}},
	}
}

func TestGeminiReasoningNonStreamingProtocolMappings(t *testing.T) {
	geminiResp := geminiThoughtAndAnswerResponse()
	raw, err := json.Marshal(geminiResp)
	require.NoError(t, err)

	claudeResp, _ := convertGeminiToClaudeMessage(geminiResp, "gemini-3-test", raw, false)
	blocks, ok := claudeResp["content"].([]any)
	require.True(t, ok)
	require.Equal(t, map[string]any{
		"type": "thinking", "thinking": "reasoning summary", "signature": "sig-1",
	}, blocks[0])
	require.Equal(t, map[string]any{"type": "text", "text": "final answer"}, blocks[1])

	chatResp, _, err := geminiResponseToChatCompletions(geminiResp, "gemini-3-test", raw, nil)
	require.NoError(t, err)
	require.Len(t, chatResp.Choices, 1)
	require.Equal(t, "reasoning summary", chatResp.Choices[0].Message.ReasoningContent)
	var content string
	require.NoError(t, json.Unmarshal(chatResp.Choices[0].Message.Content, &content))
	require.Equal(t, "final answer", content)
}

func TestGeminiReasoningStreamingProtocolMappings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstreamBody := `data: {"candidates":[{"content":{"parts":[{"text":"reasoning summary","thought":true,"thoughtSignature":"sig-1"}]}}]}` + "\n\n" +
		`data: {"candidates":[{"content":{"parts":[{"text":"final answer"}]},"finishReason":"STOP"}]}` + "\n\n" +
		"data: [DONE]\n\n"

	t.Run("anthropic messages", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(upstreamBody))}
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		svc := &GeminiMessagesCompatService{}
		_, err := svc.handleStreamingResponse(ctx, resp, time.Now(), "gemini-3-test")
		require.NoError(t, err)
		body := rec.Body.String()
		require.Contains(t, body, `"thinking":"reasoning summary","type":"thinking_delta"`)
		require.Contains(t, body, `"signature":"sig-1","type":"signature_delta"`)
		require.Contains(t, body, `"text":"final answer","type":"text_delta"`)
	})

	t.Run("chat completions", func(t *testing.T) {
		resp := &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(upstreamBody))}
		rec := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(rec)
		svc := &GeminiMessagesCompatService{}
		_, err := svc.handleChatCompletionsStreamingResponseFromGemini(ctx, resp, time.Now(), "gemini-3-test", false, false)
		require.NoError(t, err)
		body := rec.Body.String()
		require.Contains(t, body, `"reasoning_content":"reasoning summary"`)
		require.Contains(t, body, `"content":"final answer"`)
		require.NotContains(t, body, `"content":"reasoning summary"`)
	})
}

func TestCollectGeminiSSEKeepsThoughtSeparateFromAnswer(t *testing.T) {
	upstreamBody := `data: {"candidates":[{"content":{"parts":[{"text":"reasoning ","thought":true,"thoughtSignature":"sig-1"}]}}]}` + "\n\n" +
		`data: {"candidates":[{"content":{"parts":[{"text":"summary","thought":true,"thoughtSignature":"sig-1"}]}}]}` + "\n\n" +
		`data: {"candidates":[{"content":{"parts":[{"text":"final answer"}]},"finishReason":"STOP"}]}` + "\n\n" +
		"data: [DONE]\n\n"

	got, _, err := collectGeminiSSE(strings.NewReader(upstreamBody), false)
	require.NoError(t, err)
	parts := extractGeminiParts(got)
	require.Len(t, parts, 2)
	require.Equal(t, "reasoning summary", parts[0]["text"])
	require.Equal(t, true, parts[0]["thought"])
	require.Equal(t, "sig-1", parts[0]["thoughtSignature"])
	require.Equal(t, "final answer", parts[1]["text"])
	require.NotContains(t, parts[1], "thought")
}

func TestClaudeThinkingReplayRequiresGeminiSignature(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"signed","signature":"sig-1"},{"type":"thinking","thinking":"unsigned"},{"type":"text","text":"answer"}]}]}`)
	converted, err := convertClaudeMessagesToGeminiGenerateContent(body)
	require.NoError(t, err)

	var request map[string]any
	require.NoError(t, json.Unmarshal(converted, &request))
	contents, ok := request["contents"].([]any)
	require.True(t, ok)
	content, ok := contents[0].(map[string]any)
	require.True(t, ok)
	parts, ok := content["parts"].([]any)
	require.True(t, ok)
	require.Len(t, parts, 2)
	require.Equal(t, map[string]any{
		"text": "signed", "thought": true, "thoughtSignature": "sig-1",
	}, parts[0])
	require.Equal(t, map[string]any{"text": "answer"}, parts[1])
}

func TestGeminiThinkingRequestConfigurationAcrossModelGenerations(t *testing.T) {
	body := []byte(`{"model":"gemini-test","messages":[{"role":"user","content":"hello"}],"thinking":{"type":"enabled","budget_tokens":8000}}`)
	converted, err := convertClaudeMessagesToGeminiGenerateContent(body)
	require.NoError(t, err)

	gemini3, err := configureGeminiThinking(converted, "gemini-3.8-flash")
	require.NoError(t, err)
	var gemini3Req map[string]any
	require.NoError(t, json.Unmarshal(gemini3, &gemini3Req))
	gemini3Generation, ok := gemini3Req["generationConfig"].(map[string]any)
	require.True(t, ok)
	gemini3Thinking, ok := gemini3Generation["thinkingConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, gemini3Thinking["includeThoughts"])
	require.Equal(t, "medium", gemini3Thinking["thinkingLevel"])
	require.NotContains(t, gemini3Thinking, "thinkingBudget")

	gemini25, err := configureGeminiThinking(converted, "gemini-2.5-flash-thinking")
	require.NoError(t, err)
	var gemini25Req map[string]any
	require.NoError(t, json.Unmarshal(gemini25, &gemini25Req))
	gemini25Generation, ok := gemini25Req["generationConfig"].(map[string]any)
	require.True(t, ok)
	gemini25Thinking, ok := gemini25Generation["thinkingConfig"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, gemini25Thinking["includeThoughts"])
	require.EqualValues(t, 8000, gemini25Thinking["thinkingBudget"])
	require.NotContains(t, gemini25Thinking, "thinkingLevel")
}
