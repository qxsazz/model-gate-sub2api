package service

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestGroupKeyCapabilities(t *testing.T) {
	for _, tc := range []struct {
		name, platform string
		media          bool
		models         []string
		want           GroupKeyCapabilities
	}{
		{"chat only", PlatformGrok, false, []string{"grok-4.5"}, GroupKeyCapabilities{Text: true}},
		{"chat whitelist despite media flag", PlatformGrok, true, []string{"grok-4.5", "grok-4.3"}, GroupKeyCapabilities{Text: true}},
		{"image only", PlatformOpenAI, true, []string{"gpt-image-2*"}, GroupKeyCapabilities{Image: true}},
		{"Google image", PlatformGemini, true, []string{"gemini-3.1-flash-image*"}, GroupKeyCapabilities{Image: true}},
		{"video only", PlatformGrok, true, []string{"grok-imagine-video*"}, GroupKeyCapabilities{Video: true}},
		{"broad Grok allowlist", PlatformGrok, true, []string{"grok-*"}, GroupKeyCapabilities{Image: true, Video: true, Text: true}},
		{"media disabled", PlatformGrok, false, []string{"grok-imagine-image", "grok-imagine-video"}, GroupKeyCapabilities{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &Group{Status: "active", Platform: tc.platform, AllowImageGeneration: tc.media, ModelAllowlist: GroupModelAllowlist{Enabled: true, Models: tc.models}}
			require.Equal(t, tc.want, g.KeyCapabilities())
		})
	}
	require.False(t, (&Group{Status: "inactive", Platform: PlatformGrok, AllowImageGeneration: true}).KeyCapabilities().Image)
	require.False(t, GroupKeyCapabilities{Text: true}.Allows("image"))
	require.False(t, GroupKeyCapabilities{Image: true}.Allows("invalid"))
}
