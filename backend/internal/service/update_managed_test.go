//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type managedForbiddenGitHubClient struct{}

func (*managedForbiddenGitHubClient) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	panic("managed deployments must not fetch upstream releases")
}
func (*managedForbiddenGitHubClient) FetchRecentReleases(context.Context, string, int) ([]*GitHubRelease, error) {
	panic("managed deployments must not fetch upstream rollback candidates")
}
func (*managedForbiddenGitHubClient) DownloadFile(context.Context, string, string, int64) error {
	panic("managed deployments must not download binaries")
}
func (*managedForbiddenGitHubClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	panic("managed deployments must not fetch checksums")
}

type upstreamCheckClient struct {
	managedForbiddenGitHubClient
	release *GitHubRelease
	err     error
	calls   int
}

func (c *upstreamCheckClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	if repo != "Wei-Shaw/sub2api" {
		panic("unexpected upstream repository")
	}
	c.calls++
	return c.release, c.err
}
func managedFields(t *testing.T, svc *UpdateService, force bool) map[string]any {
	t.Helper()
	info, err := svc.CheckUpdate(context.Background(), force)
	require.NoError(t, err)
	encoded, err := json.Marshal(info)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(encoded, &result))
	require.Equal(t, "0.2.10-mg.2", result["current_version"])
	require.Equal(t, false, result["has_update"])
	require.Equal(t, "managed", result["deployment_mode"])
	return result
}
func TestManagedUpstreamNotificationSeparatesBaselineAndCaches(t *testing.T) {
	client := &upstreamCheckClient{release: &GitHubRelease{TagName: "v0.2.11"}}
	svc := NewUpdateService(&updateServiceCacheStub{data: `{"current_version":"0.2.8","latest_version":"9.0.0","has_update":true}`}, client, "0.2.10-mg.2", "release")
	result := managedFields(t, svc, false)
	upstream, ok := result["upstream"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "0.2.10", upstream["baseline_version"])
	require.Equal(t, "0.2.11", upstream["latest_version"])
	require.Equal(t, true, upstream["has_update"])
	require.Equal(t, "ok", upstream["status"])
	require.Equal(t, "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11", upstream["release_url"])
	require.Equal(t, 1, client.calls)
	managedFields(t, svc, true)
	require.Equal(t, 1, client.calls, "manual refresh respects cooldown")
}
func TestManagedUpstreamStableVersionValidation(t *testing.T) {
	for _, release := range []*GitHubRelease{{TagName: "v0.2.10"}, {TagName: "v0.2.11-rc.1"}, {TagName: "v0.2.11", Draft: true}, {TagName: "broken"}} {
		client := &upstreamCheckClient{release: release}
		svc := NewUpdateService(nil, client, "0.2.10-mg.2", "release")
		result := managedFields(t, svc, false)
		upstream, ok := result["upstream"].(map[string]any)
		require.True(t, ok)
		require.Equal(t, false, upstream["has_update"])
		if release.TagName == "v0.2.10" {
			require.Equal(t, "ok", upstream["status"])
		} else {
			require.Equal(t, "unavailable", upstream["status"])
		}
	}
}
func TestManagedUpstreamFailureIsUnknown(t *testing.T) {
	client := &upstreamCheckClient{err: errors.New("GitHub unavailable")}
	svc := NewUpdateService(nil, client, "0.2.10-mg.2", "release")
	result := managedFields(t, svc, false)
	upstream, ok := result["upstream"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "unavailable", upstream["status"])
	require.Equal(t, "", upstream["latest_version"])
}

func TestManagedUpdateMutationsAreBlockedBeforeIO(t *testing.T) {
	svc := NewUpdateService(nil, &managedForbiddenGitHubClient{}, "0.2.10-mg.1", "release")
	for name, operation := range map[string]func() error{
		"release assets":   func() error { return svc.applyReleaseAssets(context.Background(), nil) },
		"update":           func() error { return svc.PerformUpdate(context.Background()) },
		"local rollback":   svc.Rollback,
		"version rollback": func() error { return svc.RollbackToVersion(context.Background(), "0.1.1") },
		"rollback list":    func() error { _, err := svc.ListRollbackVersions(context.Background()); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			require.Error(t, err)
			code, status := infraerrors.ToHTTP(err)
			require.Equal(t, http.StatusConflict, code)
			require.Equal(t, "DEPLOYMENT_MANAGED", status.Reason)
		})
	}
}

func TestManagedUpstreamFailureRetainsLastSuccessfulResult(t *testing.T) {
	client := &upstreamCheckClient{release: &GitHubRelease{TagName: "v0.2.11", HTMLURL: "javascript:alert(1)"}}
	svc := NewUpdateService(nil, client, "0.2.10-mg.2", "release")
	before := managedFields(t, svc, false)["upstream"].(map[string]any)
	svc.upstream.lastAttempt = time.Now().Add(-6 * time.Minute)
	client.err = errors.New("GitHub rate limited")
	after := managedFields(t, svc, true)["upstream"].(map[string]any)
	require.Equal(t, "stale", after["status"])
	require.Equal(t, before["latest_version"], after["latest_version"])
	require.Equal(t, before["checked_at"], after["checked_at"])
	require.Equal(t, true, after["has_update"])
	require.Equal(t, 2, client.calls)
	managedFields(t, svc, false)
	require.Equal(t, 2, client.calls)
	require.Equal(t, "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.11", after["release_url"])
}
func TestManagedUpstreamSixHourCacheExpires(t *testing.T) {
	client := &upstreamCheckClient{release: &GitHubRelease{TagName: "v0.2.10"}}
	svc := NewUpdateService(nil, client, "0.2.10-mg.2", "release")
	managedFields(t, svc, false)
	svc.upstream.lastAttempt = time.Now().Add(-7 * time.Hour)
	client.release = &GitHubRelease{TagName: "v0.2.11"}
	after := managedFields(t, svc, false)["upstream"].(map[string]any)
	require.Equal(t, true, after["has_update"])
	require.Equal(t, 2, client.calls)
}

type blockingUpstreamClient struct {
	upstreamCheckClient
	started chan struct{}
	resume  chan struct{}
}

func (c *blockingUpstreamClient) FetchLatestRelease(_ context.Context, _ string) (*GitHubRelease, error) {
	c.calls++
	close(c.started)
	<-c.resume
	return c.release, nil
}
func TestManagedUpstreamConcurrentRequestsShareOneFetch(t *testing.T) {
	client := &blockingUpstreamClient{upstreamCheckClient: upstreamCheckClient{release: &GitHubRelease{TagName: "v0.2.11"}}, started: make(chan struct{}), resume: make(chan struct{})}
	svc := NewUpdateService(nil, client, "0.2.10-mg.2", "release")
	done := make(chan *UpdateInfo, 1)
	go func() { info, _ := svc.CheckUpdate(context.Background(), false); done <- info }()
	<-client.started
	secondDone := make(chan *UpdateInfo, 1)
	go func() { info, _ := svc.CheckUpdate(context.Background(), true); secondDone <- info }()
	var early *UpdateInfo
	select {
	case early = <-secondDone:
	case <-time.After(20 * time.Millisecond):
	}
	close(client.resume)
	first := <-done
	second := early
	if second == nil {
		second = <-secondDone
	}
	require.Equal(t, "ok", second.Upstream.Status)
	require.Equal(t, "ok", first.Upstream.Status)
	require.Equal(t, 1, client.calls)
}
