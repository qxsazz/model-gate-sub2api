import { mount, RouterLinkStub } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import DocsSidebar from '../DocsSidebar.vue'
import { docsNavigation } from '@/docs/registry'

describe('documentation category disclosure', () => {
  it('toggles categories and exposes the currently selected article', async () => {
    const wrapper = mount(DocsSidebar, { props: { navigation: docsNavigation, currentCategory: 'membership', currentPage: 'vip' }, global: { stubs: { RouterLink: RouterLinkStub, Transition: { props: ['name'], template: '<slot />' } } } })
    const button = wrapper.get('button[aria-label="充值与 VIP"]')
    expect(button.get('[data-nav-art]').attributes('data-icon')).toBe('creditCard')
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('[aria-current="page"]').text()).toBe('VIP 等级与权益')
    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find('[aria-current="page"]').exists()).toBe(false)
    await button.trigger('click')
    expect(wrapper.get('[aria-current="page"]').text()).toBe('VIP 等级与权益')
    await wrapper.setProps({ currentCategory: 'tools', currentPage: 'recommended' })
    expect(wrapper.get('button[aria-label="工具推荐"]').attributes('aria-expanded')).toBe('true')
  })
  it('automatically opens search results without losing the category controls', async () => {
    const wrapper = mount(DocsSidebar, { props: { navigation: docsNavigation }, global: { stubs: { RouterLink: RouterLinkStub, Transition: { props: ['name'], template: '<slot />' } } } })
    await wrapper.get('[aria-label="搜索文档"]').setValue('累充')
    expect(wrapper.get('button[aria-label="充值与 VIP"]').attributes('aria-expanded')).toBe('true')
    expect(wrapper.text()).toContain('累充奖励')
    expect(wrapper.text()).not.toContain('Cursor 接入')
  })
  it('finds error business codes in document content', async () => {
    const wrapper = mount(DocsSidebar, { props: { navigation: docsNavigation }, global: { stubs: { RouterLink: RouterLinkStub, Transition: { props: ['name'], template: '<slot />' } } } })
    await wrapper.get('[aria-label="搜索文档"]').setValue('gateway_queue_full')
    expect(wrapper.text()).toContain('HTTP 报错速查')
    expect(wrapper.get('button[aria-label="常见问题"]').attributes('aria-expanded')).toBe('true')
  })
})
