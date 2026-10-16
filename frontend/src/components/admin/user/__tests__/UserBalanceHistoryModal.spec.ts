import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { AdminUser } from '@/types'
import UserBalanceHistoryModal from '../UserBalanceHistoryModal.vue'

const mocks = vi.hoisted(() => ({ getUserBalanceHistory: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/utils/format', () => ({ formatDateTime: () => 'date' }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
function result(id: number) {
  return { items: [{ id, type: 'admin_balance', value: id, notes: `History ${id}` }], total: 1, total_recharged: id }
}
function deferred() {
  let resolve!: (value: ReturnType<typeof result>) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<ReturnType<typeof result>>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}
async function openDialog() {
  const wrapper = mount(UserBalanceHistoryModal, {
    props: { show: false, user: { id: 1, email: 'one@example.com', balance: 1 } as AdminUser },
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, Icon: true, Select: true } }
  })
  await wrapper.setProps({ show: true })
  return wrapper
}

describe('UserBalanceHistoryModal request ordering', () => {
  it('keeps the new user history when an old response finishes later', async () => {
    const old = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockResolvedValueOnce(result(20))
    const wrapper = await openDialog()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, user: { id: 2, email: 'two@example.com', balance: 2 } as AdminUser })
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.text()).toContain('History 20')
    expect(wrapper.text()).not.toContain('History 10')
    expect(wrapper.text()).toContain('$20.00')
  })

  it('does not end the current filter loading state when an old request finishes', async () => {
    const old = deferred()
    const current = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const wrapper = await openDialog()
    wrapper.findComponent({ name: 'Select' }).vm.$emit('change', 'balance')
    await flushPromises()
    old.resolve(result(10))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('History 10')
    current.resolve(result(20))
    await flushPromises()
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
    expect(wrapper.text()).toContain('History 20')
  })

  it.each(['close', 'unmount'])('ignores failures after %s', async (action) => {
    const pending = deferred()
    mocks.getUserBalanceHistory.mockReturnValueOnce(pending.promise)
    const wrapper = await openDialog()
    if (action === 'close') await wrapper.setProps({ show: false })
    else wrapper.unmount()
    pending.reject(new Error('Stale error'))
    await flushPromises()
    expect(console.error).not.toHaveBeenCalled()
  })

  it('still reports current failures and ends loading', async () => {
    const error = new Error('Current error')
    mocks.getUserBalanceHistory.mockRejectedValueOnce(error)
    const wrapper = await openDialog()
    await flushPromises()
    expect(console.error).toHaveBeenCalledWith('Failed to load balance history:', error)
    expect(wrapper.find('svg.animate-spin').exists()).toBe(false)
  })
})

describe('reward balance history', () => {
  it('shows distinct reward sources, full reference codes and precise amounts alongside existing recharge', async () => {
    const notes = '每日签到 · 2026-10-16 · VIP1；奖励 $0.01；实际入账 $0.005；抵扣奖励欠款 $0.005'
    mocks.getUserBalanceHistory.mockResolvedValue({
      items: [
        { id: 1, code: 'PAY-existing', type: 'balance', value: 10, notes: '' },
        { id: -1, code: 'SIGN-42-20261016', type: 'checkin_balance', value: .005, notes },
        { id: -2, code: 'ACH-42-T01', type: 'achievement_balance', value: .1, notes: '成就奖励 · 星火' },
        { id: -3, code: 'SERIES-42-A-K', type: 'achievement_series_balance', value: 1.4, notes: '系列成就奖励 · 知识系列' }
      ], total: 4, total_recharged: 10
    })
    const wrapper = await openDialog()
    await flushPromises()
    for (const key of ['balanceAddedRedeem', 'balanceAddedCheckin', 'balanceAddedAchievement', 'balanceAddedAchievementSeries']) {
      expect(wrapper.text()).toContain(`redeem.${key}`)
    }
    for (const code of ['SIGN-42-20261016', 'ACH-42-T01', 'SERIES-42-A-K']) expect(wrapper.text()).toContain(code)
    expect(wrapper.text()).toContain(notes)
    expect(wrapper.text()).toContain('+$0.005')
    expect(wrapper.text()).toContain('+$0.10')
    const select = wrapper.findComponent({ name: 'Select' })
    select.vm.$emit('update:modelValue', 'checkin_balance')
    await wrapper.vm.$nextTick()
    select.vm.$emit('change')
    await flushPromises()
    expect(mocks.getUserBalanceHistory).toHaveBeenLastCalledWith(1, 1, 15, 'checkin_balance')
  })
})
