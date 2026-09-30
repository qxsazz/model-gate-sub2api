import { mount, RouterLinkStub } from '@vue/test-utils'
import { reactive, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import FloatingSupport from '../FloatingSupport.vue'

const { routeData, copyToClipboard } = vi.hoisted(() => ({
  routeData: { path: '/home', fullPath: '/home', meta: { requiresAuth: false } },
  copyToClipboard: vi.fn(),
}))
let route: typeof routeData
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/content/contactChannels', () => ({ contactChannels: [
  { id: 'group', kind: 'qq-group', labelKey: 'contact.qqGroup', value: '123456789' },
  { id: 'wechat', kind: 'wechat', labelKey: 'contact.wechat1', value: 'support_example' },
] }))
const mounted: ReturnType<typeof mount>[] = []
function mountSupport() {
  const wrapper = mount(FloatingSupport, { attachTo: document.body, global: { stubs: { RouterLink: RouterLinkStub, Icon: { template: '<span />' } } } })
  mounted.push(wrapper)
  return wrapper
}
const trigger = () => document.querySelector<HTMLButtonElement>('[data-testid="support-trigger"]')!
const panel = () => document.querySelector('[data-testid="support-panel"]')
beforeEach(() => { route = reactive({ ...routeData, meta: { requiresAuth: false } }); copyToClipboard.mockReset().mockResolvedValue(true) })
afterEach(() => { mounted.splice(0).forEach(w => w.unmount()); document.body.classList.remove('modal-open'); document.body.style.overflow = '' })

describe('floating support interaction', () => {
  it('starts collapsed, opens with focus, closes on Escape and restores focus', async () => {
    mountSupport()
    expect(panel()).toBeNull()
    trigger().click(); await nextTick(); await nextTick()
    expect(panel()).not.toBeNull()
    expect(panel()!.contains(document.activeElement)).toBe(true)
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(panel()).toBeNull()
    expect(document.activeElement).toBe(trigger())
  })
  it('closes on an outside pointer action and navigation', async () => {
    mountSupport(); trigger().click(); await nextTick()
    document.body.dispatchEvent(new Event('pointerdown', { bubbles: true })); await nextTick()
    expect(panel()).toBeNull()
    trigger().click(); await nextTick()
    route.fullPath = '/login'; route.path = '/login'; await nextTick()
    expect(panel()).toBeNull()
  })
  it('copies the selected channel and reports failure', async () => {
    mountSupport(); trigger().click(); await nextTick()
    copyToClipboard.mockResolvedValue(false)
    document.querySelector<HTMLButtonElement>('[data-testid="support-copy-wechat"]')!.click()
    await nextTick(); await nextTick()
    expect(copyToClipboard).toHaveBeenCalledWith('support_example')
    expect(document.querySelector('[role="status"]')!.textContent).toBe('contact.copyFailed')
  })
  it('hides on contact and admin routes while allowing user-console pages', async () => {
    mountSupport()
    for (const path of ['/contact', '/admin/users']) {
      route.path = path; route.fullPath = path; route.meta.requiresAuth = path.startsWith('/admin')
      await nextTick(); expect(document.querySelector('[data-testid="support-trigger"]')).toBeNull()
    }
    route.path = '/keys'; route.fullPath = '/keys'; route.meta.requiresAuth = true
    await nextTick(); expect(trigger()).not.toBeNull()
  })
  it('yields to an existing business dialog and does not reopen afterwards', async () => {
    mountSupport(); trigger().click(); await nextTick()
    document.body.classList.add('modal-open'); await nextTick(); await nextTick()
    expect(document.querySelector('[data-testid="support-trigger"]')).toBeNull()
    expect(panel()).toBeNull()
    document.body.classList.remove('modal-open'); await nextTick(); await nextTick()
    expect(trigger()).not.toBeNull(); expect(panel()).toBeNull()
  })
  it('yields to announcement overlays that lock body scrolling', async () => {
    mountSupport(); trigger().click(); await nextTick()
    document.body.style.overflow = 'hidden'; await nextTick(); await nextTick()
    expect(document.querySelector('[data-testid="support-trigger"]')).toBeNull()
    document.body.style.overflow = ''; await nextTick(); await nextTick()
    expect(trigger()).not.toBeNull(); expect(panel()).toBeNull()
  })
  it.each(['/payment/qrcode', '/payment/result', '/payment/stripe', '/payment/airwallex'])('shows support on public payment page %s', async path => {
    route.path = path; route.fullPath = path; route.meta.requiresAuth = false
    mountSupport(); await nextTick()
    expect(trigger()).not.toBeNull()
  })
  it('does not overlay the dedicated payment popup window', async () => {
    route.path = '/payment/stripe-popup'; route.fullPath = route.path; route.meta.requiresAuth = false
    mountSupport(); await nextTick()
    expect(document.querySelector('[data-testid="support-trigger"]')).toBeNull()
  })
})
