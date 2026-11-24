<template>
  <section class="recharge-panel recharge-growth" :aria-label="t('payment.rechargePage.growth')">
    <div class="recharge-section-head">
      <h2><Icon name="badge" size="sm" />{{ t('payment.rechargePage.growth') }}</h2>
      <button v-if="state?.enabled" class="recharge-text-link" type="button" @click="emit('update:show', true)">
        {{ t('payment.rechargePage.allBenefits') }} <Icon name="chevronRight" size="xs" />
      </button>
    </div>
    <div v-if="pending" data-vip-skeleton aria-busy="true" class="recharge-growth-loading">{{ t('common.loading') }}</div>
    <template v-else-if="state?.enabled">
      <div class="recharge-growth-line"><span>{{ t('payment.rechargePage.accumulated') }}</span><strong>{{ usd(state.total) }}</strong></div>
      <div class="recharge-progress" role="progressbar" :aria-label="t('payment.rechargePage.growth')"
        aria-valuemin="0" aria-valuemax="100" :aria-valuenow="Math.round(progress)">
        <span :style="{ width: progress + '%' }" />
      </div>
      <div class="recharge-progress-label">
        <template v-if="nextGrowthTier">
          <span>VIP {{ nextGrowthTier.level }} · {{ tierName(nextGrowthTier.level) }}</span>
          <span>{{ t('payment.rechargePage.remaining', { amount: usd(nextGrowthTier.threshold - state.total) }) }}</span>
        </template>
        <span v-else>{{ t('payment.rechargePage.topGrowth') }}</span>
      </div>
      <p v-if="state.tier_source === 'manual'" class="recharge-note">{{ t('payment.rechargePage.manual', { level: state.tier.level }) }}</p>
      <div v-if="amount > 0" aria-live="polite" class="recharge-estimate">
        <template v-if="projected && futureTotal !== null">
          {{ t('payment.rechargePage.afterPayment') }} <strong data-future-total>{{ usd(futureTotal) }}</strong> · <span data-projected-tier>VIP {{ projected.level }}</span>
          <p v-if="bonusEnabled && projected.level !== state.tier.level">
            {{ t('payment.rechargePage.thisOrder') }} <span data-current-bonus>{{ currentBonus }}%</span> ·
            {{ t('payment.rechargePage.nextOrder') }} <span data-projected-bonus>{{ projected.recharge_bonus_percent ?? 0 }}%</span>
          </p>
        </template>
        <template v-else>{{ t('payment.rechargePage.unknownGrowth') }}</template>
      </div>
    </template>
    <p v-else-if="error" role="status" class="recharge-note">
      {{ t('payment.rechargePage.loadError') }} <button class="recharge-text-link" type="button" @click="load(false)">{{ t('payment.rechargePage.retry') }}</button>
    </p>
    <p v-else class="recharge-note">{{ t('payment.rechargePage.vipOff') }}</p>
  </section>

  <BaseDialog v-if="state?.enabled" :show="show" :title="t('payment.rechargePage.allBenefits')" width="extra-wide" @close="emit('update:show', false)">
    <div class="recharge-theme recharge-benefits">
      <div class="recharge-tier-picker" role="group" :aria-label="t('payment.rechargePage.browseTiers')">
        <button v-for="tier in tiers" :key="tier.level" type="button" :aria-pressed="selectedLevel === tier.level"
          :aria-label="'VIP ' + tier.level + ' · ' + tierName(tier.level)" @click="selectedLevel = tier.level">
          <VIPMembershipCard variant="thumbnail" :level="tier.level" :name="tierName(tier.level)" threshold-label="" :benefits="[]" />
          <span class="recharge-tier-caption">{{ tierName(tier.level) }}<Icon v-if="selectedLevel === tier.level" name="check" size="xs" /></span>
        </button>
      </div>
      <div v-if="selectedTier" class="recharge-tier-detail">
        <div class="recharge-showcase">
          <VIPMembershipCard :level="selectedTier.level" :name="tierName(selectedLevel)"
            :current="isCurrent" :current-label="t('payment.rechargePage.currentTier')" :threshold-label="thresholdLabel(selectedTier)"
            :benefits="[discountLabel, t('payment.rechargePage.cardConcurrency', { value: concurrencyLabel }), t('payment.rechargePage.cardRebate', { value: rebatePercent })]" />
          <dl class="recharge-benefit-figures">
            <div><dt>{{ t('payment.rechargePage.onlineBonus') }}</dt><dd>{{ tierBonus(selectedTier) }}</dd><small>{{ t('payment.rechargePage.atOrder') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.daily') }}</dt><dd>{{ usd(state.rules.daily_rewards?.[selectedLevel]) }}</dd><small>{{ t('payment.rechargePage.ifEligible') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.invites') }}</dt><dd>{{ rebatePercent }}%</dd><small>{{ t('payment.rechargePage.validInvites') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.milestone') }}</dt><dd>{{ usd(reward(selectedLevel)?.amount) }}</dd>
              <small>{{ t('payment.rechargePage.' + (reward(selectedLevel)?.status ?? 'oncePerTier')) }}</small></div>
          </dl>
        </div>
        <dl class="recharge-usage">
          <div><dt>{{ t('payment.rechargePage.concurrency') }}</dt><dd>{{ concurrencyLabel }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.rpm') }}</dt><dd>{{ rpmLabel }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.groups') }}</dt><dd>{{ discountLabel }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.exclusive') }}</dt><dd>{{ accessLabel }}</dd></div>
        </dl>
        <p class="recharge-note">{{ t('payment.rechargePage.pricingExample', pricingExample) }}</p>
      </div>
      <p v-if="membershipError" class="recharge-note" role="status">
        {{ t('payment.rechargePage.detailError') }} <button class="recharge-text-link" type="button" @click="loadMembership">{{ t('payment.rechargePage.retry') }}</button>
      </p>
      <details>
        <summary>{{ t('payment.rechargePage.compare') }}</summary>
        <div class="recharge-comparison" tabindex="0" role="region" :aria-label="t('payment.rechargePage.compare')">
          <table><thead><tr>
            <th v-for="field in ['grade', 'threshold', 'onlineBonus', 'daily', 'invites', 'concurrency', 'milestone']" :key="field" scope="col">{{ t('payment.rechargePage.' + field) }}</th>
          </tr></thead><tbody>
            <tr v-for="tier in tiers" :key="tier.level"><th scope="row">VIP {{ tier.level }} · {{ tierName(tier.level) }}</th>
              <td>{{ tier.level ? usd(tier.threshold) : t('payment.rechargePage.atSignup') }}</td><td>{{ tierBonus(tier) }}</td>
              <td>{{ usd(state.rules.daily_rewards?.[tier.level]) }}</td><td>{{ tier.rebate_percent }}%</td>
              <td>{{ tier.concurrency || t('payment.rechargePage.accountDefault') }}</td><td>{{ usd(reward(tier.level)?.amount) }}</td>
            </tr>
          </tbody></table>
        </div>
      </details>
      <details>
        <summary>{{ t('payment.rechargePage.rules') }}</summary>
        <ul><li v-for="rule in ['principalRule', 'bonusRule', 'rewardRule', 'dailyRule', 'groupRule', 'capacityRule']" :key="rule">{{ t('payment.rechargePage.' + rule) }}</li></ul>
      </details>
      <div class="recharge-benefit-links">
        <RouterLink to="/vip" data-vip-center>{{ t('payment.rechargePage.vipCenter') }} <Icon name="chevronRight" size="xs" /></RouterLink>
        <RouterLink to="/docs?cat=membership&amp;page=vip">{{ t('payment.rechargePage.guide') }} <Icon name="chevronRight" size="xs" /></RouterLink>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getVIP, getVIPMembership, type VIPMembership, type VIPSnapshot, type VIPTier } from '@/api/vip'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import VIPMembershipCard from '@/components/user/VIPMembershipCard.vue'
