import { describe, expect, it } from 'vitest'
import { calendarEntryFor, calendarFutureStatus } from '../calendarContent'

describe('calendar content by date', () => {
  it('keeps a full month of distinct questions and advice across year, month and leap-day boundaries', () => {
    const firstDay = Date.UTC(2027, 11, 1)
    const entries = Array.from({ length: 130 }, (_, i) => {
      const date = new Date(firstDay + i * 86_400_000).toISOString().slice(0, 10)
      return calendarEntryFor(date)
    })
    for (let i = 0; i <= entries.length - 32; i++) {
      const window = entries.slice(i, i + 32)
      for (const key of ['question', 'tip', 'prompt', 'recommended', 'avoid'] as const) {
        expect(new Set(window.map(entry => entry[key])).size).toBe(window.length)
      }
    }
    for (const entry of new Set(entries)) {
      expect(entry.question).toMatch(/？$/)
      expect(entry.prompt).toMatch(/\[.+?\]/)
      expect(entry.tip).not.toBe(entry.recommended)
    }
  })

  it('revisits the same entry and counts real calendar days across a leap day', () => {
    const entry = calendarEntryFor('2028-02-29')
    calendarEntryFor('2028-03-01')
    expect(calendarEntryFor('2028-02-29')).toEqual(entry)
    expect(calendarFutureStatus('2028-03-01', '2028-02-28')).toContain('2 天')
    expect(calendarFutureStatus('2027-03-01', '2027-02-28')).toContain('1 天')
    expect(calendarEntryFor('1969-12-31').question).toMatch(/？$/)
  })
})
