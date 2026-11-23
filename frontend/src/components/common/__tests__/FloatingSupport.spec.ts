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
function mockDragBounds(button: HTMLButtonElement) {
  vi.spyOn(button.closest('.support-bounds')!, 'getBoundingClientRect').mockReturnValue(new DOMRect(8, 8, 380, 600))
  vi.spyOn(button, 'getBoundingClientRect').mockReturnValue(new DOMRect(280, 540, 100, 50))
  Object.assign(button, {
    setPointerCapture: vi.fn(), hasPointerCapture: () => true, releasePointerCapture: vi.fn(),
  })
}
function pointer(button: HTMLButtonElement, type: string, x: number, y: number, pointerType = 'mouse') {
  const event = new MouseEvent(type, { clientX: x, clientY: y, bubbles: true })
  Object.defineProperties(event, { pointerId: { value: 1 }, isPrimary: { value: true }, pointerType: { value: pointerType } })
  button.dispatchEvent(event)
}
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
  it('distinguishes small tap movements from a drag and keeps keyboard activation', async () => {
    mountSupport()
    const button = trigger()
    const widget = button.closest<HTMLElement>('.floating-support')!
    mockDragBounds(button)

    pointer(button, 'pointerdown', 300, 550)
    pointer(button, 'pointermove', 302, 552)
    pointer(button, 'pointerup', 302, 552)
    button.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))
    await nextTick()
    expect(panel()).not.toBeNull()
    expect(widget.classList.contains('support-moved')).toBe(false)

    button.click()
    pointer(button, 'pointerdown', 300, 550)
    pointer(button, 'pointermove', -500, -500)
    pointer(button, 'pointerup', -500, -500)
    button.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))
    await nextTick()
    expect(panel()).toBeNull()
    expect(widget.style.getPropertyValue('--support-x')).toBe('0px')
    expect(widget.style.getPropertyValue('--support-y')).toBe('0px')
    expect(widget.classList.contains('support-panel-below')).toBe(true)
    expect(button.releasePointerCapture).toHaveBeenCalledWith(1)
    button.click()
    await nextTick()
    expect(panel()).not.toBeNull()
  })
  it.each(['pointercancel', 'lostpointercapture'])('caps a drag at the bottom right and ends it on %s', async end => {
    mountSupport()
    const button = trigger()
    mockDragBounds(button)
    const widget = button.closest<HTMLElement>('.floating-support')!
    pointer(button, 'pointerdown', 300, 550)
    pointer(button, 'pointermove', 1000, 1000)
    pointer(button, end, 1000, 1000)
    pointer(button, 'pointermove', 0, 0)
    await nextTick()
    expect(widget.style.getPropertyValue('--support-x')).toBe('280px')
    expect(widget.style.getPropertyValue('--support-y')).toBe('550px')
    expect(widget.classList.contains('support-panel-below')).toBe(false)
    pointer(button, 'pointerdown', 300, 550)
    pointer(button, 'pointerup', 300, 550)
    button.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))
    await nextTick()
    expect(panel()).not.toBeNull()
  })
  it.each([true, false])('activates a touch tap once after a drag (compatibility click %s)', async sendsClick => {
    mountSupport()
    const button = trigger()
    mockDragBounds(button)
    pointer(button, 'pointerdown', 300, 550, 'touch')
    pointer(button, 'pointermove', 100, 100, 'touch')
    pointer(button, 'pointerup', 100, 100, 'touch')
    await nextTick()
    expect(panel()).toBeNull()
    for (const expanded of [true, false]) {
      pointer(button, 'pointerdown', 100, 100, 'touch')
      pointer(button, 'pointerup', 100, 100, 'touch')
      if (sendsClick) button.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))
      await nextTick()
      expect(button.getAttribute('aria-expanded')).toBe(String(expanded))
    }
    button.click() // Keyboard and assistive activation have no pointer click detail.
    await nextTick()
    expect(panel()).not.toBeNull()
  })
  it.each(['pointercancel', 'lostpointercapture'])('does not activate a canceled touch tap on %s', async end => {
    mountSupport()
    const button = trigger()
    mockDragBounds(button)
    pointer(button, 'pointerdown', 300, 550, 'touch')
    pointer(button, end, 300, 550, 'touch')
    pointer(button, 'pointerup', 300, 550, 'touch')
    await nextTick()
    expect(panel()).toBeNull()
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
