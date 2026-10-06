import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DocsView from '../DocsView.vue'

async function mountDocs(path = '/docs') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/docs', component: DocsView }],
  })
  await router.push(path)
  await router.isReady()

  const wrapper = mount(DocsView, {
    global: {
      plugins: [router],
      stubs: { RouterLink: RouterLinkStub, teleport: true },
    },
  })
  await flushPromises()
  return { wrapper, router }
}

describe('DocsView', () => {
  beforeEach(() => {
    vi.stubGlobal('scrollTo', vi.fn())
  })

  it('shows the overview at /docs and links categories to real articles', async () => {
    const { wrapper, router } = await mountDocs()
    expect(wrapper.get('[data-testid="docs-overview"]').text()).toContain('文档总览')
    expect(wrapper.get('[data-overview-emblem]').attributes('data-icon')).toBe('grid')
    expect(wrapper.findAll('[data-overview-category]')).toHaveLength(8)
    expect(wrapper.findAll('[data-category-art]')).toHaveLength(8)
    expect(new Set(wrapper.findAll('[data-category-art]').map(el => el.attributes('data-icon'))).size).toBe(8)
    const entry = wrapper.findAllComponents(RouterLinkStub).find(link => link.attributes('data-overview-category') === 'membership')
    expect(entry?.props('to')).toEqual({ path: '/docs', query: { cat: 'membership', page: 'recharge' } })
    await router.push(entry!.props('to')); await flushPromises()
    expect(wrapper.get('[data-testid="docs-article-title"]').text()).toBe('充值与兑换')
    expect(wrapper.find('[data-testid="docs-overview"]').exists()).toBe(false)
  })

  it('preserves the quick-start article URL', async () => {
    const { wrapper } = await mountDocs('/docs?cat=tutorial&page=quick-start')

    expect(wrapper.get('[data-testid="docs-article-title"]').text()).toBe('快速开始')
    expect(wrapper.get('[data-testid="docs-article"]').text()).toContain('首次 API 调用')
  })

  it('reacts to query changes without remounting the documentation shell', async () => {
    const { wrapper, router } = await mountDocs('/docs?cat=tutorial&page=api-key')
    const shell = wrapper.get('[data-testid="docs-shell"]').element

    expect(wrapper.get('[data-testid="docs-article-title"]').text()).toBe('API Key')
    await router.push('/docs?cat=clients&page=codex')
    await flushPromises()

    expect(wrapper.get('[data-testid="docs-shell"]').element).toBe(shell)
    expect(wrapper.get('[data-testid="docs-article-title"]').text()).toBe('Codex 接入')
    expect(wrapper.get('[aria-current="page"]').text()).toBe('Codex 接入')
  })

  it('shows a documentation-specific not-found state for invalid parameters', async () => {
    const { wrapper } = await mountDocs('/docs?cat=tutorial&page=../../private')

    expect(wrapper.get('[data-testid="docs-not-found"]').text()).toContain('没有找到这篇文档')
    const defaultLink = wrapper
      .findAllComponents(RouterLinkStub)
      .find((link) => link.attributes('data-testid') === 'docs-default-link')
    expect(defaultLink?.props('to')).toBe('/docs')
  })

  it('opens and closes the mobile navigation drawer', async () => {
    const { wrapper } = await mountDocs('/docs?cat=tutorial&page=quick-start')

    expect(wrapper.find('[data-testid="docs-mobile-drawer"]').exists()).toBe(false)
    await wrapper.get('[data-testid="docs-menu-button"]').trigger('click')
    expect(wrapper.get('[data-testid="docs-mobile-drawer"]').isVisible()).toBe(true)
    await wrapper.get('[data-testid="docs-drawer-close"]').trigger('click')
    expect(wrapper.find('[data-testid="docs-mobile-drawer"]').exists()).toBe(false)
  })

  it('copies only the selected code block', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })
    const { wrapper } = await mountDocs('/docs?cat=tutorial&page=quick-start')

    await wrapper.get('[data-copy-code]').trigger('click')

    expect(writeText).toHaveBeenCalledOnce()
    expect(writeText.mock.calls[0]?.[0]).toContain('MODEL_GATE_API_KEY')
    expect(writeText.mock.calls[0]?.[0]).not.toContain('curl https://model-gate.cc')
  })

  it('searches navigation and shows the available staging achievement documentation', async () => {
    const { wrapper, router } = await mountDocs('/docs?cat=achievements&page=overview')
    expect(wrapper.find('[data-testid="docs-coming-soon"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="docs-article"]').text()).toContain('21 枚')
    expect(wrapper.get('[data-testid="docs-article"]').text()).toContain('测试环境')
    await wrapper.get('[aria-label="搜索文档"]').setValue('累充')
    expect(wrapper.get('[aria-label="文档目录"]').text()).toContain('累充奖励')
    expect(wrapper.get('[aria-label="文档目录"]').text()).not.toContain('Cursor 接入')
    await router.push('/docs?cat=membership&page=vip'); await flushPromises()
    expect(wrapper.find('[data-testid="docs-coming-soon"]').exists()).toBe(false)
  })
})
