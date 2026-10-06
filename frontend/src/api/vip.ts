import { apiClient } from './client'

export interface VIPTier {
  level: number
  threshold: number
  concurrency: number
  rpm: number
  rebate_percent: number
  recharge_bonus_percent?: number
}
export interface VIPGroupRule {
  group_id: number
  access: boolean
  private: boolean
  floor: number
  discounts: number[]
}
export interface VIPRules {
  daily_rewards?: number[]
  enabled: boolean
  recharge_bonus_enabled?: boolean
  currency: string
  access_threshold: number
  exchange_rates: Record<string, number>
  tiers: VIPTier[]
  groups: VIPGroupRule[]
}
export interface VIPOverride {
  benefit: string
  value: number
  expires_at: string | null
  reason: string
}
export interface VIPGroup {
  id: number
  name: string
  platform: string
  exclusive: boolean
  subscription: string
  base_rate: number
  rate: number
  granted: boolean
  participating: boolean
}
export interface VIPSnapshot {
  user?: { id: number; email: string; username: string }
  growth_tier?: VIPTier
  tier_source?: string
  level_override?: VIPOverride | null
  base_concurrency?: number
  base_rpm?: number
  enabled: boolean
  total: number
  tier: VIPTier
  badge_level: number
  concurrency: number
  rpm: number
  rebate_percent: number
  next: VIPTier | null
  rules: Omit<VIPRules, 'groups' | 'exchange_rates'> & {
    groups: VIPGroupRule[] | null
    exchange_rates: Record<string, number> | null
  }
  groups: VIPGroup[]
  ledger: {
    id: number
    source: string
    amount: number
    reason: string
    created_at: string
  }[]
  overrides: VIPOverride[]
}
export interface VIPReward {
  level: number
  threshold: number
  amount: number
  status: 'locked' | 'available' | 'claimed' | 'revoked'
}
export interface VIPMembership {
  rewards: VIPReward[]
  seats: { name: string; level: number }[]
  debt: number
  claimed: number
  discount_summaries: Record<number, number[]>
}
export async function getVIPMembership(): Promise<VIPMembership> {
  const { data } = await apiClient.get<VIPMembership>('/user/vip/membership')
  return data
}
export async function claimVIPReward(
  level: number,
): Promise<{ amount: number }> {
  const { data } = await apiClient.post<{ amount: number }>(
    `/user/vip/rewards/${level}/claim`,
  )
  return data
}
export async function getVIP(userId?: number): Promise<VIPSnapshot> {
  const { data } = await apiClient.get<VIPSnapshot>(
    userId ? `/admin/vip/users/${userId}` : '/user/vip',
  )
  return data
}
export async function getVIPRules(): Promise<VIPRules> {
  const { data } = await apiClient.get<VIPRules>('/admin/vip/rules')
  return data
}
export async function saveVIPRules(rules: VIPRules): Promise<void> {
  await apiClient.put('/admin/vip/rules', rules)
}
export async function saveVIPOverride(
  id: number,
  override: VIPOverride,
): Promise<void> {
  await apiClient.put(`/admin/vip/users/${id}/overrides`, override)
}
export async function clearVIPOverride(
  id: number,
  benefit: string,
): Promise<void> {
  await apiClient.delete(`/admin/vip/users/${id}/overrides/${benefit}`)
}
export async function createVIPOpening(
  id: number,
  amount: number,
  reason: string,
): Promise<void> {
  await apiClient.post(`/admin/vip/users/${id}/opening`, { amount, reason })
}

export const setVIPLevel = async (
  id: number,
  body: { level: number; expires_at: string | null; reason: string },
) => {
  await apiClient.put(`/admin/vip/users/${id}/level`, body)
}
export const restoreVIPLevel = async (id: number, reason: string) => {
  await apiClient.delete(`/admin/vip/users/${id}/level`, { data: { reason } })
}
