package service

import "strings"

// GroupKeyCapabilities describes creation choices, not a separate credential store.
// Requests still use the group's gateway permissions and model allowlist.
type GroupKeyCapabilities struct {
	Image bool `json:"image"`
	Video bool `json:"video"`
	Text  bool `json:"text"`
}

func (c GroupKeyCapabilities) Allows(purpose string) bool {
	switch purpose {
	case "":
		return true
	case "image":
		return c.Image
	case "video":
		return c.Video
	case "text":
		return c.Text
	default:
		return false
	}
}

func keyModelPurpose(model string) string {
	model = strings.ToLower(model)
	for _, word := range []string{"video", "sora", "veo", "kling", "hailuo"} {
		if strings.Contains(model, word) {
			return "video"
		}
	}
	for _, word := range []string{"image", "imagen", "banana", "dall-e", "dalle", "flux", "seedream", "midjourney", "sdxl", "grok-imagine", "stable-diffusion"} {
		if strings.Contains(model, word) {
			return "image"
		}
	}
	for _, word := range []string{"tts", "audio", "whisper", "speech"} {
		if strings.Contains(model, word) {
			return "audio"
		}
	}
	return "text"
}

// KeyCapabilities uses permissions and model policy, never a group's display name.
func (g *Group) KeyCapabilities() GroupKeyCapabilities {
	if g == nil || g.Status != "active" {
		return GroupKeyCapabilities{}
	}
	result := GroupKeyCapabilities{Text: true}
	switch g.Platform {
	case PlatformOpenAI, PlatformGemini, PlatformAntigravity, PlatformGrok, PlatformComposite:
		result.Image = g.AllowImageGeneration
	}
	result.Video = g.AllowImageGeneration && (g.Platform == PlatformGrok || g.Platform == PlatformComposite)
	if !g.ModelAllowlist.Enabled {
		return result
	}
	allowed := map[string]bool{}
	// Concrete entries cover custom model names; witnesses cover broad wildcard rules.
	candidates := append([]string{}, g.ModelAllowlist.Models...)
	candidates = append(candidates, "gpt-image-1", "gpt-image-2", "gemini-2.5-flash-image", "gemini-3-pro-image-preview", "gemini-3.1-flash-image-preview", "grok-imagine-image", "grok-imagine-video", "gpt-5.5", "gpt-6", "gemini-2.5-flash", "grok-4.5", "claude-sonnet-4-6")
	for _, model := range candidates {
		model = strings.ReplaceAll(model, "*", "")
		if model == "" {
			continue
		}
		if g.ModelAllowlist.Allows(model) {
			allowed[keyModelPurpose(model)] = true
		}
	}
	result.Image = result.Image && allowed["image"]
	result.Video = result.Video && allowed["video"]
	result.Text = allowed["text"]
	return result
}
