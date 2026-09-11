import type { EndpointStat, GroupStat, ModelStat, TrendDataPoint } from '@/types'
import type { UserDashboardStats } from '@/api/usage'
import { formatDateLocalInput } from '@/utils/format'

export interface PreviewRecord extends TrendDataPoint {
  id: number
  model: string
  platform: string
  group: string
  key: string
  endpoint: string
  duration: number
}

export const previewKeys = [
  { name: '开发应用', suffix: 'a001', group: '演示 OpenAI', platform: 'openai', rate: 0.3 },
  { name: '文档助手', suffix: 'b002', group: '演示 Gemini', platform: 'gemini', rate: 0.4 },
  { name: '代码助手', suffix: 'c003', group: '演示 Claude', platform: 'anthropic', rate: 0.2 },
  { name: '测试应用', suffix: 'd004', group: '演示 Grok', platform: 'grok', rate: 0.5 },
  { name: '备用密钥', suffix: 'e005', group: '演示 OpenAI', platform: 'openai', rate: 0.3 }
]

export function createPreviewRecords(now = new Date()): PreviewRecord[] {
  const models = ['gpt-demo', 'gemini-demo', 'claude-demo', 'grok-demo', 'gpt-demo-mini']
  return Array.from({ length: 7 }, (_, dayIndex) => {
    const day = new Date(now)
    day.setDate(day.getDate() - dayIndex)
    return [1, 7, 13, 19].flatMap(hour => previewKeys.map((key, keyIndex) => {
      const stamp = new Date(day)
      stamp.setHours(hour, keyIndex * 7, 0, 0)
      if (stamp > now) return null
      const weight = (7 - dayIndex) * (keyIndex + 1) * (hour + 3)
      const input = weight * 130
      const output = weight * 18
      const creation = weight * 24
      const read = weight * 540
      const cost = weight * 0.0002
      return {
        id: dayIndex * 1000 + hour * 10 + keyIndex,
        date: formatDateLocalInput(stamp) + ' ' + String(hour).padStart(2, '0') + ':' + String(keyIndex * 7).padStart(2, '0'),
        requests: 1, input_tokens: input, output_tokens: output,
        cache_creation_tokens: creation, cache_read_tokens: read,
        total_tokens: input + output + creation + read, cost,
        actual_cost: cost * key.rate, model: models[keyIndex], platform: key.platform,
        group: key.group, key: key.name,
        endpoint: key.platform === 'anthropic' ? '/v1/messages' : '/v1/responses',
        duration: 18000 + weight * 19
      }
    }).filter((record): record is PreviewRecord => record !== null))
  }).flat().sort((first, second) => second.date.localeCompare(first.date))
}

export function summarizePreview(records: PreviewRecord[]): TrendDataPoint {
  return records.reduce((sum, record) => ({
    date: '', requests: sum.requests + record.requests,
    input_tokens: sum.input_tokens + record.input_tokens,
    output_tokens: sum.output_tokens + record.output_tokens,
    cache_creation_tokens: sum.cache_creation_tokens + record.cache_creation_tokens,
    cache_read_tokens: sum.cache_read_tokens + record.cache_read_tokens,
    total_tokens: sum.total_tokens + record.total_tokens,
    cost: sum.cost + record.cost, actual_cost: sum.actual_cost + record.actual_cost
  }), { date: '', requests: 0, input_tokens: 0, output_tokens: 0, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 0, cost: 0, actual_cost: 0 })
}

function grouped(records: PreviewRecord[], key: (record: PreviewRecord) => string) {
  const buckets = new Map<string, PreviewRecord[]>()
  for (const record of records) {
    const name = key(record)
    buckets.set(name, [...(buckets.get(name) || []), record])
  }
  return [...buckets].map(([name, items]) => ({ name, ...summarizePreview(items) }))
}

export function previewCharts(records: PreviewRecord[], granularity: string) {
  const models: ModelStat[] = grouped(records, record => record.model).map(item => ({ ...item, model: item.name }))
  const groups: GroupStat[] = grouped(records, record => record.group).map((item, index) => ({ ...item, group_id: index + 1, group_name: item.name }))
  const endpoints: EndpointStat[] = grouped(records, record => record.endpoint).map(item => ({ ...item, endpoint: item.name }))
  const trend: TrendDataPoint[] = grouped(records, record => granularity === 'hour' ? record.date.slice(0, 13) + ':00' : record.date.slice(0, 10))
    .map(item => ({ ...item, date: item.name })).sort((first, second) => first.date.localeCompare(second.date))
  return { models, groups, endpoints, trend }
}

export function previewDashboardStats(records: PreviewRecord[], today: string): UserDashboardStats {
  const all = summarizePreview(records)
  const daily = summarizePreview(records.filter(record => record.date.startsWith(today)))
  return {
    total_api_keys: previewKeys.length, active_api_keys: previewKeys.length,
    total_requests: all.requests, total_input_tokens: all.input_tokens, total_output_tokens: all.output_tokens,
    total_cache_creation_tokens: all.cache_creation_tokens, total_cache_read_tokens: all.cache_read_tokens,
    total_tokens: all.total_tokens, total_cost: all.cost, total_actual_cost: all.actual_cost,
    today_requests: daily.requests, today_input_tokens: daily.input_tokens, today_output_tokens: daily.output_tokens,
    today_cache_creation_tokens: daily.cache_creation_tokens, today_cache_read_tokens: daily.cache_read_tokens,
    today_tokens: daily.total_tokens, today_cost: daily.cost, today_actual_cost: daily.actual_cost,
    average_duration_ms: records.length ? records.reduce((sum, record) => sum + record.duration, 0) / records.length : 0,
    rpm: 0, tpm: 0,
    by_platform: grouped(records, record => record.platform).map(item => {
      const platformToday = summarizePreview(records.filter(record => record.platform === item.name && record.date.startsWith(today)))
      return { platform: item.name, total_requests: item.requests, total_tokens: item.total_tokens, total_actual_cost: item.actual_cost,
        today_requests: platformToday.requests, today_tokens: platformToday.total_tokens, today_actual_cost: platformToday.actual_cost }
    })
  }
}
