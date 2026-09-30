/** Defaults for the MG fork. Configured site identity always takes precedence. */
export const MG_BRAND = {
  name: 'MODEL-GATE',
  logo: '/model-gate-mg-luxury.svg',
  repositoryUrl: 'https://github.com/qxsazz/model-gate-sub2api',
  deploymentUrl: 'https://github.com/qxsazz/model-gate-sub2api/actions/workflows/deploy-production.yml',
} as const

export function resolveSiteName(value: unknown): string {
  return typeof value === 'string' && value.trim() ? value.trim() : MG_BRAND.name
}
