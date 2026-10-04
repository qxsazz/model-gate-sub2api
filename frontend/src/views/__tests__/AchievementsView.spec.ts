import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import AchievementsView from '@/views/user/AchievementsView.vue'

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  change: vi.fn(),
  refresh: vi.fn(),
  previewCard: vi.fn(),
  useCard: vi.fn(),
}))
vi.mock('@/api/achievements', () => ({
  getAchievements: mocks.get,
  changeAchievement: mocks.change,
  previewAchievementCard: mocks.previewCard,
  useAchievementCard: mocks.useCard,
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
  it('uses the previewed historical tier and retries a lost response with the same card request', async () => {
    Object.defineProperty(HTMLDialogElement.prototype, 'showModal', {
      configurable: true,
      value: vi.fn(),
    })
    Object.defineProperty(HTMLDialogElement.prototype, 'close', {
      configurable: true,
      value: vi.fn(),
    })
    mocks.get.mockResolvedValue({
      ...state,
      card_balance: 1,
      card_min_date: '2026-09-03',
      card_max_date: '2026-10-02',
      calendar: ['2026-10-01'],
    })
    mocks.previewCard.mockResolvedValue({
      day: '2026-10-02',
      tier: 1,
      gross: 0.05,
      policy_known: true,
      existing: false,
      available: true,
      card_balance: 1,
      cash_reason: 'eligible',
    })
    mocks.useCard
      .mockRejectedValueOnce(new Error('网络中断'))
      .mockResolvedValue({
        cards_spent: 1,
        gross: 0.05,
        net: 0.05,
        offset_amount: 0,
        existing: false,
        card_balance: 0,
      })
    const w = render()
    await flushPromises()
    await w.get('[data-testid="calendar-day-2026-10-02"]').trigger('click')
    await flushPromises()
    const dialog = w.get('.card-dialog')
    expect(dialog.find('select').exists()).toBe(false)
    expect(mocks.previewCard).toHaveBeenLastCalledWith('2026-10-02')
    expect(
      w.get('[data-testid="calendar-day-2026-10-01"]').attributes('disabled'),
    ).toBeDefined()
    expect(dialog.text()).toContain('2026-10-02')
    await dialog
      .findAll('button')
      .find((b) => b.text() === '补签')!
      .trigger('click')
    await flushPromises()
    const first = mocks.useCard.mock.calls.at(-1)![0]
    expect(first).toEqual(
      expect.objectContaining({
        date: '2026-10-02',
        expected_tier: 1,
        expected_gross: 0.05,
      }),
    )
    expect(dialog.findAll('button').some((b) => b.text() === '重新核验')).toBe(
      false,
    )
    await dialog
      .findAll('button')
      .find((b) => b.text() === '重试补签')!
      .trigger('click')
    await flushPromises()
    expect(mocks.useCard.mock.calls.at(-1)![0].request_key).toBe(
      first.request_key,
    )
    expect(w.text()).toContain('补签成功')
  })
  it('navigates the calendar across a short month while only allowing the server date window', async () => {
    mocks.get.mockResolvedValue({
      ...state,
      date: '2026-03-01',
      card_balance: 1,
      card_min_date: '2026-01-30',
      card_max_date: '2026-02-28',
      calendar: ['2026-01-31'],
    })
    const w = render()
    await flushPromises()
    expect(
      w.get('[data-testid="calendar-day-2026-03-01"]').attributes('disabled'),
    ).toBeDefined()
    await w.get('[aria-label="上个月"]').trigger('click')
    expect(
      w.get('[data-testid="calendar-day-2026-02-28"]').attributes('disabled'),
    ).toBeUndefined()
    await w.get('[aria-label="上个月"]').trigger('click')
    expect(
      w.get('[data-testid="calendar-day-2026-01-30"]').attributes('disabled'),
    ).toBeUndefined()
    expect(
      w.get('[data-testid="calendar-day-2026-01-29"]').attributes('disabled'),
    ).toBeDefined()
    expect(
      w.get('[data-testid="calendar-day-2026-01-31"]').attributes('disabled'),
    ).toBeDefined()
    expect(w.get('[aria-label="上个月"]').attributes('disabled')).toBeDefined()
  })
  it('shows activity card rewards and separate series collection rewards', async () => {
    mocks.change.mockResolvedValue({
      cards_awarded: 1,
      card_balance: 3,
      gross: 0,
      net: 0,
      offset_amount: 0,
    })
    mocks.get.mockResolvedValue({
      ...state,
      card_balance: 2,
      series: [
        {
          key: 'A-K',
          name: '知识系列',
          reward: 1.4,
          collected: 1,
          total: 3,
          unlocked: false,
          claim: null,
        },
      ],
      medals: [
        {
          key: 'A-K01',
          category: 'activity',
          name: '初识星图',
          description: '知识主题',
          target: 1,
          reward: 0,
          card_reward: 1,
          progress: 1,
          unlocked: true,
          preview: false,
          claim: null,
          card_claim: null,
        },
      ],
    })
    const w = render()
    await flushPromises()
    expect(w.get('[data-testid="card-balance"]').text()).toContain('2')
    await w.findAll('.achievement-tabs button')[3].trigger('click')
    expect(w.text()).toContain('1 张补签卡')
    expect(w.text()).toContain('知识系列')
    expect(w.text()).toContain('$1.40')
    await w.get('[data-testid="claim-A-K01"]').trigger('click')
    await flushPromises()
    expect(mocks.change).toHaveBeenCalledWith(
      'claim',
      expect.objectContaining({ key: 'A-K01' }),
    )
    expect(w.text()).toContain('已领取 1 张补签卡')
  })
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
