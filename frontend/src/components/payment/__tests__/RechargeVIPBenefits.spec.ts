import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import zh from '@/i18n/locales/zh/misc'
import type { VIPSnapshot } from '@/api/vip'
import RechargeVIPBenefits from '../RechargeVIPBenefits.vue'
import RechargeVIPGrowth from '../RechargeVIPGrowth.vue'

const api = vi.hoisted(() => ({ getVIP: vi.fn(), getVIPMembership: vi.fn() }))
vi.mock('@/api/vip', () => api)
const tiers = [100, 300, 600, 1500, 3000].map((threshold, index) => ({
  level: index + 1, threshold, concurrency: [8, 12, 16, 20, 30][index], rpm: 0,
  rebate_percent: (index + 1) * 2, recharge_bonus_percent: index + 1,
}))
const snapshot = (): VIPSnapshot => ({
  enabled: true, total: 420, tier: tiers[1], tier_source: 'growth',
  concurrency: 18, rpm: 75, rebate_percent: 4.5, badge_level: 2, next: tiers[2], ledger: [], overrides: [],
  groups: [{ id: 1, name: 'VIP', platform: 'openai', exclusive: true, subscription: '', base_rate: 1, rate: 1, granted: true, participating: true }],
  rules: { enabled: true, currency: 'USD', groups: [], exchange_rates: { CNY: 1 }, access_threshold: 100, tiers, daily_rewards: [.01, .05, .1, .25, .5, 1] },
})
const render = (extra = {}) => mount(RechargeVIPBenefits, {
  props: { show: false, currentMultiplier: 1.02, bonusEnabled: true, ...extra },
  global: {
    plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: { ...zh, common: { loading: '加载中' } } } })],
    stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' }, RouterLink: { template: '<a><slot /></a>' } },
  },
})

