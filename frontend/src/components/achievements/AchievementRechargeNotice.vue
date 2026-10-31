<template>
  <section v-if="access.supported" class="recharge-notice" :class="{ 'is-unlocked': !access.blocked }" aria-labelledby="achievement-recharge-heading" role="status" aria-live="polite">
    <div>
      <h2 id="achievement-recharge-heading">{{ heading }}</h2>
      <p>{{ access.message }}</p>
      <p v-if="needsRecharge" class="recharge-amounts">
        当前有效充值 <strong>¥{{ access.qualified }}</strong>，还差 <strong>¥{{ access.remaining }}</strong>
        <span v-if="access.kind === 'refunded'">（当前门槛 ¥{{ access.minimum }}）</span>。
      </p>
      <details v-if="access.kind !== 'account'">
        <summary>有效充值如何计算？</summary>
        <p>仅统计已确认到账的在线余额充值本金，可多笔累计，扣除已确认退款。手续费、充值赠金、签到和活动奖励、管理员赠送、订阅购买不计入；正常消费余额不会影响资格。</p>
        <p>历史记录、成长和徽章保留。退款后不足门槛会重新锁定相关操作，再次达标不会重置已领取奖励。既有奖励追回规则继续生效。</p>
        <p>差额是资格缺口，实际可充值金额以充值页为准。达到门槛不代表一定获得奖励，仍需满足对应成就和发奖规则。</p>
      </details>
    </div>
    <div v-if="access.blocked" class="recharge-actions">
      <RouterLink v-if="needsRecharge" class="primary" to="/purchase">去余额充值</RouterLink>
      <button type="button" class="text-button" :disabled="refreshing" @click="$emit('refresh')">
        {{ refreshing ? '正在核验…' : needsRecharge ? '已充值，刷新资格' : '重新核验' }}
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { AchievementRechargeAccess } from './rechargeAccess'
const props = defineProps<{ access: AchievementRechargeAccess; refreshing: boolean }>()
defineEmits<{ refresh: [] }>()
const needsRecharge = computed(() => ['required', 'refunded'].includes(props.access.kind))
const heading = computed(() => ({
  legacy: '', eligible: '有效充值门槛已满足', required: '充值解锁签到与成就奖励',
  refunded: '有效充值不足，相关功能暂时锁定', pending: '等待充值到账确认',
  unavailable: '充值资格暂时无法核验', account: '当前账户不可用',
})[props.access.kind])
</script>

<style scoped>
.recharge-notice { display: flex; justify-content: space-between; align-items: flex-start; gap: 20px; padding: 20px; margin-bottom: 20px; border: 1px solid var(--line); border-radius: 16px; background: var(--wash); color: var(--ink); }
.recharge-notice h2 { margin: 0 0 8px; font-size: 1rem; font-weight: 650; }
.recharge-notice p { margin: 6px 0; line-height: 1.7; font-size: .875rem; }
.recharge-notice summary { cursor: pointer; font-size: .8rem; margin-top: 8px; }
.recharge-actions { display: flex; gap: 10px; flex-shrink: 0; flex-wrap: wrap; align-items: center; }
.recharge-actions a, .recharge-actions button { padding: 9px 12px; font-size: .875rem; border-radius: 8px; text-decoration: none; }
.recharge-actions a { background: var(--gold); color: var(--surface); }
.recharge-actions button { border: 1px solid currentColor; background: transparent; color: inherit; }
.recharge-actions button:disabled { opacity: .6; cursor: wait; }
.recharge-actions a:focus-visible, .recharge-actions button:focus-visible, summary:focus-visible { outline: 2px solid currentColor; outline-offset: 3px; }
.is-unlocked { color: var(--green); }
@media (max-width: 640px) { .recharge-notice { flex-direction: column; gap: 12px; padding: 16px; } .recharge-actions { width: 100%; } }
</style>
