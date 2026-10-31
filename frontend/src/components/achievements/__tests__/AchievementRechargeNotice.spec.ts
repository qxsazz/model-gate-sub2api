import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AchievementRechargeNotice from '../AchievementRechargeNotice.vue'
import { achievementRechargeAccess } from '../rechargeAccess'

const locked = {
  access_unlocked: false,
  access_reason: 'recharge_required',
  minimum_recharge_cny: 12,
  qualified_recharge_cny: 9.99,
  remaining_recharge_cny: 2.01,
}

const render = (snapshot: Parameters<typeof achievementRechargeAccess>[0]) =>
  mount(AchievementRechargeNotice, {
    props: { access: achievementRechargeAccess(snapshot), refreshing: false },
    global: {
      stubs: { RouterLink: { props: ['to'], template: '<a :href="to"><slot /></a>' } },
    },
  })

describe('achievement recharge guidance', () => {
  it('shows server amounts in CNY and links to the existing recharge page', async () => {
    const wrapper = render(locked)
    expect(wrapper.text()).toContain('¥12.00')
    expect(wrapper.text()).toContain('¥9.99')
    expect(wrapper.text()).toContain('¥2.01')
    expect(wrapper.text()).toContain('正常消费余额不会影响资格')
    expect(wrapper.text()).not.toContain('$')
    expect(wrapper.get('a').attributes('href')).toBe('/purchase')
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('does not announce a rule for a legacy backend', () => {
    expect(render({}).find('section').exists()).toBe(false)
    expect(achievementRechargeAccess({}).blocked).toBe(false)
  })

  it.each([
    { ...locked, qualified_recharge_cny: undefined },
    { ...locked, remaining_recharge_cny: -1 },
    { ...locked, qualified_recharge_cny: 'NaN' },
    { ...locked, remaining_recharge_cny: 1 },
    { ...locked, access_reason: 'verification_unavailable' },
  ])('offers retry without requesting payment when qualification cannot be verified', (snapshot) => {
    const wrapper = render(snapshot)
    expect(wrapper.text()).toContain('暂时无法核验充值资格')
    expect(wrapper.text()).toContain('无需重复充值')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(achievementRechargeAccess(snapshot).blocked).toBe(true)
  })

  it('does not suggest payment to restore a disabled account', () => {
    const snapshot = { ...locked, access_unlocked: true, access_reason: 'account_unavailable' }
    const wrapper = render(snapshot)
    expect(wrapper.text()).toContain('充值不会解除账户限制')
    expect(wrapper.find('a').exists()).toBe(false)
    expect(achievementRechargeAccess(snapshot).blocked).toBe(true)
  })

  it('acknowledges qualification without promising cash rewards', () => {
    const wrapper = render({ ...locked, access_unlocked: true, access_reason: 'eligible', qualified_recharge_cny: 12, remaining_recharge_cny: 0 })
    expect(wrapper.text()).toContain('有效充值门槛已满足')
    expect(wrapper.text()).toContain('现金开关、预算和领取记录')
    expect(wrapper.find('a').exists()).toBe(false)
  })

  it('only uses a refund-specific explanation when the backend confirms the reason', () => {
    expect(render({ ...locked, access_reason: 'recharge_refunded' }).text()).toContain('已扣除确认退款')
    expect(render(locked).text()).not.toContain('退款后重新锁定')
  })

  it('does not encourage duplicate payment while the backend confirms an order', () => {
    const wrapper = render({ ...locked, access_reason: 'payment_pending' })
    expect(wrapper.text()).toContain('支付结果正在确认')
    expect(wrapper.find('a').exists()).toBe(false)
  })
})
