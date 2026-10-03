import { apiClient } from './client'
export interface CashReceipt {
  gross: number
  net: number
  offset_amount: number
  replayed?: boolean
  revoked_at?: string | null
}
export interface AchievementReceipt extends CashReceipt {
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
}
export interface AchievementState {
  date: string
  timezone: string
  tier: number
  daily_amount: number
  daily_rewards: number[]
  cash_reason: string
  milestone_cash_enabled: boolean
  today: AchievementReceipt | null
  total_days: number
  streak: number
  longest: number
  calendar: string[]
  history: AchievementReceipt[]
  equipment: string | null
  passes: { kind: string; topic: string }[]
  medals: Medal[]
}
export interface AchievementConfig {
  daily_rewards: number[]
  cash_enabled: boolean
  milestone_cash_enabled: boolean
  cash_allowlist: number[]
  daily_budget: number
  monthly_budget: number
}
export const getAchievements = async () =>
  (await apiClient.get<AchievementState>('/user/achievements')).data
type AchievementBody = { key?: string; date?: string; request_key?: string }
export function changeAchievement(
  action: 'checkin',
  body: AchievementBody,
): Promise<AchievementReceipt>
export function changeAchievement(
  action: 'checkin' | 'claim',
  body: AchievementBody,
): Promise<CashReceipt>
export function changeAchievement(
  action: 'equip',
  body: AchievementBody,
): Promise<{ saved: boolean }>
export async function changeAchievement(
  action: 'checkin' | 'claim' | 'equip',
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