describe('recharge VIP benefits', () => {
  beforeEach(() => {
    api.getVIP.mockReset().mockResolvedValue(snapshot())
    api.getVIPMembership.mockReset().mockResolvedValue({
      discount_summaries: { 2: [.03], 5: [.075] },
      rewards: [{ level: 2, amount: 6, status: 'available' }, { level: 5, amount: 63, status: 'locked' }],
    })
  })

  it('reuses the parallel checkout snapshot and displays the current benefits once loaded', async () => {
    let resolve!: (value: ReturnType<typeof snapshot>) => void
    const wrapper = render({ initialRequest: new Promise(done => { resolve = done }) })
    expect(wrapper.get('[data-vip-skeleton]').attributes('aria-busy')).toBe('true')
    resolve(snapshot())
    await flushPromises()
    expect(api.getVIP).not.toHaveBeenCalled()
    expect(wrapper.find('[data-vip-skeleton]').exists()).toBe(false)
    expect(wrapper.emitted('snapshot')?.[0]).toEqual([snapshot()])
    expect(api.getVIPMembership).toHaveBeenCalledTimes(1)
    const rights = wrapper.get('.recharge-perk-list')
    expect(rights.findAll('dt')).toHaveLength(8)
    expect(rights.findAll('dd').map(node => node.text().replace(/\s+/g, ''))).toEqual([
      '2%', '$0.10符合签到资格时', '4.5%', '$6.00可在VIP中心领取', '18', '75', '倍率最高减免0.03', '已开放',
    ])
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
    expect(wrapper.get('.recharge-perk-list').text()).toContain('$6.00')
    expect(wrapper.get('.recharge-perk-list').text()).not.toContain('$63.00')
    expect(wrapper.get('.recharge-tier-detail').text()).toContain('22.62%')
    expect(wrapper.get('.recharge-perk-list dd').text()).toBe('2%')
    expect(wrapper.get('[data-vip-center]').attributes('to')).toBe('/vip')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    expect(wrapper.get('.recharge-tier-picker [aria-pressed="true"]').text()).toContain('VIP 2')
    expect(api.getVIPMembership).toHaveBeenCalledTimes(1)
  })

  it('shows account benefits when details fail and retries only the missing membership request', async () => {
    api.getVIPMembership.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render({ bonusEnabled: false })
    await flushPromises()
    expect(wrapper.get('.recharge-perk-list').text()).toContain('未启用')
    expect(wrapper.get('.recharge-perk-list').text()).toContain('4.5%')
    await wrapper.get('[role="status"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('.recharge-perk-list').text()).toContain('$6.00')
    expect(api.getVIP).toHaveBeenCalledTimes(1)
  })

  it('keeps checkout usable and can recover from a failed snapshot', async () => {
    api.getVIP.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[role="status"]').text()).toContain('权益暂时无法读取')
    expect(wrapper.emitted('snapshot')?.[0]).toEqual([null])
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(wrapper.emitted('snapshot')?.at(-1)).toEqual([snapshot()])
    expect(wrapper.get('.recharge-perk-list').exists()).toBe(true)
  })
})

describe('recharge membership growth', () => {
  const renderGrowth = (overrides: Partial<InstanceType<typeof RechargeVIPGrowth>['$props']> = {}) =>
    mount(RechargeVIPGrowth, {
      props: { snapshot: snapshot(), amount: 0, growthRate: 1, bonusPercent: '2', ...overrides },
      global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] },
    })

  it('keeps settled progress separate from estimates and opens all benefits', async () => {
    const wrapper = renderGrowth()
    expect(wrapper.get('.recharge-growth-line').text()).toContain('$420.00')
    expect(wrapper.get('.recharge-progress-label').text()).toContain('下一档 VIP 3 · 黄金')
    expect(wrapper.get('.recharge-progress-label').text()).toContain('还差 $180.00')
    expect(wrapper.find('.recharge-estimate').exists()).toBe(false)
    await wrapper.setProps({ amount: 100 })
    expect(wrapper.get('.recharge-estimate').text()).toBe('本次充值后，距 VIP 3 还差 $80.00。')
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('40')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('show-benefits')).toEqual([[]])
  })

  it('uses the checkout conversion, including exact and multi-tier upgrades, without upgrading the current order', async () => {
    const wrapper = renderGrowth({ amount: 90, growthRate: 2 })
    const estimate = wrapper.get('.recharge-estimate')
    expect(estimate.text()).toContain('VIP 3 · 黄金，累计 $600.00')
    expect(estimate.text()).toContain('本笔加赠 2% · 下一笔加赠 3%')
    await wrapper.setProps({ amount: 1290 })
    expect(estimate.text()).toContain('VIP 5 · 黑钻，累计 $3000.00')
    expect(estimate.text()).toContain('本笔加赠 2% · 下一笔加赠 5%')
    await wrapper.setProps({ bonusPercent: undefined })
    expect(estimate.text()).not.toContain('加赠')
  })

  it('withholds estimates when conversion is unavailable and keeps the manual tier fixed', async () => {
    const wrapper = renderGrowth({ amount: 180, growthRate: undefined })
    expect(wrapper.get('.recharge-estimate').text()).toContain('无法预计')
    await wrapper.setProps({ growthRate: 1, snapshot: { ...snapshot(), tier_source: 'manual', tier: tiers[4] } })
    expect(wrapper.text()).toContain('管理员指定为 VIP 5')
    expect(wrapper.get('.recharge-estimate').text()).toContain('$600.00')
    expect(wrapper.get('.recharge-estimate').text()).not.toContain('升至')
  })

  it('shows a full bar at the highest tier and continues accumulating principal', () => {
    const wrapper = renderGrowth({ amount: 100, snapshot: { ...snapshot(), total: 3000, tier: tiers[4], next: null } })
    expect(wrapper.get('[role="progressbar"]').attributes('aria-valuenow')).toBe('100')
    expect(wrapper.get('.recharge-progress-label').text()).toBe('已达最高成长等级')
    expect(wrapper.get('.recharge-estimate').text()).toContain('$3100.00')
  })
})
