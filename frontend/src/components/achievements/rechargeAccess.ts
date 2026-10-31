import type { AchievementAccessSnapshot } from '@/api/achievements'

export interface AchievementRechargeAccess {
  supported: boolean
  blocked: boolean
  kind: 'legacy' | 'eligible' | 'required' | 'refunded' | 'pending' | 'unavailable' | 'account'
  minimum?: string
  qualified?: string
  remaining?: string
  message: string
}

// Display validation only. Authorization and the qualified total belong to the backend.
function cents(value: unknown): number | null {
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (typeof value === 'string' && !value.trim()) return null
  const amount = Number(value)
  const minor = Math.round(amount * 100)
  return Number.isFinite(amount) && amount >= 0 && Number.isSafeInteger(minor) &&
    Math.abs(amount * 100 - minor) < 0.000001 ? minor : null
}

export function achievementRechargeAccess(
  snapshot: AchievementAccessSnapshot | null,
  requireConfirmation = false,
): AchievementRechargeAccess {
  const unavailable: AchievementRechargeAccess = {
    supported: true, blocked: true, kind: 'unavailable',
    message: '暂时无法核验充值资格，相关操作暂不可用。请稍后重新核验，无需重复充值。',
  }
  if (!snapshot || !['access_unlocked', 'access_reason', 'minimum_recharge_cny', 'qualified_recharge_cny', 'remaining_recharge_cny']
    .some((key) => Object.prototype.hasOwnProperty.call(snapshot, key))) {
    return requireConfirmation ? unavailable : { supported: false, blocked: false, kind: 'legacy', message: '' }
  }
  if (snapshot.access_reason === 'account_unavailable') {
    return { supported: true, blocked: true, kind: 'account', message: '当前账户不可使用相关功能，请联系管理员核查。充值不会解除账户限制。' }
  }
  if (snapshot.access_reason === 'payment_pending' && snapshot.access_unlocked === false) {
    return { supported: true, blocked: true, kind: 'pending', message: '支付结果正在确认，请稍后刷新资格。仅到账确认后的有效充值计入门槛，请勿重复支付。' }
  }
  if (snapshot.access_reason === 'verification_unavailable') return unavailable
  const minimum = cents(snapshot.minimum_recharge_cny)
  const qualified = cents(snapshot.qualified_recharge_cny)
  const remaining = cents(snapshot.remaining_recharge_cny)
  if (minimum === null || minimum === 0 || qualified === null || remaining === null ||
    remaining !== Math.max(minimum - qualified, 0)) return unavailable
  const amounts = {
    minimum: (minimum / 100).toFixed(2), qualified: (qualified / 100).toFixed(2), remaining: (remaining / 100).toFixed(2),
  }
  if (snapshot.access_unlocked === true && snapshot.access_reason === 'eligible' && qualified >= minimum) {
    return { supported: true, blocked: false, kind: 'eligible', ...amounts,
      message: '有效充值门槛已满足；奖励仍需符合账户状态、活动规则、现金开关、预算和领取记录。' }
  }
  if (snapshot.access_unlocked === false && qualified < minimum &&
    ['recharge_required', 'recharge_refunded'].includes(snapshot.access_reason ?? '')) {
    const refunded = snapshot.access_reason === 'recharge_refunded'
    return { supported: true, blocked: true, kind: refunded ? 'refunded' : 'required', ...amounts,
      message: refunded ? '当前有效充值已扣除确认退款，尚未达到充值门槛。相关操作暂不可用，历史记录保留。' :
        `累计有效在线余额充值满 ¥${amounts.minimum} 后，可使用签到、答题活动、成就领奖和补签卡。` }
  }
  return unavailable
}

export function isAchievementAccessRestricted(error: unknown): boolean {
  if (!error || typeof error !== 'object') return false
  const value = error as { code?: unknown; reason?: unknown; response?: { data?: { code?: unknown; reason?: unknown } } }
  return [value.code, value.reason, value.response?.data?.code, value.response?.data?.reason]
    .some((code) => code === 'ACHIEVEMENT_RECHARGE_REQUIRED' || code === 'ACHIEVEMENT_ACCESS_UNAVAILABLE')
}
