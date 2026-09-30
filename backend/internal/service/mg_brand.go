package service

import "strings"

// Defaults identify this fork without rewriting existing site settings.
const DefaultSiteName = "MODEL-GATE"
const ManagedDeploymentURL = "https://github.com/qxsazz/model-gate-sub2api/actions/workflows/deploy-production.yml"

func resolveSiteName(value string) string {
	if normalized := strings.TrimSpace(value); normalized != "" {
		return normalized
	}
	return DefaultSiteName
}
