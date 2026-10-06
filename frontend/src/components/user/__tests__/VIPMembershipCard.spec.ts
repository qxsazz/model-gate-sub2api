import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import VIPMembershipCard from '../VIPMembershipCard.vue'

describe('metal membership card', () => {
  it('preserves every benefit and owner identity while adding decorative material layers', () => {
    const benefits = ['最高减免 0.03 · 加赠 2%', '12 并发 · 邀请返利 4%', '本档奖励 $6.00', '签到 $0.10 / 日']
    const wrapper = mount(VIPMembershipCard, { props: { level: 2, name: '白银', thresholdLabel: '累计有效充值 $300.00 起', benefits, ownerName: 'Member', ownerId: 702 } })
    expect(wrapper.findAll('.vip-card-description p')).toHaveLength(5)
    for (const text of benefits) expect(wrapper.text()).toContain(text)
    expect(wrapper.text()).toContain('UID 702')
    expect(wrapper.find('.vip-card-grain').exists()).toBe(true)
    expect(wrapper.find('.vip-card-reflection').attributes('aria-hidden')).toBe('true')
  })
  it('changes reflection only for a mouse and restores the resting material on leave', async () => {
    const media = vi.spyOn(window, 'matchMedia').mockReturnValue({ matches: false } as MediaQueryList)
    const wrapper = mount(VIPMembershipCard, { props: { level: 0, name: '普通会员', thresholdLabel: '注册即享', benefits: [] } })
    const card = wrapper.element as HTMLElement
    card.getBoundingClientRect = () => ({ left: 0, top: 0, width: 500, height: 300 }) as DOMRect
    await wrapper.trigger('pointermove', { pointerType: 'touch', clientX: 400, clientY: 100 })
    expect(card.style.getPropertyValue('--vip-light-x')).toBe('')
    await wrapper.trigger('pointermove', { pointerType: 'mouse', clientX: 400, clientY: 100 })
    expect(card.style.getPropertyValue('--vip-light-x')).toBe('80%')
    await wrapper.trigger('pointerleave')
    expect(card.style.getPropertyValue('--vip-light-x')).toBe('')
    media.mockRestore()
  })
  it('shares one engraving artwork and keeps owner data separate from the background', () => {
    const wrapper = mount(VIPMembershipCard, { props: { level: 5, name: '黑钻', thresholdLabel: '累计有效充值 $3,000 起', benefits: ['普通分组最高减免 0.075', '30 并发 · 邀请返利 10%'], ownerName: '测试会员', ownerId: 702 } })
    expect(wrapper.get('.vip-card-artwork').attributes('src')).toBe('/mg-vip-b-engraving.png')
    expect(wrapper.get('.vip-card-number').text()).toBe('5')
    expect(wrapper.get('.member-owner').text()).toContain('UID 702')
    expect(wrapper.text()).toContain('测试会员')
    expect(wrapper.text()).toContain('普通分组最高减免 0.075')
    expect(wrapper.text()).not.toContain('0.075%')
  })
  it('does not invent owner identity on unowned tier previews', () => {
    const cards = [0,1,2,3,4,5].map(level => mount(VIPMembershipCard, { props: { level, name: '会员', thresholdLabel: '规则门槛', benefits: [] } }))
    expect(new Set(cards.map(card => card.get('.vip-card-artwork').attributes('src'))).size).toBe(1)
    expect(new Set(cards.map(card => card.attributes('data-material'))).size).toBe(6)
    expect(cards[0].attributes('data-material')).toBe('silver')
    expect(cards[2].attributes('data-material')).toBe('pearl')
    for (const card of cards) {
      expect(card.find('.member-owner').exists()).toBe(false)
      expect(card.text()).not.toContain('UID')
    }
  })
})
