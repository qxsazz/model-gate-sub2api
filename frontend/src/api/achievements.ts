import { apiClient } from './client'
export interface CashReceipt {
  gross: number
  net: number
  offset_amount: number
  replayed?: boolean
  revoked_at?: string | null
  cards_awarded?: number
  card_balance?: number
  cards_reclaimed?: number
  series_recovered?: number
  series_debt_added?: number
}
export interface AchievementReceipt extends CashReceipt {
  source?: 'user' | 'admin' | 'card'
  reason: string
  day: string
  tier: number
  streak: number
}
export interface Medal {
  key: string
  category: string
  name: string
  description: string
  target: number
  reward: number
  preview: boolean
  progress: number
  unlocked: boolean
  claim: CashReceipt | null
  manual?: 'granted' | 'revoked' | null
  card_reward?: number
  card_claim?: { amount: number; used: number; reclaimed: number } | null
}
export interface AchievementSeries {
  key: string
  name: string
  reward: number
  collected: number
  total: number
  unlocked: boolean
  preview?: boolean
  prior_amount?: number
  claimable_amount?: number
  claim: (CashReceipt & { prior_amount: number }) | null
}
export interface CardHistory {
  id: number
  key: string
  delta: number
  kind: 'claim' | 'use' | 'reclaim'
  day: string | null
}
export interface AchievementState {
  date: string
  timezone: string
  tier: number
  daily_amount: number
  daily_rewards: number[]
  cash_reason: string
  milestone_cash_enabled: boolean
  milestone_cash_reason?: string
  today: AchievementReceipt | null
  total_days: number
  streak: number
  longest: number
  calendar: string[]
  history: AchievementReceipt[]
  equipment: string | null
  passes: { kind: string; topic: string }[]
  medals: Medal[]
  card_balance?: number
  card_min_date?: string
  card_max_date?: string
  card_history?: CardHistory[]
  series?: AchievementSeries[]
}
export interface AchievementConfig {
  cash_scope: 'all' | 'allowlist'
  budget_enabled: boolean
  daily_rewards: number[]
  cash_enabled: boolean
  milestone_cash_enabled: boolean
  cash_allowlist: number[]
  daily_budget: number
  monthly_budget: number
}
export interface AchievementAudit {
  id: number
  actor_id: number
  user_id: number
  action: string
  key: string | null
  day: string | null
  reason: string
  response: CashReceipt
  created_at: string
}
export interface AdminAchievementState extends AchievementState {
  user: {
    id: number
    email: string
    username: string
    balance: number
    status: string
    created_at: string
  }
  operations: AchievementAudit[]
}
export interface BackfillPreview {
  day: string
  tier: number
  gross: number
  existing: boolean
  policy_known: boolean
  vip_total?: number
  reason?: string
}
export interface CardPreview extends BackfillPreview {
  card_balance: number
  cash_reason: string
  available: boolean
}
export interface CardCommand {
  date: string
  request_key: string
  expected_gross: number
  expected_tier: number
}
export const previewAchievementCard = async (date: string) =>
  (
    await apiClient.get<CardPreview>('/user/achievements/card-preview', {
      params: { date },
    })
  ).data
export const useAchievementCard = async (body: CardCommand) =>
  (
    await apiClient.post<
      AchievementReceipt & {
        cards_spent: number
        card_balance: number
        existing: boolean
      }
    >('/user/achievements/use-card', body)
  ).data
export interface AchievementAdminBody {
  expected_gross?: number
  expected_tier?: number
  key?: string
  date?: string
  reason: string
  request_key: string
  grant_reward?: boolean
  reclaim_reward?: boolean
}
export const getAdminAchievements = async (id: number) =>
  (
    await apiClient.get<AdminAchievementState>(
      `/admin/achievements/users/${id}`,
    )
  ).data
export const previewAchievementBackfill = async (id: number, date: string) =>
  (
    await apiClient.get<BackfillPreview>(
      `/admin/achievements/users/${id}/backfill-preview`,
      { params: { date } },
    )
  ).data
export const adminAchievementAction = async (
  id: number,
  action: 'backfill' | 'grant' | 'revoke' | 'restore',
  body: AchievementAdminBody,
) =>
  (
    await apiClient.post<
      CashReceipt & {
        saved: boolean
        action: string
        existing?: boolean
        recovered?: number
        debt_added?: number
      }
    >(`/admin/achievements/users/${id}/${action}`, body)
  ).data
export const getAchievementAudit = async () =>
  (await apiClient.get<AchievementAudit[]>('/admin/achievements/audit')).data
export const getAchievements = async () =>
  (await apiClient.get<AchievementState>('/user/achievements')).data
type AchievementBody = { key?: string; date?: string; request_key?: string }
export function changeAchievement(
  action: 'checkin',
  body: AchievementBody,
): Promise<AchievementReceipt>
export function changeAchievement(
  action: 'checkin' | 'claim' | 'claim_series',
  body: AchievementBody,
): Promise<CashReceipt>
export function changeAchievement(
  action: 'equip',
  body: AchievementBody,
): Promise<{ saved: boolean }>
export async function changeAchievement(
  action: 'checkin' | 'claim' | 'claim_series' | 'equip',
  body: AchievementBody,
): Promise<CashReceipt | { saved: boolean }> {
  return (await apiClient.post(`/user/achievements/${action}`, body)).data
}
export const getAchievementConfig = async () =>
  (await apiClient.get<AchievementConfig>('/admin/achievements/config')).data
export const saveAchievementConfig = async (c: AchievementConfig) => {
  await apiClient.put('/admin/achievements/config', c)
}
export interface ActivityTopic {
  kind: string
  key: string
  name: string
  description: string
}
export interface ActivityQuiz {
  topic: ActivityTopic
  questions: { prompt: string; options: string[] }[]
}
export const getActivityTopics = async () =>
  (await apiClient.get<ActivityTopic[]>('/user/achievements/activity/topics'))
    .data
export const getActivityQuiz = async (kind: string, topic: string) =>
  (
    await apiClient.get<ActivityQuiz>(
      `/user/achievements/activity/${kind}/${topic}`,
    )
  ).data
export const submitActivityQuiz = async (
  kind: string,
  topic: string,
  answers: number[],
  request_key: string,
) =>
  (
    await apiClient.post<{ score: number; passed: boolean; replayed: boolean }>(
      `/user/achievements/activity/${kind}/${topic}`,
      { answers, request_key },
    )
  ).data
