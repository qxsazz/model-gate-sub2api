<template>
  <div>
    <div class="recharge-section-head">
      <h2><Icon name="dollar" size="sm" />{{ t('payment.rechargePage.chooseAmount') }}</h2>
      <span class="recharge-unit">{{ currency }}</span>
    </div>
    <div id="recharge-amounts" class="recharge-amounts" role="group" :aria-label="t('payment.quickAmounts')">
      <button v-for="amt in visibleAmounts" :key="amt" type="button" class="recharge-amount"
        :aria-pressed="modelValue === amt" :data-testid="`quick-amount-${amt}`" @click="selectAmount(amt)">
        <span class="recharge-amount-value"><small>{{ currencySymbol(currency) }}</small>{{ amt.toLocaleString() }}</span>
        <span v-if="quoteFor(amt).percent > 0" class="recharge-tier-offer" data-testid="quick-amount-bonus-badge">{{ badgeText(amt) }}</span>
        <span v-if="showSecondLine" class="recharge-amount-detail" data-testid="quick-amount-credited">{{ secondLine(amt) }}</span>
        <Icon v-if="modelValue === amt" name="check" size="xs" class="recharge-selected-check" />
      </button>
    </div>
    <button v-if="filteredAmounts.length > 6" class="recharge-more" type="button"
      :aria-expanded="expanded" aria-controls="recharge-amounts" @click="expanded = !expanded">
      {{ t(expanded ? 'payment.rechargePage.lessAmounts' : 'payment.rechargePage.moreAmounts') }} <Icon name="chevronDown" size="xs" />
    </button>
    <div class="recharge-custom">
      <label for="recharge-custom-amount">{{ t('payment.customAmount') }}</label>
      <div><span aria-hidden="true">{{ currencySymbol(currency) }}</span>
        <input id="recharge-custom-amount" type="text" inputmode="decimal" autocomplete="off" :value="customText"
          :placeholder="placeholderText" @input="handleInput" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'
import type { RechargeBonusTier } from '@/types/payment'
import { formatRechargeBonusNumber, quoteRechargeBonus, type RechargeBonusMode } from '@/utils/rechargeBonus'
import { currencySymbol, formatPaymentAmount } from './currency'

const props = withDefaults(defineProps<{
  amounts?: number[]
  modelValue: number | null
  min?: number
  max?: number
  /** 充值优惠阶梯（按 min_amount 升序）；为空时不显示价签与第二行 */
  bonusTiers?: RechargeBonusTier[]
  /** 阶梯模式：bonus 赠金 / discount 折扣 */
  bonusMode?: RechargeBonusMode
  /** 充值倍率（1 支付币种 = multiplier USD），用于计算到账金额 */
  multiplier?: number
  /** 支付币种（折扣模式第二行实付金额的币种与精度） */
  currency?: string
}>(), {
  amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
  min: 0,
  max: 0,
  currency: 'USD',
  bonusTiers: () => [],
  bonusMode: 'bonus',
  multiplier: 1,
})

const emit = defineEmits<{
  'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

// 0 = no limit
const filteredAmounts = computed(() =>
  props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const expanded = ref(false)
const visibleAmounts = computed(() => expanded.value ? filteredAmounts.value : filteredAmounts.value.slice(0, 6))

const showSecondLine = computed(() => props.bonusTiers.length > 0)

function currencyDigits(): number {
  if (!props.currency) return 2
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: props.currency }).resolvedOptions().maximumFractionDigits ?? 2
  } catch {
    return 2
  }
}

function quoteFor(amt: number) {
  return quoteRechargeBonus(props.bonusTiers, amt, {
    multiplier: props.multiplier,
    mode: props.bonusMode,
    currencyDigits: currencyDigits(),
  })
}

// 价签文案：赠金「+20%」，折扣「20% OFF」
function badgeText(amt: number): string {
  const percent = formatRechargeBonusNumber(quoteFor(amt).percent)
  return props.bonusMode === 'discount' ? `${percent}% OFF` : `+${percent}%`
}

function secondLine(amt: number): string {
  const quote = quoteFor(amt)
  if (props.bonusMode === 'discount') {
    return t('payment.rechargeBonus.payShort', { amount: formatPaymentAmount(quote.payBase, props.currency) })
  }
  return t('payment.rechargeBonus.creditedShort', { amount: '$' + quote.credited.toFixed(2) })
}

const placeholderText = computed(() => {
  if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
  if (props.min > 0) return `≥ ${props.min}`
  if (props.max > 0) return `≤ ${props.max}`
  return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

function selectAmount(amt: number) {
  customText.value = String(amt)
  emit('update:modelValue', amt)
}

function handleInput(e: Event) {
  const input = e.target as HTMLInputElement
  const val = input.value
  if (!AMOUNT_PATTERN.test(val)) {
    input.value = customText.value
    return
  }
  customText.value = val
  if (val === '') {
    emit('update:modelValue', null)
    return
  }
  const num = parseFloat(val)
  if (!isNaN(num) && num > 0) {
    emit('update:modelValue', num)
  } else {
    emit('update:modelValue', null)
  }
}

watch(() => props.modelValue, (v) => {
  if (v !== null && String(v) !== customText.value) {
    customText.value = String(v)
  }
}, { immediate: true })
</script>
