//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Exercise MG's thinking/text conversion together with upstream disconnect billing.
func TestModelGateGeminiThinkingStreamPreservesUsageOnDisconnect(t *testing.T) {
	body := []byte(`{"model":"gemini-3-flash-preview","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	thought := `data: {"candidates":[{"content":{"parts":[{"text":"reason about it","thought":true,"thoughtSignature":"signature-1"}],"role":"model"}}]}` + "\n\n"
	for _, finished := range []bool{false, true} {
		t.Run(map[bool]string{false: "partial", true: "finished"}[finished], func(t *testing.T) {
			usage := geminiAbortUsageChunk
			if finished {
				usage = geminiAbortFinalChunk
			}
			svc, c, rec := newGeminiAbortingStreamService(t, "/v1/chat/completions", body, thought+geminiAbortTextChunk+usage, true)
			result, err := svc.ForwardAsChatCompletions(context.Background(), c, geminiVertexAPIKeyTestAccount(""), body)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.True(t, result.ClientDisconnect)
			require.Positive(t, result.Usage.InputTokens)
			require.Positive(t, result.Usage.OutputTokens)
			require.Contains(t, rec.Body.String(), `"reasoning_content":"reason about it"`)
			require.Contains(t, rec.Body.String(), `"content":"hello"`)
		})
	}
}
