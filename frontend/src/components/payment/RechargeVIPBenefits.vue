<template>
  <section class="recharge-order-perks" aria-labelledby="recharge-perks-title">
    <div class="recharge-section-head">
      <h3 id="recharge-perks-title">
        <Icon name="badge" size="sm" />
        {{ state?.enabled ? t('payment.rechargePage.currentBenefits', { level: state.tier.level }) : t('payment.rechargePage.membership') }}
      </h3>
      <button v-if="state?.enabled" class="recharge-text-link" type="button" @click="emit('update:show', true)">
        {{ t('payment.rechargePage.compare') }} <Icon name="chevronRight" size="xs" />
      </button>
    </div>
    <div v-if="pending" data-vip-skeleton aria-busy="true" class="recharge-growth-loading">{{ t('common.loading') }}</div>
    <template v-else-if="state?.enabled && currentBenefits">
      <dl class="recharge-perk-list" :aria-busy="loadingMembership">
        <div v-for="(value, field) in currentBenefits" :key="field">
          <dt>{{ t(`payment.rechargePage.${field}`) }}</dt>
          <dd>
            {{ value }}
            <small v-if="field === 'daily'">{{ t('payment.rechargePage.ifEligible') }}</small>
            <small v-if="field === 'milestone' && currentReward">{{ t(`payment.rechargePage.${currentReward.status}`) }}</small>
          </dd>
        </div>
      </dl>
      <p v-if="membershipError" class="recharge-note" role="status">
        {{ t('payment.rechargePage.detailError') }} <button class="recharge-text-link" type="button" @click="loadMembership">{{ t('payment.rechargePage.retry') }}</button>
      </p>
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
      <div v-if="selectedTier && selectedBenefits" class="recharge-tier-detail">
        <div class="recharge-showcase">
          <VIPMembershipCard :level="selectedTier.level" :name="tierName(selectedLevel)"
            :current="isCurrent" :current-label="t('payment.rechargePage.currentTier')" :threshold-label="thresholdLabel(selectedTier)"
            :benefits="[selectedBenefits.groups, t('payment.rechargePage.cardConcurrency', { value: selectedBenefits.concurrency }), t('payment.rechargePage.cardRebate', { value: selectedBenefits.invites })]" />
          <dl class="recharge-benefit-figures">
            <div><dt>{{ t('payment.rechargePage.onlineBonus') }}</dt><dd>{{ selectedBenefits.onlineBonus }}</dd><small>{{ t('payment.rechargePage.atOrder') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.daily') }}</dt><dd>{{ selectedBenefits.daily }}</dd><small>{{ t('payment.rechargePage.ifEligible') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.invites') }}</dt><dd>{{ selectedBenefits.invites }}</dd><small>{{ t('payment.rechargePage.validInvites') }}</small></div>
            <div><dt>{{ t('payment.rechargePage.milestone') }}</dt><dd>{{ selectedBenefits.milestone }}</dd>
              <small v-if="selectedLevel">{{ t('payment.rechargePage.' + (reward(selectedLevel)?.status ?? 'oncePerTier')) }}</small></div>
          </dl>
        </div>
        <dl class="recharge-usage">
          <div><dt>{{ t('payment.rechargePage.concurrency') }}</dt><dd>{{ selectedBenefits.concurrency }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.rpm') }}</dt><dd>{{ selectedBenefits.rpm }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.groups') }}</dt><dd>{{ selectedBenefits.groups }}</dd></div>
          <div><dt>{{ t('payment.rechargePage.exclusive') }}</dt><dd>{{ selectedBenefits.exclusive }}</dd></div>
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
const currentBonus = computed(() => formatRechargeBonusNumber((props.currentMultiplier - 1) * 100))
const selectedTier = computed(() => tiers.value.find(tier => tier.level === selectedLevel.value))
const isCurrent = computed(() => selectedLevel.value === state.value?.tier.level)
const currentReward = computed(() => state.value && reward(state.value.tier.level))
const currentBenefits = computed(() => state.value && benefitsFor(state.value, state.value.tier))
const selectedBenefits = computed(() => state.value && selectedTier.value && benefitsFor(state.value, selectedTier.value))

function benefitsFor(snapshot: VIPSnapshot, tier: VIPTier) {
  const current = tier.level === snapshot.tier.level
  const concurrency = current ? snapshot.concurrency : tier.concurrency
  const rpm = current ? snapshot.rpm : tier.rpm
  const rebate = current ? snapshot.rebate_percent ?? tier.rebate_percent : tier.rebate_percent
  const cut = membership.value?.discount_summaries?.[tier.level]?.[0]
  const groupPrice = cut ? t('payment.rechargePage.maxCut', { value: cut }) : t('payment.rechargePage.groupPricing')
  const granted = snapshot.groups?.some(group => group.exclusive && group.granted)
  const threshold = t('payment.rechargePage.accessFrom', { threshold: usd(snapshot.rules.access_threshold) })
  return {
    onlineBonus: current && props.bonusEnabled ? currentBonus.value + '%' : tierBonus(tier),
    daily: usd(snapshot.rules.daily_rewards?.[tier.level]),
    invites: rebate + '%',
    milestone: tier.level ? usd(reward(tier.level)?.amount) : t('payment.rechargePage.noMilestone'),
    concurrency: concurrency || t('payment.rechargePage.accountDefault'),
    rpm: rpm || t('payment.rechargePage.accountDefault'),
    groups: membership.value ? groupPrice : '—',
    exclusive: current ? t(granted ? 'payment.rechargePage.openAccess' : 'payment.rechargePage.noAccess') : threshold,
  }
}
// The VIP guide illustrates a 0.400 base rate, 0.015 reduction and 1% bonus per tier.
const pricingExample = computed(() => {
  const level = selectedLevel.value
  const rate = 0.4 - level * 0.015
  const savings = 100 * (1 - rate / (0.4 * (1 + level / 100)))
  return { rate: rate.toFixed(3), bonus: level, savings: savings.toFixed(2) }
})
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
    if (result.enabled) void loadMembership()
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
