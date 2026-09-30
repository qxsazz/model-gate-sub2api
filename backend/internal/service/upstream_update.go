package service

import (
	"context"
	_ "embed"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"
)

// Update only after merging and verifying the corresponding upstream source.
//
//go:embed upstream_version.txt
var upstreamBaseline string

const upstreamCacheTTL = 6 * time.Hour
const upstreamRefreshCooldown = 5 * time.Minute

// UpstreamUpdateInfo is informational and never grants permission to install binaries.
type UpstreamUpdateInfo struct {
	BaselineVersion string `json:"baseline_version"`
	LatestVersion   string `json:"latest_version"`
	HasUpdate       bool   `json:"has_update"`
	Status          string `json:"status"`
	ReleaseURL      string `json:"release_url"`
	CheckedAt       string `json:"checked_at"`
	AttemptedAt     string `json:"attempted_at"`
}

type upstreamUpdateState struct {
	mu          sync.Mutex
	info        UpstreamUpdateInfo
	lastAttempt time.Time
	checking    bool
	done        chan struct{}
}

// A single bounded read-only fetch serves concurrent admin requests. Old Redis
// updater records are intentionally ignored; this process cache contains no MG version.
func (s *UpdateService) checkUpstream(ctx context.Context, force bool) UpstreamUpdateInfo {
	state := &s.upstream
	now := time.Now()
	state.mu.Lock()
	state.info.BaselineVersion = strings.TrimSpace(upstreamBaseline)
	if state.checking {
		done := state.done
		previous := state.info
		state.mu.Unlock()
		select {
		case <-done:
			state.mu.Lock()
			result := state.info
			state.mu.Unlock()
			return result
		case <-ctx.Done():
			previous.Status = "unavailable"
			if previous.CheckedAt != "" {
				previous.Status = "stale"
			}
			return previous
		}
	}
	elapsed := now.Sub(state.lastAttempt)
	if !state.lastAttempt.IsZero() && (elapsed < upstreamRefreshCooldown || (!force && state.info.Status == "ok" && elapsed < upstreamCacheTTL)) {
		result := state.info
		state.mu.Unlock()
		return result
	}
	state.checking = true
	state.done = make(chan struct{})
	state.lastAttempt = now
	state.info.AttemptedAt = now.UTC().Format(time.RFC3339)
	state.mu.Unlock()

	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var release *GitHubRelease
	valid := false
	if s.githubClient != nil {
		fetched, err := s.githubClient.FetchLatestRelease(fetchCtx, githubRepo)
		if err == nil && fetched != nil {
			tag := "v" + strings.TrimPrefix(strings.TrimSpace(fetched.TagName), "v")
			if !fetched.Draft && !fetched.Prerelease && semver.IsValid("v"+strings.TrimSpace(upstreamBaseline)) && semver.IsValid(tag) && semver.Prerelease(tag) == "" {
				release = fetched
				valid = true
			}
		}
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	state.checking = false
	close(state.done)
	if !valid {
		state.info.Status = "unavailable"
		if state.info.CheckedAt != "" {
			state.info.Status = "stale"
		}
		return state.info
	}
	tag := "v" + strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	state.info.LatestVersion = strings.TrimPrefix(tag, "v")
	state.info.HasUpdate = semver.Compare("v"+state.info.BaselineVersion, tag) < 0
	state.info.ReleaseURL = "https://github.com/" + githubRepo + "/releases/tag/" + tag
	state.info.Status = "ok"
	state.info.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return state.info
}
