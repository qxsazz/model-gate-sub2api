import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import HeaderCheckin from '../HeaderCheckin.vue'

const mocks = vi.hoisted(() => ({
  getStatus: vi.fn(),
  change: vi.fn(),
  refreshUser: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/achievements', () => ({
  getAchievementCheckinStatus: mocks.getStatus,
  changeAchievement: mocks.change,
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess }),
  useAuthStore: () => ({ user: { id: 9 }, refreshUser: mocks.refreshUser }),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const status = {
  date: '2026-11-25',
  checked_in: false,
  minimum_recharge_cny: 10,
  qualified_recharge_cny: 10,
  remaining_recharge_cny: 0,
  access_unlocked: true,
  access_reason: 'eligible',
}
const receipt = { day: status.date, gross: 0.1, net: 0.1, offset_amount: 0 }
const selector = '[data-testid="header-checkin"]'

function render() {
  return mount(HeaderCheckin, { props: { userId: 9 } })
}

enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  mocks.getStatus.mockResolvedValue(status)
  mocks.refreshUser.mockResolvedValue(undefined)
})

describe('header daily check-in', () => {
  it('submits one request on repeated clicks and confirms a check-in even if balance refresh fails', async () => {
    let finish!: (value: typeof receipt) => void
    mocks.change.mockReturnValue(new Promise<typeof receipt>((resolve) => { finish = resolve }))
    mocks.refreshUser.mockRejectedValueOnce(new Error('Balance refresh unavailable'))
    const wrapper = render()
    await flushPromises()

    const button = wrapper.get(selector)
    expect(button.text()).toBe('nav.checkin.action')
    expect(button.find('.checkin-dot').exists()).toBe(true)
    await button.trigger('click')
    await button.trigger('click')
    expect(mocks.change).toHaveBeenCalledTimes(1)
    expect(mocks.change).toHaveBeenCalledWith('checkin', {
      date: status.date,
      request_key: expect.any(String),
    })

    finish(receipt)
    await flushPromises()
    expect(button.text()).toBe('nav.checkin.done')
    expect(button.find('.checkin-dot').exists()).toBe(false)
    expect(mocks.showSuccess).toHaveBeenCalledWith('nav.checkin.success')
    expect(mocks.showError).not.toHaveBeenCalled()
  })

  it('preserves the request key when retrying after a lost response', async () => {
    mocks.change
      .mockRejectedValueOnce(new Error('Connection lost'))
      .mockResolvedValueOnce({ ...receipt, replayed: true })
    const wrapper = render()
    await flushPromises()

    await wrapper.get(selector).trigger('click')
    await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith('Connection lost')
    expect(wrapper.get(selector).text()).toBe('nav.checkin.action')

    await wrapper.get(selector).trigger('click')
    await flushPromises()
    expect(mocks.change).toHaveBeenCalledTimes(2)
    expect(mocks.change.mock.lastCall).toEqual(mocks.change.mock.calls[0])
    expect(wrapper.get(selector).text()).toBe('nav.checkin.done')
  })

  it('hides a blocked reminder and learns a completed check-in when the window regains focus', async () => {
    mocks.getStatus
      .mockResolvedValueOnce({
        ...status,
        access_reason: 'recharge_required',
        access_unlocked: false,
        qualified_recharge_cny: 9,
        remaining_recharge_cny: 1,
      })
      .mockResolvedValueOnce({ ...status, checked_in: true })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find(selector).exists()).toBe(false)

    window.dispatchEvent(new Event('focus'))
    await flushPromises()
    expect(wrapper.get(selector).text()).toBe('nav.checkin.done')
    expect(wrapper.find('.checkin-dot').exists()).toBe(false)
    expect(mocks.change).not.toHaveBeenCalled()
  })
})
