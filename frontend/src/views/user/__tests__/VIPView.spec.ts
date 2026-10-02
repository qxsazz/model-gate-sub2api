import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import VIPView from '../VIPView.vue'
const { getMock, membershipMock, claimMock, authMock } = vi.hoisted(() => ({ getMock: vi.fn(), membershipMock: vi.fn(), claimMock: vi.fn(), authMock: {user:{id:702,username:'测试会员',email:'vip@example.com'},refreshUser:vi.fn().mockResolvedValue({})} }))
vi.mock('@/api/vip', () => ({ getVIP: getMock, getVIPMembership: membershipMock, claimVIPReward: claimMock }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authMock }))
function render() { return mount(VIPView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, VIPBadge: true, Icon: true, RouterLink: { template: '<a><slot /></a>' } } } }) }
describe('VIP center', () => {
 beforeEach(() => { getMock.mockReset(); claimMock.mockReset(); membershipMock.mockResolvedValue({rewards:[{level:1,threshold:100,amount:1,status:'available'}],seats:[{name:'17***',level:5}],debt:0,claimed:0,discount_summaries:{1:[.02,.005]}}) })
 it('preserves small group rate precision and distinguishes concurrency from RPM', async () => {
  getMock.mockResolvedValue({ enabled:true,total:100,tier:{level:1,threshold:100},badge_level:1,concurrency:8,rpm:0,rebate_percent:2,next:{level:2,threshold:300},rules:{access_threshold:100,tiers:[]},groups:[{id:1,name:'codex-plus2',exclusive:false,base_rate:0.088,rate:0.088,participating:false}],ledger:[] })
  const wrapper = render(); await flushPromises()
  expect(wrapper.text()).not.toContain('codex-plus2')
  expect(wrapper.text()).toContain('8 并发')
  expect(wrapper.text()).toContain('每分钟请求上限')
  expect(wrapper.text()).toContain('200.00')
  expect(wrapper.text()).toContain('17***')
  await wrapper.get('[data-tab="rewards"]').trigger('click')
  expect(wrapper.text()).toContain('$1.00')
  expect(wrapper.get('.reward-actions').get('.reward-amount').text()).toContain('$1.00')
  expect(wrapper.get('.reward-actions').get('[data-claim="1"]').text()).toContain('领取奖励')
  claimMock.mockResolvedValue({amount:1})
  await wrapper.get('[data-claim="1"]').trigger('click'); await flushPromises()
  expect(claimMock).toHaveBeenCalledWith(1)
  claimMock.mockRejectedValueOnce({reason:'VIP_REWARD_THRESHOLD_NOT_REACHED',message:'累计有效充值未达到该档奖励门槛。'})
  await wrapper.get('[data-claim="1"]').trigger('click'); await flushPromises()
  expect(wrapper.get('[role="status"]').text()).toContain('累计有效充值未达到')
 })
 it('does not display zero recharge when the request fails', async () => {
  getMock.mockRejectedValue(new Error('network'))
  const wrapper = render(); await flushPromises()
  expect(wrapper.get('[role="alert"]').text()).toContain('读取失败')
  expect(wrapper.find('.membership').exists()).toBe(false)
 })
 it('keeps manual privilege notices outside the edge-aligned summary and does not invent recharge qualification', async () => {
  getMock.mockResolvedValue({enabled:true,total:100,tier:{level:1,threshold:100},badge_level:5,concurrency:30,rpm:0,rebate_percent:2,next:null,rules:{access_threshold:100,tiers:[]},groups:[],ledger:[],overrides:[{benefit:'badge',value:5}]})
  const wrapper=render(); await flushPromises()
  expect(wrapper.get('.member-card').text()).toContain('充值成长等级 VIP 1')
  expect(wrapper.get('.membership-followup').text()).toContain('管理员授予')
  expect(wrapper.get('.member-overview').find('.status-note').exists()).toBe(false)
 })
 it('shows private owner identity, branded tier cards and accurate membership rules', async () => {
  getMock.mockResolvedValue({enabled:true,total:100,tier:{level:1,threshold:100},badge_level:1,concurrency:8,rpm:0,rebate_percent:2,next:null,rules:{access_threshold:100,tiers:[1,2,3,4,5].map(level=>({level,threshold:level*100,concurrency:8,rebate_percent:2}))},groups:[],ledger:[]})
  const wrapper=render(); await flushPromises()
  expect(wrapper.get('.member-owner').text()).toContain('UID 702')
  expect(wrapper.get('.member-owner').text()).toContain('测试会员')
  expect(wrapper.get('.member-card .vip-card-artwork').attributes('src')).toContain('mg-vip-b-engraving.png')
  expect(new Set(wrapper.findAll('.tier-card').map(card=>card.attributes('data-material'))).size).toBe(6)
  expect(wrapper.get('.honors').text()).not.toContain('测试会员')
  expect(wrapper.get('.honors').text()).not.toContain('702')
  expect(wrapper.get('.membership-rules').text()).toContain('后台手动增加余额')
  expect(wrapper.get('.membership-rules').text()).toContain('不重复发放')
  expect(wrapper.get('.membership-rules').text()).not.toContain('7 天')
  authMock.user.username=''
  const fallback=render(); await flushPromises()
  expect(fallback.get('.member-owner').text()).toContain('vip@example.com')
  authMock.user.username='测试会员'
 })
})
