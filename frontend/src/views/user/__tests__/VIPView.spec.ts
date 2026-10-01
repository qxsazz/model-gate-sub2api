import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import VIPView from '../VIPView.vue'
const { getMock } = vi.hoisted(() => ({ getMock: vi.fn() }))
vi.mock('@/api/vip', () => ({ getVIP: getMock }))
function render() { return mount(VIPView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, VIPBadge: true, Icon: true, RouterLink: { template: '<a><slot /></a>' } } } }) }
describe('VIP center', () => {
 beforeEach(() => { getMock.mockReset() })
 it('preserves small group rate precision and distinguishes concurrency from RPM', async () => {
  getMock.mockResolvedValue({ enabled:true,total:100,tier:{level:1,threshold:100},badge_level:1,concurrency:8,rpm:0,rebate_percent:2,next:{level:2,threshold:300},rules:{access_threshold:100,tiers:[]},groups:[{id:1,name:'codex-plus2',exclusive:false,base_rate:0.088,rate:0.088,participating:false}],ledger:[] })
  const wrapper = render(); await flushPromises()
  expect(wrapper.text()).toContain('×0.088')
  expect(wrapper.text()).toContain('同时请求上限')
  expect(wrapper.text()).toContain('每分钟请求上限')
  expect(wrapper.text()).toContain('200.00')
 })
 it('does not display zero recharge when the request fails', async () => {
  getMock.mockRejectedValue(new Error('network'))
  const wrapper = render(); await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('读取失败')
  expect(wrapper.find('.membership').exists()).toBe(false)
 })
})
