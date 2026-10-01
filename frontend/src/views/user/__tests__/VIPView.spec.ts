import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import VIPView from '../VIPView.vue'
const { getMock, membershipMock, claimMock } = vi.hoisted(() => ({ getMock: vi.fn(), membershipMock: vi.fn(), claimMock: vi.fn() }))
vi.mock('@/api/vip', () => ({ getVIP: getMock, getVIPMembership: membershipMock, claimVIPReward: claimMock }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ refreshUser: vi.fn().mockResolvedValue({}) }) }))
function render() { return mount(VIPView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, VIPBadge: true, Icon: true, RouterLink: { template: '<a><slot /></a>' } } } }) }
describe('VIP center', () => {
 beforeEach(() => { getMock.mockReset(); claimMock.mockReset(); membershipMock.mockResolvedValue({rewards:[{level:1,threshold:100,amount:1,status:'available'}],seats:[{name:'17***',level:5}],debt:0,claimed:0,discount_summaries:{1:[.02,.005]}}) })
 it('preserves small group rate precision and distinguishes concurrency from RPM', async () => {
  getMock.mockResolvedValue({ enabled:true,total:100,tier:{level:1,threshold:100},badge_level:1,concurrency:8,rpm:0,rebate_percent:2,next:{level:2,threshold:300},rules:{access_threshold:100,tiers:[]},groups:[{id:1,name:'codex-plus2',exclusive:false,base_rate:0.088,rate:0.088,participating:false}],ledger:[] })
  const wrapper = render(); await flushPromises()
  expect(wrapper.text()).not.toContain('codex-plus2')
  expect(wrapper.text()).toContain('同时请求上限')
  expect(wrapper.text()).toContain('每分钟请求上限')
  expect(wrapper.text()).toContain('200.00')
  expect(wrapper.text()).toContain('17***')
  await wrapper.get('[data-tab="rewards"]').trigger('click')
  expect(wrapper.text()).toContain('$1.00')
  claimMock.mockResolvedValue({amount:1})
  await wrapper.get('[data-claim="1"]').trigger('click'); await flushPromises()
  expect(claimMock).toHaveBeenCalledWith(1)
 })
 it('does not display zero recharge when the request fails', async () => {
  getMock.mockRejectedValue(new Error('network'))
  const wrapper = render(); await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('读取失败')
  expect(wrapper.find('.membership').exists()).toBe(false)
 })
})
