import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { authStore, appStore } = vi.hoisted(() => ({
  authStore: {
    checkAuth: vi.fn(),
    isAuthenticated: false,
    isAdmin: false,
    isSimpleMode: false,
    hasPendingAuthSession: false
  },
  appStore: {
    siteName: 'MODEL-GATE',
    backendModeEnabled: false,
    cachedPublicSettings: null
  }
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('@/stores/adminSettings', () => ({ useAdminSettingsStore: () => ({ customMenuItems: [] }) }))
vi.mock('@/stores/adminCompliance', () => ({ useAdminComplianceStore: () => ({}) }))
vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({ startNavigation: vi.fn(), endNavigation: vi.fn() })
}))
vi.mock('@/composables/useRoutePrefetch', () => ({ useRoutePrefetch: () => ({ triggerPrefetch: vi.fn() }) }))
vi.mock('@/views/auth/LoginView.vue', () => ({ default: { template: '<div>Login</div>' } }))
vi.mock('@/views/auth/MCPOAuthAuthorizeView.vue', () => ({ default: { template: '<div>MCP consent</div>' } }))
vi.mock('@/views/user/MCPView.vue', () => ({ default: { template: '<div>MCP connections</div>' } }))
vi.mock('@/router/title', () => ({ resolveRouteDocumentTitle: () => 'MCP OAuth' }))

let activeRouter: (typeof import('@/router'))['default'] | undefined
const authorizationPath = '/oauth/mcp/authorize?' + new URLSearchParams({
  response_type: 'code', client_id: 'desktop', redirect_uri: 'http://127.0.0.1:54321/callback?workspace=team',
  resource: 'https://api.example.com/api/v1/mcp', state: 'a+b&x=1/%#中文',
  code_challenge: 'a'.repeat(43), code_challenge_method: 'S256'
}).toString()

beforeEach(() => {
  vi.resetModules()
  vi.clearAllMocks()
  authStore.isAuthenticated = false
  authStore.isAdmin = false
  authStore.isSimpleMode = false
  authStore.hasPendingAuthSession = false
  appStore.backendModeEnabled = false
  window.history.replaceState({}, '', '/')
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
})

afterEach(() => {
  activeRouter?.options.history.destroy()
  activeRouter = undefined
  vi.restoreAllMocks()
})

async function getRouter() {
  const { default: router } = await import('@/router')
  activeRouter = router
  return router
}

describe('MCP OAuth route', () => {
  it.each([false, true])('requires login and preserves the entire authorization query with backend mode %s', async (backendModeEnabled) => {
    appStore.backendModeEnabled = backendModeEnabled
    const router = await getRouter()
    const record = router.getRoutes().find(route => route.name === 'MCPOAuthAuthorize')
    expect(record?.path).toBe('/oauth/mcp/authorize')
    expect(record?.meta.requiresAuth).toBe(true)
    expect(record?.meta.requiresAdmin).toBe(false)
    await router.push(authorizationPath)
    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.redirect).toBe(authorizationPath)
    expect(authStore.checkAuth).toHaveBeenCalledOnce()
  })

  it.each(['/settings/mcp', authorizationPath])('keeps ordinary users blocked from %s in backend mode', async (path) => {
    authStore.isAuthenticated = true
    appStore.backendModeEnabled = true
    const router = await getRouter()

    await router.push(path)

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.redirect).toBeUndefined()
  })

  it.each(['/settings/mcp', authorizationPath])('allows administrators to access %s in backend mode', async (path) => {
    authStore.isAuthenticated = true
    authStore.isAdmin = true
    appStore.backendModeEnabled = true
    const router = await getRouter()

    await router.push(path)

    expect(router.currentRoute.value.fullPath).toBe(path)
  })

  it('also requires authentication for the MCP connection settings page', async () => {
    const router = await getRouter()
    const record = router.getRoutes().find(route => route.name === 'MCP')
    expect(record?.meta.requiresAuth).toBe(true)
    expect(record?.meta.requiresAdmin).toBe(false)

    await router.push('/settings/mcp')

    expect(router.currentRoute.value.path).toBe('/login')
    expect(router.currentRoute.value.query.redirect).toBe('/settings/mcp')
  })
})