import { formatRechargeBonusNumber } from '@/utils/rechargeBonus'

const props = defineProps<{
  show: boolean
  amount: number
  currency: string
  growthRates?: Record<string, number>
  currentMultiplier: number
  bonusEnabled: boolean
  initialRequest?: Promise<VIPSnapshot | null>
}>()
const emit = defineEmits<{ 'update:show': [show: boolean]; snapshot: [value: VIPSnapshot | null] }>()
const { t } = useI18n()
const state = ref<VIPSnapshot | null>(null)
const membership = ref<VIPMembership | null>(null)
const pending = ref(true)
const error = ref(false)
const membershipError = ref(false)
const loadingMembership = ref(false)
const selectedLevel = ref(0)

const baseTier: VIPTier = { level: 0, threshold: 0, concurrency: 0, rpm: 0, rebate_percent: 0, recharge_bonus_percent: 0 }
const tiers = computed(() => [baseTier, ...(state.value?.rules.tiers ?? [])])
const nextGrowthTier = computed(() => tiers.value.find(tier => tier.threshold > (state.value?.total ?? 0)))
const progress = computed(() => {
  if (!state.value || !nextGrowthTier.value) return 100
  const total = state.value.total
  const start = tiers.value.filter(tier => tier.threshold <= total).at(-1)?.threshold ?? 0
  return Math.max(0, Math.min(100, (total - start) / (nextGrowthTier.value.threshold - start) * 100))
})
const futureTotal = computed(() => {
  const rate = props.growthRates?.[props.currency]
  if (!state.value || !rate || rate <= 0 || !Number.isFinite(rate) || !Number.isFinite(props.amount) || props.amount < 0) return null
  return Math.round((state.value.total + props.amount * rate) * 1e8) / 1e8
})
const projected = computed(() => {
  const total = futureTotal.value
  if (!state.value || total === null) return null
  if (state.value.tier_source === 'manual') return state.value.tier
  return tiers.value.filter(tier => tier.threshold <= total).at(-1) ?? baseTier
})
const currentBonus = computed(() => formatRechargeBonusNumber((props.currentMultiplier - 1) * 100))
const selectedTier = computed(() => tiers.value.find(tier => tier.level === selectedLevel.value))
const isCurrent = computed(() => selectedLevel.value === state.value?.tier.level)
const concurrencyLabel = computed(() => (isCurrent.value ? state.value?.concurrency : selectedTier.value?.concurrency) || t('payment.rechargePage.accountDefault'))
const rpmLabel = computed(() => (isCurrent.value ? state.value?.rpm : selectedTier.value?.rpm) || t('payment.rechargePage.accountDefault'))
const rebatePercent = computed(() => (isCurrent.value ? state.value?.rebate_percent : undefined) ?? selectedTier.value?.rebate_percent ?? 0)
const discountLabel = computed(() => {
  if (!membership.value) return '—'
  const cut = membership.value.discount_summaries?.[selectedLevel.value]?.[0]
  return cut ? t('payment.rechargePage.maxCut', { value: cut }) : t('payment.rechargePage.groupPricing')
})
// The VIP guide illustrates a 0.400 base rate, 0.015 reduction and 1% bonus per tier.
const pricingExample = computed(() => {
  const level = selectedLevel.value
  const rate = 0.4 - level * 0.015
  const savings = 100 * (1 - rate / (0.4 * (1 + level / 100)))
  return { rate: rate.toFixed(3), bonus: level, savings: savings.toFixed(2) }
})
const accessLabel = computed(() => isCurrent.value && state.value?.groups?.some(group => group.exclusive && group.granted)
  ? t('payment.rechargePage.openAccess')
  : t('payment.rechargePage.accessFrom', { threshold: usd(state.value?.rules.access_threshold) }))

