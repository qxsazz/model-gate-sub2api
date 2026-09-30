//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
)

type brandSettingsRepo struct{ settingPublicRepoStub }

func (r *brandSettingsRepo) GetValue(_ context.Context, key string) (string, error) {
	return r.values[key], nil
}

func TestMGBrandDefaultsAndConfiguredIdentity(t *testing.T) {
	for _, value := range []string{"", "   ", "Custom Gateway", "Sub2API"} {
		t.Run(value, func(t *testing.T) {
			repo := &brandSettingsRepo{settingPublicRepoStub{values: map[string]string{SettingKeySiteName: value, SettingKeySiteLogo: "/uploads/custom.svg"}}}
			svc := NewSettingService(repo, &config.Config{})
			expected := value
			if value == "" || value == "   " {
				expected = "MODEL-GATE"
			}
			public, err := svc.GetPublicSettings(context.Background())
			require.NoError(t, err)
			require.Equal(t, expected, public.SiteName)
			require.Equal(t, "/uploads/custom.svg", public.SiteLogo)
			require.Equal(t, expected, svc.GetSiteName(context.Background()))
		})
	}
}
