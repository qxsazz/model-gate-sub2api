import { apiClient } from './client'

export interface MonthlyCheckinCredit {
  available: number
  month: string
  expires_at: string
}

export interface LegacyCheckinCredit {
  available: number
  next_amount: number
  next_expiry: string
}

export interface WalletBalance {
  available_balance: number
  frozen_balance: number
  other_available: number
  checkin_credit: MonthlyCheckinCredit
  legacy_checkin_credit?: LegacyCheckinCredit | null
}

export async function getWalletBalance(signal?: AbortSignal): Promise<WalletBalance> {
  const { data } = await apiClient.get<WalletBalance>('/user/balance', { signal })
  return data
}
