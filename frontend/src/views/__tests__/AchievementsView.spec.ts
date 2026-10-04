import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import AchievementsView from '@/views/user/AchievementsView.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  change: vi.fn(),
  refresh: vi.fn(),
}))
vi.mock('@/api/achievements', () => ({
  getAchievements: mocks.get,
  changeAchievement: mocks.change,
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: mocks.refresh }),
}))

const state = {
  date: '2026-10-03',
  timezone: 'Asia/Shanghai',
  tier: 2,
  daily_amount: 0.1,
  daily_rewards: [0.01, 0.05, 0.1, 0.25, 0.5, 1],
  cash_reason: 'eligible',
  milestone_cash_enabled: true,
  today: null,
  streak: 0,
  longest: 0,
  total_days: 0,
  calendar: [],
  history: [],
  equipment: null,
  passes: [],
  medals: [],
}
function render() {
  return mount(AchievementsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}
describe('achievement checkin', () => {
  it('shows the current cash policy separately from an earlier zero-reward receipt', async () => {
    mocks.get.mockResolvedValue({
      ...state,
      cash_reason: 'eligible',
      today: {
        day: state.date,
        gross: 0,
        net: 0,
        offset_amount: 0,
        reason: 'cash_disabled',
        streak: 1,
      },
    })
    const w = render()
    await flushPromises()
    expect(w.get('[data-testid="current-cash-status"]').text()).toContain(
      '已开放',
    )
    expect(w.get('[data-testid="checkin-receipt-reason"]').text()).toContain(
      '签到时金额奖励尚未开启',
    )
    expect(w.text()).not.toContain('当前现金奖励未开放')
  })
  it('shows the administrator backfill source in readable Chinese', async () => {
    const receipt = {
      day: state.date,
      gross: 0.1,
      net: 0.1,
      offset_amount: 0,
      reason: 'admin_backfill',
      source: 'admin',
      streak: 1,
    }
    mocks.get.mockResolvedValue({
      ...state,
      today: receipt,
      history: [receipt],
    })
    const w = render()
    await flushPromises()
    expect(w.text()).toContain('管理员已补签')
    expect(w.text()).toContain('管理员补签')
    expect(w.text()).not.toContain('admin_backfill')
  })
  it('opens on checkin and uses server day for the real transaction', async () => {
    mocks.get.mockResolvedValue(state)
    mocks.change.mockResolvedValue({
      gross: 0.1,
      net: 0.06,
      offset_amount: 0.04,
    })
    const w = render()
    await flushPromises()
    expect(w.get('[data-testid="checkin"]').text()).toContain('签到')
    await w.get('[data-testid="checkin"]').trigger('click')
    await flushPromises()
    expect(mocks.change).toHaveBeenCalledWith(
      'checkin',
      expect.objectContaining({
        date: '2026-10-03',
        request_key: expect.any(String),
      }),
    )
    expect(w.text()).toContain('0.06')
    expect(w.text()).toContain('0.04')
  })
  it('shows a reload action when backend is unavailable', async () => {
    mocks.get.mockRejectedValue(new Error('连接失败'))
    const w = render()
    await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('连接失败')
    expect(w.text()).toContain('重试')
  })
  it('retries an uncertain cash response with the same request key', async () => {
    mocks.get.mockResolvedValue(state)
    mocks.change
      .mockRejectedValueOnce(new Error('网络中断'))
      .mockResolvedValue({ gross: 0, net: 0, offset_amount: 0 })
    const w = render()
    await flushPromises()
    await w.get('[data-testid="checkin"]').trigger('click')
    await flushPromises()
    const first = mocks.change.mock.calls.at(-1)![1].request_key
    await w.get('[data-testid="checkin"]').trigger('click')
    await flushPromises()
    expect(mocks.change.mock.calls.at(-1)![1].request_key).toBe(first)
  })
  it('shows the recorded no-cash reason after a budget-exhausted checkin', async () => {
    mocks.get.mockResolvedValue({
      ...state,
      today: {
        day: state.date,
        gross: 0,
        net: 0,
        offset_amount: 0,
        reason: 'budget_exhausted',
        streak: 1,
      },
    })
    const w = render()
    await flushPromises()
    expect(w.get('[data-testid="checkin-receipt-reason"]').text()).toContain(
      '本次签到时奖励额度不足',
    )
    expect(
      w.get('[data-testid="checkin"]').attributes('disabled'),
    ).toBeDefined()
    expect(w.text()).not.toContain('今日现金奖励已开放，签到后即时入账')
  })
})
