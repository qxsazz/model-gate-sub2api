import { describe, expect, it } from 'vitest'

import router from '@/router'

describe('documentation route', () => {
  it('provides an authenticated documentation route for the user sidebar', () => {
    const route = router.resolve('/guide?cat=clients&page=codex')
    expect(route.name).toBe('UserDocs')
    expect(route.meta.requiresAuth).toBe(true)
    expect(route.query.page).toBe('codex')
  })
  it('registers one public /docs route and keeps articles in query parameters', () => {
    const route = router.resolve('/docs?cat=clients&page=codex')

    expect(route.name).toBe('Docs')
    expect(route.meta.requiresAuth).toBe(false)
    expect(route.query).toEqual({ cat: 'clients', page: 'codex' })
    expect(
      router.getRoutes().filter((entry) => entry.path === '/docs')
    ).toHaveLength(1)
  })
})
