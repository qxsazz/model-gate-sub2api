import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import VIPRulesView from '../VIPRulesView.vue'
import { createI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'

const mocks = vi.hoisted(() => ({
  rules: vi.fn(),
  groups: vi.fn(),
  user: vi.fn(),
  save: vi.fn(),
  override: vi.fn(),
  restore: vi.fn(),
  level: vi.fn(),
  restoreLevel: vi.fn(),
  opening: vi.fn(),
  success: vi.fn(),
  error: vi.fn(),
}))
vi.mock('@/api/vip', () => ({
  getVIPRules: mocks.rules,
  getVIP: mocks.user,
  saveVIPRules: mocks.save,
  saveVIPOverride: mocks.override,
  clearVIPOverride: mocks.restore,
  setVIPLevel: mocks.level,
  restoreVIPLevel: mocks.restoreLevel,
  createVIPOpening: mocks.opening,
}))
vi.mock('@/api/client', () => ({ apiClient: { get: mocks.groups } }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: mocks.success, showError: mocks.error }),
}))
function render() {
  return mount(VIPRulesView, {
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh: {} },
          missingWarn: false,
          fallbackWarn: false,
        }),
      ],
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
        ConfirmDialog: {
          props: ['show'],
          template:
            '<div v-if="show" data-confirm><button @click="$emit(\'confirm\')">confirm</button><button @click="$emit(\'cancel\')">cancel</button></div>',
        },
      },
    },
  })
}
describe('Admin VIP privileges', () => {
  it('preserves the exact expiry of a temporary grade when changing its level', async () => {
    const expiry = '2026-12-31T08:35:20.123Z'
    mocks.user.mockResolvedValue({
      total: 0,
      tier: { level: 4 },
      overrides: [],
      level_override: {
        benefit: 'tier',
        value: 4,
        expires_at: expiry,
        reason: 'existing',
      },
    })
    const w = render()
    await flushPromises()
    await w.get('[data-tab="users"]').trigger('click')
    await w.get('[data-user-id]').setValue('7')
    await w.get('#vip-panel-users form').trigger('submit')
    await flushPromises()
    expect(
      (
        w.get('.vip-level-form input[type=datetime-local]')
          .element as HTMLInputElement
      ).value,
    ).not.toBe('')
    await w.get('[data-level-reason]').setValue('保持临时等级')
    await w.get('.vip-level-form').trigger('submit')
    await w.get('[data-confirm] button').trigger('click')
    await flushPromises()
    expect(mocks.level).toHaveBeenCalledWith(
      7,
      expect.objectContaining({ expires_at: expiry }),
    )
  })
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.rules.mockResolvedValue({
      enabled: true,
      currency: 'USD',
      access_threshold: 100,
      exchange_rates: { USD: 1 },
      daily_rewards: [0.01, 0.05, 0.1, 0.25, 0.5, 1],
      tiers: [
        { level: 1, threshold: 100, concurrency: 8, rpm: 0, rebate_percent: 2 },
      ],
      groups: [],
    })
    mocks.groups.mockResolvedValue({
      data: [
        { id: 1, name: 'normal', rate_multiplier: 0.3, is_exclusive: false },
      ],
    })
    mocks.user.mockResolvedValue({
      total: 400,
      tier: { level: 2 },
      concurrency: 12,
      rebate_percent: 4,
      overrides: [{ benefit: 'badge', value: 2, reason: 'test' }],
    })
  })
  it('separates three panels, retains drafts and only notifies after writes', async () => {
    const wrapper = render()
    await flushPromises()
    expect(mocks.success).not.toHaveBeenCalled()
    expect(wrapper.findAll('[role="tab"]')).toHaveLength(3)
    await wrapper.get('[data-threshold="1"]').setValue('100')
    await wrapper.get('[data-tab="groups"]').trigger('click')
    await wrapper.get('[data-discount="1-1"]').setValue('0.02')
    await wrapper.get('[data-tab="rules"]').trigger('click')
    await wrapper.get('[data-save-rules]').trigger('click')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({
        groups: [expect.objectContaining({ discounts: [0.02, 0, 0, 0, 0] })],
      }),
    )
    expect(mocks.success).toHaveBeenCalledTimes(1)
    expect(mocks.override).not.toHaveBeenCalled()
  })
  it('queries silently and confirms restore and opening without writing on cancel', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-tab="users"]').trigger('click')
    await wrapper.get('[data-user-id]').setValue('7')
    await wrapper.get('#vip-panel-users form').trigger('submit')
    await flushPromises()
    expect(mocks.success).not.toHaveBeenCalled()
    await wrapper.get('[data-restore-level]').trigger('click')
    expect(mocks.restoreLevel).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm] button').trigger('click')
    await flushPromises()
    expect(mocks.restoreLevel).toHaveBeenCalledWith(7, expect.any(String))
    await wrapper.get('[data-opening-amount]').setValue('100')
    await wrapper.get('[data-opening-reason]').setValue('核对完成')
    await wrapper.get('details form').trigger('submit')
    expect(mocks.opening).not.toHaveBeenCalled()
    await wrapper.findAll('[data-confirm] button')[1].trigger('click')
    expect(mocks.opening).not.toHaveBeenCalled()
  })
  it('displays structured API errors and clears stale user data after failed lookup', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-tab="users"]').trigger('click')
    await wrapper.get('[data-user-id]').setValue('7')
    await wrapper.get('#vip-panel-users form').trigger('submit')
    await flushPromises()
    mocks.user.mockRejectedValueOnce({ message: '用户不存在', status: 404 })
    await wrapper.get('[data-user-id]').setValue('8')
    await wrapper.get('#vip-panel-users form').trigger('submit')
    await flushPromises()
    expect(mocks.error).toHaveBeenCalledWith('用户不存在')
    expect(wrapper.find('[data-user-summary]').exists()).toBe(false)
  })
  it('preserves dirty rules when refresh is cancelled and supports keyboard tabs', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper
      .get('[data-tab="rules"]')
      .trigger('keydown', { key: 'ArrowRight' })
    expect(wrapper.get('[data-tab="groups"]').attributes('aria-selected')).toBe(
      'true',
    )
    await wrapper.get('[data-discount="1-1"]').setValue('0.03')
    await wrapper.get('[aria-label="刷新数据"]').trigger('click')
    expect(mocks.rules).toHaveBeenCalledTimes(1)
    await wrapper.findAll('[data-confirm] button')[1].trigger('click')
    expect(
      (wrapper.get('[data-discount="1-1"]').element as HTMLInputElement).value,
    ).toBe('0.03')
  })
  it('sets one complete grade only after confirmation and refuses a changed user', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-tab="users"]').trigger('click')
    await wrapper.get('[data-user-id]').setValue('7')
    await wrapper.get('#vip-panel-users form').trigger('submit')
    await flushPromises()
    const picker = wrapper
      .findAllComponents(Select)
      .find((c) => c.attributes('data-vip-level') !== undefined)!
    picker.vm.$emit('update:modelValue', 5)
    await wrapper.get('[data-level-reason]').setValue('人工核对')
    await wrapper.get('.vip-level-form').trigger('submit')
    expect(mocks.level).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm] button').trigger('click')
    await flushPromises()
    expect(mocks.level).toHaveBeenCalledWith(7, {
      level: 5,
      expires_at: null,
      reason: '人工核对',
    })
    expect(mocks.override).not.toHaveBeenCalled()
    expect(mocks.save).not.toHaveBeenCalled()
    expect(wrapper.find('.vip-override-form').exists()).toBe(false)
    await wrapper.get('[data-user-id]').setValue('8')
    expect(
      wrapper.get('[data-restore-level]').attributes('disabled'),
    ).toBeDefined()
    await wrapper.get('.vip-level-form').trigger('submit')
    await flushPromises()
    expect(mocks.level).toHaveBeenCalledTimes(1)
  })
  it('displays and saves the same six daily reward policy values', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-daily-reward="0"]').element.value).toBe('0.01')
    await wrapper.get('[data-daily-reward="1"]').setValue('0.08')
    await wrapper.get('[data-save-rules]').trigger('click')
    await flushPromises()
    expect(mocks.save).toHaveBeenCalledWith(
      expect.objectContaining({
        daily_rewards: [0.01, 0.08, 0.1, 0.25, 0.5, 1],
      }),
    )
  })
})
