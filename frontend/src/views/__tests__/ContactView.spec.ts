import { mount, RouterLinkStub } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ContactView from '../ContactView.vue'

const { channels, copyToClipboard } = vi.hoisted(() => ({
  channels: [] as Array<{ id: string; kind: string; labelKey: string; value: string }>,
  copyToClipboard: vi.fn(),
}))
vi.mock('@/content/contactChannels', () => ({ contactChannels: channels }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ siteName: 'Model-Gate', siteLogo: '', publicSettingsLoaded: true }),
  useAuthStore: () => ({ isAuthenticated: false, isAdmin: false }),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const mountContact = () => mount(ContactView, {
  global: { stubs: { RouterLink: RouterLinkStub, Icon: { template: '<span />' } } },
})

beforeEach(() => {
  channels.splice(0, channels.length,
    { id: 'group', kind: 'qq-group', labelKey: 'contact.qqGroup', value: '123456789' },
    { id: 'wechat', kind: 'wechat', labelKey: 'contact.wechat1', value: 'support_example' },
  )
  copyToClipboard.mockReset().mockResolvedValue(true)
})

describe('public contact channel selection', () => {
  it('shows multiple contact choices without requiring login', () => {
    const wrapper = mountContact()
    expect(wrapper.findAll('[data-testid="contact-card"]')).toHaveLength(2)
    expect(wrapper.text()).toContain('support_example')
    expect(wrapper.findAllComponents(RouterLinkStub).some(link => link.props('to') === '/home')).toBe(true)
  })

  it('copies the selected identifier rather than another contact', async () => {
    const wrapper = mountContact()
    await wrapper.get('[data-testid="copy-wechat"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith('support_example')
    expect(wrapper.get('[role="status"]').text()).toBe('contact.copied')
  })

  it('keeps identifiers selectable and reports clipboard failure', async () => {
    copyToClipboard.mockResolvedValue(false)
    const wrapper = mountContact()
    await wrapper.get('[data-testid="copy-group"]').trigger('click')
    expect(wrapper.get('[role="status"]').text()).toBe('contact.copyFailed')
    expect(wrapper.get('[data-testid="contact-value-group"]').text()).toBe('123456789')
  })

  it('hides unconfigured channels and never invents external contact links', () => {
    channels[0].value = ''
    const wrapper = mountContact()
    expect(wrapper.findAll('[data-testid="contact-card"]')).toHaveLength(1)
    expect(wrapper.find('a[href^="http"]').exists()).toBe(false)
    channels.splice(0)
    expect(mountContact().text()).toContain('contact.empty')
  })
})
