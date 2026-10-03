import { mount, flushPromises, RouterLinkStub } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { describe, it, expect, vi } from 'vitest'
import UserDocsView from '../UserDocsView.vue'

async function render(path = '/guide') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/guide', component: UserDocsView }]
  })
  await router.push(path)
  await router.isReady()
  vi.stubGlobal('scrollTo', vi.fn())
  const w = mount(UserDocsView, {
    global: {
      plugins: [router],
      stubs: {
        AppLayout: { template: '<main data-workspace><slot/></main>' },
        RouterLink: RouterLinkStub,
        teleport: true
      }
    }
  })
  await flushPromises()
  return { w, router }
}
describe('signed-in documentation', () => {
  it('reuses the public content inside the workspace and keeps article navigation inside it', async () => {
    const { w, router } = await render()
    expect(w.find('[data-workspace]').exists()).toBe(true)
    expect(w.get('[data-testid="docs-overview"]').text()).toContain('文档总览')
    const link = w
      .findAllComponents(RouterLinkStub)
      .find((l) => l.attributes('data-overview-category') === 'membership')!
    expect(link.props('to')).toEqual({
      path: '/guide',
      query: { cat: 'membership', page: 'recharge' }
    })
    await router.push('/guide?cat=tutorial&page=api-key')
    await flushPromises()
    expect(w.get('[data-testid="docs-article-title"]').text()).toBe('API Key')
    const navigation = w
      .findAllComponents(RouterLinkStub)
      .filter((l) => l.attributes('class')?.includes('docs-article-nav__card'))
    expect(navigation.length).toBeGreaterThan(0)
    for (const n of navigation) expect(n.props('to').path).toBe('/guide')
    w.unmount()
  })
  it('follows the workspace theme when the sidebar toggles it', async () => {
    const original = document.documentElement.classList.contains('dark')
    document.documentElement.classList.remove('dark')
    const { w } = await render()
    expect(w.get('[data-testid="docs-shell"]').classes()).not.toContain(
      'is-dark'
    )
    document.documentElement.classList.add('dark')
    await flushPromises()
    expect(w.get('[data-testid="docs-shell"]').classes()).toContain('is-dark')
    w.unmount()
    document.documentElement.classList.toggle('dark', original)
  })
})
