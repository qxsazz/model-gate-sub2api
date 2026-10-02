import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import VIPMembershipCard from '../VIPMembershipCard.vue'

describe('B membership card', () => {
  it('shares one engraving artwork and keeps owner data separate from the background', () => {
    const wrapper = mount(VIPMembershipCard, { props: { level: 5, name: '黑钻', thresholdLabel: '累计有效充值 $3,000 起', benefits: ['普通分组最高减免 0.1', '30 并发 · 邀请返利 10%'], ownerName: '测试会员', ownerId: 702 } })
    expect(wrapper.get('.vip-card-artwork').attributes('src')).toBe('/mg-vip-b-engraving.png')
    expect(wrapper.get('.vip-card-number').text()).toBe('5')
    expect(wrapper.get('.member-owner').text()).toContain('UID 702')
    expect(wrapper.text()).toContain('测试会员')
    expect(wrapper.text()).toContain('普通分组最高减免 0.1')
    expect(wrapper.text()).not.toContain('0.1%')
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
