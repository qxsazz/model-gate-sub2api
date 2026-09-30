//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

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

func TestManagedUpdateCheckIgnoresUpstreamCacheAndNetwork(t *testing.T) {
	svc := NewUpdateService(&updateServiceCacheStub{data: `{"current_version":"0.2.8","latest_version":"9.0.0","has_update":true}`}, &managedForbiddenGitHubClient{}, "0.2.10-mg.1", "release")
	for _, force := range []bool{false, true} {
		info, err := svc.CheckUpdate(context.Background(), force)
		require.NoError(t, err)
		require.Equal(t, "0.2.10-mg.1", info.CurrentVersion)
		require.False(t, info.HasUpdate)
		encoded, err := json.Marshal(info)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(encoded, &fields))
		require.Equal(t, "managed", fields["deployment_mode"])
		require.Equal(t, "https://github.com/qxsazz/model-gate-sub2api/actions/workflows/deploy-production.yml", fields["deployment_url"])
	}
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
