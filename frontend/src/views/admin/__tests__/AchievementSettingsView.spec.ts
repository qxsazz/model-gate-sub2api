import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, it, expect, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import AchievementSettingsView from '../AchievementSettingsView.vue'
const m = vi.hoisted(() => ({
  config: vi.fn(),
  save: vi.fn(),
  user: vi.fn(),
  preview: vi.fn(),
  action: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}))
vi.mock('@/api/achievements', () => ({
  getAchievementConfig: m.config,
  saveAchievementConfig: m.save,
  getAdminAchievements: m.user,
  previewAchievementBackfill: m.preview,
  adminAchievementAction: m.action,
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: m.success, showError: m.error }),
}))
const state = {
  date: '2026-10-03',
  timezone: 'Asia/Shanghai',
  tier: 0,
  daily_amount: 0.01,
  daily_rewards: [0.01, 0.05, 0.1, 0.25, 0.5, 1],
  cash_reason: 'eligible',
  milestone_cash_enabled: true,
  today: null,
  total_days: 1,
  streak: 1,
  longest: 1,
  calendar: [],
  history: [],
  equipment: null,
  passes: [],
  medals: [
    {
      key: 'T01',
      name: '星火',
      category: 'token',
      target: 1000000,
      progress: 0,
      reward: 0.1,
      preview: false,
      unlocked: false,
      claim: null,
    },
  ],
  user: {
    id: 11,
    email: 'qa@example.invalid',
    username: '测试用户',
    balance: 1,
  },
}
function render() {
  return mount(AchievementSettingsView, {
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh: { common: { confirm: '确认', cancel: '取消' } } },
          missingWarn: false,
        }),
      ],
      stubs: {
        AppLayout: { template: '<main><slot/></main>' },
        Icon: true,
        RouterLink: { template: '<a><slot/></a>' },
        ConfirmDialog: {
          props: ['show'],
          template:
            '<div v-if="show" data-confirm><button @click="$emit(\'confirm\')">confirm</button><button @click="$emit(\'cancel\')">cancel</button></div>',
        },
      },
    },
  })
}
describe('achievement administration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    m.config.mockResolvedValue({
      daily_rewards: state.daily_rewards,
      cash_enabled: true,
      milestone_cash_enabled: true,
      cash_scope: 'all',
      cash_allowlist: [],
      budget_enabled: false,
      daily_budget: 10,
      monthly_budget: 100,
    })
    m.user.mockResolvedValue(state)
    m.preview.mockResolvedValue({
      day: '2026-10-02',
      tier: 2,
      gross: 0.1,
      existing: false,
      policy_known: true,
    })
    m.action.mockResolvedValue({
      saved: true,
      gross: 0.1,
      net: 0.1,
      offset_amount: 0,
    })
  })
  it('uses the management tabs and standard input controls with immediate reward wording', async () => {
    const w = render()
    await flushPromises()
    expect(w.findAll('[role="tab"]')).toHaveLength(3)
    expect(w.text()).toContain('即时到账')
    expect(w.find('input[type="checkbox"]').exists()).toBe(false)
    expect(w.findAll('input.input').length).toBeGreaterThan(0)
  })
  it('previews the historical payout and confirms the exact user and day before backfill', async () => {
    const w = render()
    await flushPromises()
    await w.get('[data-tab="users"]').trigger('click')
    await w.get('[data-user-id]').setValue('11')
    await w.get('form').trigger('submit')
    await flushPromises()
    await w.get('[data-backfill-date]').setValue('2026-10-02')
    await w.get('[data-operation-reason]').setValue('核对漏签')
    await w.get('[data-preview-backfill]').trigger('click')
    await flushPromises()
    expect(m.preview).toHaveBeenCalledWith(11, '2026-10-02')
    expect(w.text()).toContain('历史 VIP 2')
    await w.get('[data-backfill]').trigger('click')
    expect(m.action).not.toHaveBeenCalled()
    await w.get('[data-confirm] button').trigger('click')
    await flushPromises()
    expect(m.action).toHaveBeenCalledWith(
      11,
      'backfill',
      expect.objectContaining({
        date: '2026-10-02',
        reason: '核对漏签',
        request_key: expect.any(String),
      }),
    )
  })
  it('does not mutate a different user after changing the selected ID', async () => {
    const w = render()
    await flushPromises()
    await w.get('[data-tab="users"]').trigger('click')
    await w.get('[data-user-id]').setValue('11')
    await w.get('form').trigger('submit')
    await flushPromises()
    await w.get('[data-operation-reason]').setValue('核验后授予')
    expect(w.get('[data-grant="T01"]').attributes('disabled')).toBeUndefined()
    await w.get('[data-user-id]').setValue('12')
    expect(w.get('[data-grant="T01"]').attributes('disabled')).toBeDefined()
  })
})
