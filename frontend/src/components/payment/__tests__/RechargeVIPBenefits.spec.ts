import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import zh from '@/i18n/locales/zh/misc'
import RechargeVIPBenefits from '../RechargeVIPBenefits.vue'

const api = vi.hoisted(() => ({ getVIP: vi.fn(), getVIPMembership: vi.fn() }))
vi.mock('@/api/vip', () => api)
const tiers = [100, 300, 600, 1500, 3000].map((threshold, index) => ({
  level: index + 1, threshold, concurrency: [8, 12, 16, 20, 30][index], rpm: 0,
  rebate_percent: (index + 1) * 2, recharge_bonus_percent: index + 1,
}))
const snapshot = () => ({
  enabled: true, total: 400, tier: tiers[1], tier_source: 'growth',
  rules: { access_threshold: 100, tiers, daily_rewards: [.01, .05, .1, .25, .5, 1] },
})
const render = (extra = {}) => mount(RechargeVIPBenefits, {
  props: { show: false, amount: 200, currency: 'CNY', growthRates: { CNY: 1 }, currentMultiplier: 1.02, bonusEnabled: true, ...extra },
  global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { ...zh, common: { loading: '加载中' } } } })],
    stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, RouterLink: { template: '<a><slot /></a>' } },
  },
})

describe('recharge VIP benefits', () => {
  beforeEach(() => {
    api.getVIP.mockReset().mockResolvedValue(snapshot())
    api.getVIPMembership.mockReset().mockResolvedValue({ discount_summaries: { 5: [.075] }, rewards: [{ level: 5, amount: 63, status: 'locked' }] })
  })

  it('reuses the parallel checkout snapshot and reserves growth space while it loads', async () => {
    let resolve!: (value: ReturnType<typeof snapshot>) => void
    const wrapper = render({ initialRequest: new Promise(done => { resolve = done }) })
    expect(wrapper.get('[data-vip-skeleton]').attributes('aria-busy')).toBe('true')
    resolve(snapshot())
    await flushPromises()
    expect(api.getVIP).not.toHaveBeenCalled()
    expect(wrapper.find('[data-vip-skeleton]').exists()).toBe(false)
    expect(wrapper.emitted('snapshot')?.[0]).toEqual([snapshot()])
    expect(api.getVIPMembership).not.toHaveBeenCalled()
  })

  it('browses six original cards independently of the account tier and uses actual benefit configuration', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('update:show')).toEqual([[true]])
    await wrapper.setProps({ show: true })
    await flushPromises()
    const choices = wrapper.findAll('.recharge-tier-picker button')
    expect(choices).toHaveLength(6)
    expect(new Set(choices.map(button => button.get('[data-material]').attributes('data-material'))).size).toBe(6)
    expect(choices[2].attributes('aria-pressed')).toBe('true')
    await choices[5].trigger('click')
    expect(wrapper.get('.recharge-benefit-figures').text()).toContain('$63.00')
    expect(wrapper.get('.recharge-usage').text()).toContain('0.075')
    expect(wrapper.get('.recharge-tier-detail').text()).toContain('22.62%')
    expect(wrapper.get('[data-current-bonus]').text()).toBe('2%')
    expect(wrapper.get('[data-vip-center]').attributes('to')).toBe('/vip')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    expect(wrapper.get('.recharge-tier-picker [aria-pressed="true"]').text()).toContain('VIP 2')
    expect(api.getVIPMembership).toHaveBeenCalledTimes(1)
  })

  it('keeps the progress bar on paid principal and applies future upgrades only to later orders', async () => {
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('33')
    expect(wrapper.get('[data-future-total]').text()).toBe('$600.00')
    expect(wrapper.get('[data-projected-tier]').text()).toBe('VIP 3')
    expect(wrapper.get('[data-current-bonus]').text()).toBe('2%')
    expect(wrapper.get('[data-projected-bonus]').text()).toBe('3%')
    await wrapper.setProps({ amount: 2600 })
    expect(wrapper.get('[data-projected-tier]').text()).toBe('VIP 5')
    expect(wrapper.get('[data-current-bonus]').text()).toBe('2%')
  })

  it('does not guess an unavailable currency conversion', async () => {
    const wrapper = render({ growthRates: {} })
    await flushPromises()
    expect(wrapper.find('[data-projected-tier]').exists()).toBe(false)
    expect(wrapper.text()).toContain('无法预计')
  })

  it('respects a manual tier and continues tracking actual top-up principal', async () => {
    api.getVIP.mockResolvedValue({ ...snapshot(), tier_source: 'manual', tier: tiers[4] })
    const wrapper = render({ currentMultiplier: 1.05 })
    await flushPromises()
    expect(wrapper.get('[data-projected-tier]').text()).toBe('VIP 5')
    expect(wrapper.get('[data-future-total]').text()).toBe('$600.00')
    expect(wrapper.text()).toContain('管理员指定')
  })

  it('keeps checkout usable and can recover from a failed snapshot', async () => {
    api.getVIP.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toContain('权益暂时无法读取')
    expect(wrapper.emitted('snapshot')?.[0]).toEqual([null])
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-projected-tier]').text()).toBe('VIP 3')
  })
})
