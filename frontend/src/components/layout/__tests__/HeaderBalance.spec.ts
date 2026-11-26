import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { WalletBalance } from '@/api/wallet'
import zh from '@/i18n/locales/zh/common'
import HeaderBalance from '../HeaderBalance.vue'

const getBalance = vi.hoisted(() => vi.fn())
vi.mock('@/api/wallet', () => ({ getWalletBalance: getBalance }))

const juneBalance: WalletBalance = {
  available_balance: 10530.444,
  frozen_balance: 4,
  other_available: 10517.84,
  checkin_credit: { available: 0.004, month: '2026-06', expires_at: '2026-06-30T16:00:00Z' },
  legacy_checkin_credit: { available: 12.60, next_amount: 3.60, next_expiry: '2026-06-25T03:00:00Z' },
}

function render() {
  const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh } })
  return mount(HeaderBalance, {
    props: { available: 10, frozen: 4 },
    global: { plugins: [i18n] },
    attachTo: document.body,
  })
}

enableAutoUnmount(afterEach)
beforeEach(() => getBalance.mockReset())

describe('header balance', () => {
  it('loads the atomic wallet split on open and keeps legacy and frozen amounts separate', async () => {
    getBalance.mockResolvedValue(juneBalance)
    const wrapper = render()
    const trigger = wrapper.get('.header-balance-trigger')
    expect(trigger.text()).toContain('$10.00')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(getBalance).not.toHaveBeenCalled()

    await trigger.trigger('click')
    await flushPromises()
    expect(getBalance).toHaveBeenCalledTimes(1)
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('.header-balance-heading strong').text()).toBe('$10530.444')
    const rows = wrapper.findAll('.header-balance-line')
    expect(rows[0].text()).toContain('6月签到余额')
    expect(rows[0].text()).toContain('优先扣除')
    expect(rows[0].text()).toContain('$0.004')
    expect(rows[0].text()).toContain('00:00')
    expect(rows[1].text()).toContain('历史签到余额')
    expect(rows[1].text()).toContain('$12.60')
    expect(rows[1].text()).toContain('$3.60')
    expect(rows[2].text()).toContain('$10517.84')
    expect(rows[3].text()).toContain('$4.00')

    await trigger.trigger('keydown', { key: 'Escape' })
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })

  it('refreshes the open breakdown when the Shanghai month switches', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    vi.setSystemTime(new Date('2026-06-30T15:59:59Z'))
    try {
      getBalance.mockResolvedValueOnce({ ...juneBalance, legacy_checkin_credit: null })
        .mockResolvedValueOnce({
          ...juneBalance,
          available_balance: 10517.84,
          checkin_credit: { available: 0, month: '2026-07', expires_at: '2026-07-31T16:00:00Z' },
          legacy_checkin_credit: null,
          frozen_balance: 0,
        })
      const wrapper = render()
      await wrapper.get('.header-balance-trigger').trigger('click')
      await flushPromises()
      await vi.advanceTimersByTimeAsync(1250)
      await flushPromises()

      expect(getBalance).toHaveBeenCalledTimes(2)
      expect(wrapper.findAll('.header-balance-line')).toHaveLength(2)
      expect(wrapper.get('.header-balance-credit').text()).toContain('7月签到余额')
      expect(wrapper.get('.header-balance-credit').text()).toContain('$0.00')
      expect(wrapper.get('.header-balance-heading strong').text()).toBe('$10517.84')
      wrapper.unmount()
    } finally {
      vi.useRealTimers()
    }
  })

  it('keeps keyboard focus on the trigger when a failed balance request is retried', async () => {
    getBalance.mockRejectedValueOnce(new Error('Temporary wallet error')).mockResolvedValueOnce(juneBalance)
    const wrapper = render()
    const trigger = wrapper.get<HTMLButtonElement>('.header-balance-trigger')
    await trigger.trigger('click')
    await flushPromises()
    const retry = wrapper.get<HTMLButtonElement>('.header-balance-retry')
    retry.element.focus()
    await retry.trigger('click')
    await flushPromises()

    expect(getBalance).toHaveBeenCalledTimes(2)
    expect(document.activeElement).toBe(trigger.element)
    await trigger.trigger('keydown', { key: 'Escape' })
    expect(trigger.attributes('aria-expanded')).toBe('false')
  })
})
