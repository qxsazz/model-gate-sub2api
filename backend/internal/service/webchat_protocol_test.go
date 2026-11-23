package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestWebchatNativeProtocolsKeepImagesAndSystemContext(t *testing.T) {
	wire := []webchatWireMessage{{Role: "system", Content: "站方资料"}}
	wire = append(wire, webchatWireMessages([]WebchatMessage{{Role: "user", Content: "请读图", Attachments: []WebchatAttachment{webchatTestImage(t)}}})...)
	messages, err := json.Marshal(wire)
	require.NoError(t, err)
	for _, tc := range []struct {
		platform, model, path, system, image, reply string
	}{
		{PlatformOpenAI, "gpt-5.5", "/v1/responses", "input.0.content", "input.1.content.1.image_url", `{"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}]}`},
		{PlatformAnthropic, "claude-haiku-5-5", "/v1/messages", "system", "messages.0.content.1.source.data", `{"content":[{"type":"thinking","thinking":"internal"},{"type":"text","text":"OK"}],"stop_reason":"end_turn"}`},
		{PlatformGemini, "gemini-2.5-flash", "/v1beta/models/gemini-2.5-flash:generateContent", "systemInstruction.parts.0.text", "contents.0.parts.1.inlineData.data", `{"candidates":[{"content":{"role":"model","parts":[{"text":"OK"}]},"finishReason":"STOP"}]}`},
		{PlatformGrok, "grok-test", "/v1/chat/completions", "messages.0.content", "messages.1.content.1.image_url.url", `{"choices":[{"message":{"role":"assistant","content":"OK"}}]}`},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			endpoint := webchatPlatformEndpoint(tc.platform, tc.model)
			path, body, err := webchatNativeRequest(endpoint, tc.model, messages)
			require.NoError(t, err)
			require.Equal(t, tc.path, path)
			require.Contains(t, gjson.GetBytes(body, tc.system).String(), "站方资料")
			require.NotEmpty(t, gjson.GetBytes(body, tc.image).String())
			require.False(t, gjson.GetBytes(body, "stream").Bool())
			if tc.platform == PlatformOpenAI {
				require.Equal(t, gjson.False, gjson.GetBytes(body, "store").Type)
			}
			reply, err := webchatNativeReply(endpoint, tc.model, []byte(tc.reply))
			require.NoError(t, err)
			require.Equal(t, "OK", reply.text())
		})
	}
}

func TestWebchatCompositeAliasUsesConfiguredEndpoint(t *testing.T) {
	group := &Group{ID: 17, Platform: PlatformComposite}
	r := NewCompositeRouteResolver(&compositeRouteRepoStub{routes: []CompositeModelRoute{
		{GroupID: group.ID, PublicModel: "team", MatchType: CompositeRouteMatchPrefix, Endpoint: CompositeRouteEndpointAny, TargetPlatform: PlatformOpenAI, Enabled: true},
		{GroupID: group.ID, PublicModel: "team-claude", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointMessages, TargetPlatform: PlatformAnthropic, Enabled: true},
		{GroupID: group.ID, PublicModel: "team-compat", MatchType: CompositeRouteMatchExact, Endpoint: CompositeRouteEndpointChatCompletions, TargetPlatform: PlatformOpenAI, Enabled: true},
	}})
	r.SetModelOwnershipResolver(func(context.Context, int64, string) (CompositeModelOwnership, error) {
		return CompositeModelOwnership{Matched: true, TargetPlatform: PlatformGemini}, nil
	})
	for alias, want := range map[string]string{"team-claude": CompositeRouteEndpointMessages, "team-compat": CompositeRouteEndpointChatCompletions, "team-gpt": CompositeRouteEndpointResponses, "owned-alias": CompositeRouteEndpointGemini} {
		got, err := r.webchatEndpoint(context.Background(), group, alias)
		require.NoError(t, err)
		require.Equal(t, want, got, alias)
	}
}

func TestWebchatKnowledgeMatchesTheTurn(t *testing.T) {
	history := []WebchatMessage{{Role: "user", Content: "codex-pro2 分组倍率怎么计算？"}, {Role: "assistant", Content: "分组有不同的 VIP 档位。"}}
	for _, tc := range []struct{ question, want string }{
		{"那 VIP3 呢？", "codex-pro2：0.4 / 0.385 / 0.37 / 0.355"},
		{"codex-plus2 分组 gpt-5.5 输入输出的单价是多少？", "分组「codex-plus2」模型「gpt-5.5」"},
		{"How do group multipliers work?", "ModelGate 29 个分组倍率"},
		{"本站 token 的费用怎么算？", "ModelGate 余额与计费"},
		{"你好", ""}, {"那帮我写首诗", ""}, {"介绍 Gemini 的工作原理", ""},
	} {
		got := webchatKnowledgeForTurn(append(history, WebchatMessage{Role: "user", Content: tc.question}))
		if tc.want == "" {
			require.Empty(t, got, tc.question)
		} else {
			require.Contains(t, got, tc.want, tc.question)
			require.Contains(t, got, "2026-11-23")
		}
	}
}