const tierName = (level: number) => t(`payment.rechargePage.tier${level}`)
const usd = (n?: number) => n == null ? '—' : '$' + n.toFixed(2)
const tierBonus = (tier: VIPTier) => props.bonusEnabled ? (tier.recharge_bonus_percent ?? 0) + '%' : t('payment.rechargePage.inactive')
const reward = (level: number) => membership.value?.rewards?.find(item => item.level === level)
const thresholdLabel = (tier: VIPTier) => tier.level
  ? t('payment.rechargePage.thresholdFrom', { amount: usd(tier.threshold) }) : t('payment.rechargePage.atSignup')

async function loadMembership() {
  if (membership.value || loadingMembership.value) return
  loadingMembership.value = true
  membershipError.value = false
  try { membership.value = await getVIPMembership() }
  catch { membershipError.value = true }
  finally { loadingMembership.value = false }
}

watch([() => props.show, () => state.value?.enabled], ([open]) => {
  if (open && state.value?.enabled) {
    selectedLevel.value = state.value.tier.level
    void loadMembership()
  }
})

async function load(useInitial = true) {
  pending.value = true
  error.value = false
  try {
    const result = useInitial && props.initialRequest ? await props.initialRequest : await getVIP()
    if (!result) throw new Error('VIP snapshot unavailable')
    state.value = result
    emit('snapshot', result)
  } catch {
    error.value = true
    state.value = null
    emit('snapshot', null)
  } finally {
    pending.value = false
  }
}
onMounted(() => load())
</script>
