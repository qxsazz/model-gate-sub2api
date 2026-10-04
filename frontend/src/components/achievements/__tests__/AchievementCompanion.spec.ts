import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import AchievementCompanion from '../AchievementCompanion.vue'
import type { AchievementState } from '@/api/achievements'
const save = vi.hoisted(() => vi.fn())
vi.mock('@/api/achievements', () => ({ saveAchievementZodiac: save }))
const state = {
  date: '2026-10-04',
  joined_date: '2026-06-29',
  companionship_days: 98,
  zodiac: '',
  medals: [],
  total_days: 2,
} as unknown as AchievementState
describe('account companion', () => {
  it('uses real dates and saves an explicit optional choice', async () => {
    save.mockResolvedValue({ zodiac: 'libra' })
    const w = mount(AchievementCompanion, { props: { state } })
    expect(w.text()).toContain('98')
    expect(w.get('.memory-ribbon').attributes('aria-label')).toContain(
      '2026-06-29',
    )
    expect(w.get('select').element.value).toBe('')
    await w.get('select').setValue('libra')
    await flushPromises()
    expect(save).toHaveBeenCalledWith('libra')
    expect(w.emitted('updated')).toHaveLength(1)
  })
  it('reports a failed save without claiming persistence', async () => {
    save.mockRejectedValue(new Error('network'))
    const w = mount(AchievementCompanion, { props: { state } })
    await w.get('select').setValue('aries')
    await flushPromises()
    expect(w.get('[role=alert]').text()).toContain('暂未保存')
    expect(w.get('select').element.value).toBe('')
    expect(w.emitted('updated')).toBeUndefined()
  })
  it('keeps the approved zodiac symbol, keyword, date, sun advice and footnote', () => {
    const w = mount(AchievementCompanion, {
      props: { state: { ...state, zodiac: 'gemini' }, date: '2026-10-06' },
    })
    expect(w.get('.zodiac-symbol').text()).toBe('♊')
    expect(w.get('.zodiac-keyword').text()).toContain('灵感')
    expect(w.get('.zodiac-day').text()).toBe('10 / 06')
    expect(w.find('.energy-suggestion svg').exists()).toBe(true)
    expect(w.text()).toContain('今天怎样度过，仍由你决定')
  })
})
