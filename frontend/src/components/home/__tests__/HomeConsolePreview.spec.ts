import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import HomeConsolePreview from '../HomeConsolePreview.vue'

const { getModelPlaza } = vi.hoisted(() => ({ getModelPlaza: vi.fn() }))
vi.mock('@/api/modelPlaza', () => ({ getModelPlaza }))
vi.mock('vue-i18n', async () => ({ ...await vi.importActual('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({ Doughnut: { template: '<canvas />' }, Line: { template: '<canvas />' } }))

function render() {
  return mount(HomeConsolePreview, {
    props: { isDark: false, siteName: 'Model-Gate', siteLogo: '/logo.svg', active: true },
    global: { plugins: [createPinia()], stubs: { RouterLink: { template: '<a><slot /></a>' }, GroupBadge: true, PlatformIcon: true } }
  })
}

afterEach(() => vi.clearAllMocks())

describe('HomeConsolePreview', () => {
  it('scopes the preview palette to its reactive theme', async () => {
    const wrapper = render()
    expect(wrapper.classes()).not.toContain('is-dark')
    await wrapper.setProps({ isDark: true })
    expect(wrapper.classes()).toContain('is-dark')
    await wrapper.setProps({ isDark: false })
    expect(wrapper.classes()).not.toContain('is-dark')
    wrapper.unmount()
  })
  it('only exposes four preview tabs and labels demo data', () => {
    const wrapper = render()
    expect(wrapper.findAll('[role="tab"]').map(tab => tab.text())).toEqual(['仪表盘', 'API 密钥', '使用记录', '可用渠道'])
    expect(wrapper.text()).toContain('演示数据')
    expect(wrapper.text()).not.toContain('dashboard.platformBreakdown')
    expect(getModelPlaza).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('loads plaza groups, searches models and refreshes without static fallback rows', async () => {
    getModelPlaza.mockResolvedValue({ description: '', groups: [{ id: 81, name: 'Live group', description: 'Live description', platform: 'openai', subscription_type: 'standard', rate_multiplier: 0.27, is_exclusive: false, models: [{ name: 'live-model-unique', platform: 'openai', pricing: null }] }] })
    const wrapper = render()
    await wrapper.get('[data-tab="channels"]').trigger('click')
    await flushPromises()
    expect(getModelPlaza).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Live group')
    expect(wrapper.text()).toContain('live-model-unique')
    await wrapper.get('input[aria-label="搜索渠道或模型"]').setValue('absent-model')
    expect(wrapper.text()).not.toContain('live-model-unique')
    await wrapper.get('input[aria-label="搜索渠道或模型"]').setValue('live-model')
    expect(wrapper.text()).toContain('live-model-unique')
    getModelPlaza.mockResolvedValue({ description: '', groups: [] })
    await wrapper.get('button[aria-label="刷新渠道"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('live-model-unique')
    expect(wrapper.text()).toContain('暂无可展示的渠道')
    wrapper.unmount()
  })

  it('shows a retryable error instead of invented channels', async () => {
    getModelPlaza.mockRejectedValue(new Error('unavailable'))
    const wrapper = render()
    await wrapper.get('[data-tab="channels"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('加载失败')
    expect(wrapper.text()).not.toContain('claude-kiro')
    wrapper.unmount()
  })

  it('does not expose exclusive groups from a signed-in plaza response', async () => {
    getModelPlaza.mockResolvedValue({ groups: [{ id: 9, name: 'PRIVATE_ACCOUNT_GROUP', platform: 'openai', is_exclusive: true, models: [{ name: 'private-model' }] }] })
    const wrapper = render()
    await wrapper.get('[data-tab="channels"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('PRIVATE_ACCOUNT_GROUP')
    expect(wrapper.text()).not.toContain('private-model')
    expect(wrapper.text()).toContain('暂无可展示的渠道')
    wrapper.unmount()
  })

  it('waits for the preview to become visible and aborts requests on unmount', async () => {
    getModelPlaza.mockReturnValue(new Promise(() => {}))
    const wrapper = render()
    await wrapper.setProps({ active: false })
    await wrapper.get('[data-tab="channels"]').trigger('click')
    expect(getModelPlaza).not.toHaveBeenCalled()
    await wrapper.setProps({ active: true })
    expect(getModelPlaza).toHaveBeenCalledTimes(1)
    const signal = getModelPlaza.mock.calls[0][0].signal as AbortSignal
    expect(signal.aborted).toBe(false)
    wrapper.unmount()
    expect(signal.aborted).toBe(true)
  })

  it('keeps key actions in a local demo and supports searching', async () => {
    const wrapper = render()
    await wrapper.get('[data-tab="apikeys"]').trigger('click')
    expect(wrapper.text()).toContain('sk-demo-')
    await wrapper.get('button[data-action="create-key"]').trigger('click')
    expect(wrapper.get('[role="status"]').text()).toContain('不会创建真实密钥')
    await wrapper.get('input[aria-label="搜索密钥"]').setValue('no-such-key')
    expect(wrapper.text()).toContain('没有匹配的密钥')
    expect(getModelPlaza).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
