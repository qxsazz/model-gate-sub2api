import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RechargeVIPBenefits from '../RechargeVIPBenefits.vue'
const api = vi.hoisted(() => ({ getVIP: vi.fn(), getVIPMembership: vi.fn() }))
vi.mock('@/api/vip', () => api)
const tiers = [100,300,600,1500,3000].map((threshold,index) => ({ level:index+1,threshold,concurrency:[8,12,16,20,30][index],rpm:0,rebate_percent:(index+1)*2,recharge_bonus_percent:index+1 }))
const snapshot = () => ({ enabled:true,total:400,tier:tiers[1],growth_tier:tiers[1],tier_source:'growth',rules:{enabled:true,recharge_bonus_enabled:true,access_threshold:100,tiers,daily_rewards:[.01,.05,.1,.25,.5,1]},next:tiers[2] })
const render = (extra = {}) => mount(RechargeVIPBenefits, { props:{amount:200,currency:'CNY',growthRates:{CNY:1},currentLevel:2,currentMultiplier:1.02,bonusEnabled:true,...extra},global:{stubs:{RouterLink:{template:'<a><slot /></a>'}}} })
describe('recharge VIP benefits', () => {
 beforeEach(() => { api.getVIP.mockResolvedValue(snapshot());api.getVIPMembership.mockResolvedValue({discount_summaries:{3:[.045]}}) })
 it('forecasts growth from principal only and keeps this order at the server bonus',async()=>{
  const wrapper=render();await flushPromises()
  expect(wrapper.get('[data-projected-tier]').text()).toContain('VIP 3')
  expect(wrapper.get('[data-future-total]').text()).toContain('600.00')
  expect(wrapper.get('[data-current-bonus]').text()).toContain('2%')
  expect(wrapper.get('[data-projected-bonus]').text()).toContain('3%')
  expect(wrapper.text()).toContain('下一笔')
  expect(wrapper.text()).toContain('13.83%')
  await wrapper.setProps({amount:2600})
  expect(wrapper.get('[data-projected-tier]').text()).toContain('VIP 5')
  expect(wrapper.get('[data-current-bonus]').text()).toContain('2%')
  expect(wrapper.text()).toContain('22.62%')
 })
 it('does not guess currency conversion or treat bonus as growth',async()=>{
  const wrapper=render({growthRates:{}});await flushPromises()
  expect(wrapper.find('[data-projected-tier]').exists()).toBe(false)
  expect(wrapper.text()).toContain('无法预计')
 })
 it('preserves manual grade instead of promising an automatic upgrade',async()=>{
  api.getVIP.mockResolvedValue({...snapshot(),tier_source:'manual',tier:tiers[4]})
  const wrapper=render({currentLevel:5,currentMultiplier:1.05});await flushPromises()
  expect(wrapper.get('[data-projected-tier]').text()).toContain('VIP 5')
  expect(wrapper.text()).toContain('管理员指定')
 })
 it('keeps payment usable when the benefit service fails',async()=>{
  api.getVIP.mockRejectedValue(new Error('unavailable'))
  const wrapper=render();await flushPromises()
  expect(wrapper.text()).toContain('权益暂时无法读取')
  expect(wrapper.emitted('ready')?.at(-1)).toEqual([false])
 })
})
