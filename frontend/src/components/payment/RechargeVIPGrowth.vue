<template>
  <section class="recharge-panel recharge-growth" aria-labelledby="recharge-growth-title">
    <div class="recharge-section-head">
      <h2 id="recharge-growth-title"><Icon name="gem" size="sm" />{{ t('payment.rechargePage.growth') }}</h2>
      <button type="button" class="recharge-text-link" @click="emit('show-benefits')">
        {{ t('payment.rechargePage.allBenefits') }} <Icon name="chevronRight" size="xs" />
      </button>
    </div>
    <div class="recharge-growth-line">
      <span>{{ t('payment.rechargePage.accumulated') }}</span>
      <strong>{{ usd(snapshot.total) }}</strong>
    </div>
    <div class="recharge-progress" role="progressbar" :aria-label="t('payment.rechargePage.growth')"
      aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(progress)">
      <span :style="{ width: progress + '%' }" />
    </div>
    <div class="recharge-progress-label">
      <template v-if="nextTier">
        <span>{{ t('payment.rechargePage.nextGrowthTier', { level: nextTier.level, name: tierName(nextTier.level) }) }}</span>
        <span>{{ t('payment.rechargePage.remaining', { amount: usd(nextTier.threshold - snapshot.total) }) }}</span>
      </template>
      <span v-else>{{ t('payment.rechargePage.topGrowth') }}</span>
    </div>
    <p v-if="manualTier" class="recharge-note">{{ t('payment.rechargePage.manual', { level: snapshot.tier.level }) }}</p>
    <div v-if="amount > 0" aria-live="polite" class="recharge-estimate">
      <template v-if="futureTotal === null">{{ t('payment.rechargePage.unknownGrowth') }}</template>
      <template v-else-if="upgrade">
        {{ t('payment.rechargePage.growthUpgrade', { level: upgrade.level, name: tierName(upgrade.level), total: usd(futureTotal) }) }}
        <p v-if="bonusPercent !== undefined">
          {{ t('payment.rechargePage.thisOrder') }} {{ bonusPercent }}% ·
          {{ t('payment.rechargePage.nextOrder') }} {{ upgrade.recharge_bonus_percent ?? 0 }}%
        </p>
      </template>
      <template v-else-if="nextTier && !manualTier">
        {{ t('payment.rechargePage.growthRemainingAfter', { level: nextTier.level, amount: usd(nextTier.threshold - futureTotal) }) }}
      </template>
      <template v-else>{{ t('payment.rechargePage.afterPayment') }} <strong>{{ usd(futureTotal) }}</strong></template>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { VIPSnapshot } from '@/api/vip'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  snapshot: VIPSnapshot
  amount: number
  growthRate?: number
  bonusPercent?: string
}>()
const emit = defineEmits<{ 'show-benefits': [] }>()
const { t } = useI18n()
const usd = (amount: number) => '$' + amount.toFixed(2)
const tierName = (level: number) => t(`payment.rechargePage.tier${level}`)
const manualTier = computed(() => props.snapshot.tier_source === 'manual')
const tiers = computed(() => props.snapshot.rules.tiers ?? [])
const nextIndex = computed(() => tiers.value.findIndex(tier => tier.threshold > props.snapshot.total))
const nextTier = computed(() => tiers.value[nextIndex.value])
const progress = computed(() => {
  if (!nextTier.value) return 100
  const start = tiers.value[nextIndex.value - 1]?.threshold ?? 0
  return Math.max(0, (props.snapshot.total - start) / (nextTier.value.threshold - start) * 100)
})
const futureTotal = computed(() => {
  const { amount, growthRate } = props
  if (!Number.isFinite(amount) || !growthRate || !Number.isFinite(growthRate) || growthRate <= 0) return null
  return Math.round((props.snapshot.total + amount * growthRate) * 1e8) / 1e8
})
const upgrade = computed(() => {
  const total = futureTotal.value
  if (manualTier.value || total === null || !nextTier.value || total < nextTier.value.threshold) return null
  return tiers.value.filter(tier => tier.threshold <= total).at(-1)
})
</script>
