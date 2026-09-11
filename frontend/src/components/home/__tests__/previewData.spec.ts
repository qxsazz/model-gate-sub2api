import { describe, expect, it } from 'vitest'
import { createPreviewRecords, previewCharts, previewDashboardStats, summarizePreview } from '../previewData'

describe('isolated preview data', () => {
  const now = new Date(2026, 8, 10, 15, 0, 0)
  const records = createPreviewRecords(now)

  it('does not generate future records or real credentials', () => {
    expect(records.length).toBeGreaterThan(0)
    expect(records.every(record => new Date(record.date.replace(' ', 'T')) <= now)).toBe(true)
    expect(records.every(record => record.model.includes('demo'))).toBe(true)
    expect(JSON.stringify(records)).not.toContain('sk-')
  })

  it('keeps card, model, group, endpoint and trend totals consistent', () => {
    const total = summarizePreview(records)
    const stats = previewDashboardStats(records, '2026-09-10')
    const charts = previewCharts(records, 'day')
    expect(stats.total_requests).toBe(records.length)
    expect(stats.total_tokens).toBe(total.total_tokens)
    for (const series of [charts.models, charts.groups, charts.endpoints, charts.trend]) {
      expect(series.reduce((sum, item) => sum + item.total_tokens, 0)).toBe(total.total_tokens)
      expect(series.reduce((sum, item) => sum + item.actual_cost, 0)).toBeCloseTo(total.actual_cost, 10)
    }
    expect(stats.by_platform?.reduce((sum, item) => sum + item.total_tokens, 0)).toBe(total.total_tokens)
  })

  it('supports date filtering, hourly grouping and empty periods', () => {
    const daily = records.filter(record => record.date.startsWith('2026-09-09'))
    expect(previewCharts(daily, 'day').trend).toHaveLength(1)
    expect(previewCharts(daily, 'hour').trend).toHaveLength(4)
    expect(previewCharts([], 'day')).toEqual({ models: [], groups: [], endpoints: [], trend: [] })
    expect(summarizePreview([]).actual_cost).toBe(0)
  })
})
