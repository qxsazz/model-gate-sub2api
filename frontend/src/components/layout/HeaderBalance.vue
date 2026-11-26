<template>
  <div ref="container" class="header-balance" :class="{ 'header-balance-inline': inline }" @keydown.esc="handleEscape">
    <button
      ref="trigger"
      type="button"
      class="user-console-balance header-balance-trigger"
      :aria-label="t('common.balanceBreakdown.open', { amount: triggerAmount })"
      :aria-expanded="open"
      :aria-controls="panelId"
      @click="toggle"
    >
      <Icon name="creditCard" size="sm" aria-hidden="true" />
      <span>{{ triggerAmount }}</span>
      <Icon name="chevronDown" size="xs" aria-hidden="true" class="header-balance-chevron" />
    </button>

    <section v-show="open" :id="panelId" class="header-balance-panel" :aria-labelledby="headingId" :aria-busy="loading">
      <header class="header-balance-heading">
        <h2 :id="headingId">{{ t('common.availableBalance') }}</h2>
        <strong>{{ triggerAmount }}</strong>
        <button type="button" class="header-balance-close" :aria-label="t('common.balanceBreakdown.close')" @click="closeAndFocus">
          <Icon name="x" size="sm" aria-hidden="true" />
        </button>
      </header>

      <p v-if="loading" class="header-balance-feedback" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="failed" class="header-balance-feedback" role="alert">
        {{ t('common.balanceBreakdown.error') }}
        <button type="button" class="header-balance-retry" @click="retry">{{ t('common.tryAgain') }}</button>
      </div>
      <dl v-else-if="details" class="header-balance-lines">
        <div class="header-balance-line header-balance-credit">
          <dt>
            <span class="header-balance-icon"><Icon name="calendar" size="xs" aria-hidden="true" /></span>
            <span>{{ t('common.balanceBreakdown.monthlyCheckin', { month: monthLabel }) }}</span>
            <span v-if="details.checkin_credit.available > 0" class="header-balance-priority">{{ t('common.balanceBreakdown.priority') }}</span>
          </dt>
          <dd class="header-balance-value">{{ formatAmount(details.checkin_credit.available) }}</dd>
          <dd class="header-balance-expiry">{{ t('common.balanceBreakdown.expiry', { date: expiryLabel(details.checkin_credit.expires_at) }) }}</dd>
        </div>
        <div class="header-balance-line">
          <dt>
            <span class="header-balance-icon"><Icon name="creditCard" size="xs" aria-hidden="true" /></span>
            <span>{{ t('common.balanceBreakdown.other') }}</span>
          </dt>
          <dd class="header-balance-value">
            {{ formatAmount(details.other_available + (details.legacy_checkin_credit?.available ?? 0)) }}
          </dd>
          <dd v-if="(details.legacy_checkin_credit?.available ?? 0) > 0" class="header-balance-note">
            {{ t('common.balanceBreakdown.legacyNote') }}
          </dd>
        </div>
        <div v-if="details.frozen_balance > 0" class="header-balance-line">
          <dt>
            <span class="header-balance-icon"><Icon name="lock" size="xs" aria-hidden="true" /></span>
            <span>{{ t('common.frozenBalance') }}</span>
          </dt>
          <dd class="header-balance-value">{{ formatAmount(details.frozen_balance) }}</dd>
        </div>
      </dl>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, useId, watch } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { getWalletBalance, type WalletBalance } from '@/api/wallet'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime, formatMoneyFixed } from '@/utils/format'

const props = defineProps<{ available: number; frozen: number; inline?: boolean }>()
const { t, locale } = useI18n()
const panelId = useId()
const headingId = useId()
const container = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const open = ref(false)
const loading = ref(false)
const failed = ref(false)
const details = ref<WalletBalance | null>(null)
const triggerAmount = computed(() => formatAmount(details.value?.available_balance ?? props.available))
const monthLabel = computed(() => {
  const month = details.value?.checkin_credit.month
  if (!month) return ''
  if (locale.value.startsWith('zh')) return Number(month.slice(5))
  return formatDateTime(`${month}-01T00:00:00+08:00`, { month: 'long', timeZone: 'Asia/Shanghai' }, locale.value)
})

const expiryFormat: Intl.DateTimeFormatOptions = {
  timeZone: 'Asia/Shanghai', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
}
let request: AbortController | undefined
let expiryTimer: number | undefined

function formatAmount(amount: number) {
  return formatMoneyFixed(amount, '$')
}

function expiryLabel(date: string) {
  return formatDateTime(date, expiryFormat, locale.value)
}

async function loadBalance() {
  request?.abort()
  window.clearTimeout(expiryTimer)
  const next = new AbortController()
  request = next
  loading.value = true
  failed.value = false
  try {
    const balance = await getWalletBalance(next.signal)
    if (next.signal.aborted) return
    details.value = balance
    const legacyExpiry = balance.legacy_checkin_credit?.next_expiry
    const nextExpiry = Math.min(Date.parse(balance.checkin_credit.expires_at), legacyExpiry ? Date.parse(legacyExpiry) : Infinity)
    const remaining = nextExpiry - Date.now()
    if (remaining > 0) {
      expiryTimer = window.setTimeout(loadBalance, Math.min(remaining + 250, 2_147_483_647))
    }
  } catch {
    if (next.signal.aborted) return
    details.value = null
    failed.value = true
  } finally {
    if (!next.signal.aborted) loading.value = false
  }
}

function close() {
  open.value = false
  request?.abort()
  window.clearTimeout(expiryTimer)
}

function toggle() {
  if (open.value) {
    close()
    return
  }
  open.value = true
  void loadBalance()
}

function closeAndFocus() {
  close()
  trigger.value?.focus()
}

function retry() {
  trigger.value?.focus()
  void loadBalance()
}

function handleEscape(event: KeyboardEvent) {
  if (!open.value) return
  event.stopPropagation()
  closeAndFocus()
}

onClickOutside(container, close)
onBeforeUnmount(close)
watch(() => [props.available, props.frozen], () => {
  details.value = null
  if (open.value) void loadBalance()
})
</script>

<style scoped>
.header-balance {
  position: relative;
  --balance-sheet: #fffdf8;
  --balance-edge: #e0d6c3;
  --balance-ink: #292721;
  --balance-muted: #6b604d;
  --balance-credit: #f8f2e4;
  --balance-credit-border: #ebdfc4;
  --balance-badge: #efe1be;
  --balance-badge-ink: #6f501e;
  --balance-icon: #f2eee6;
}

.dark .header-balance {
  --balance-sheet: #232120;
  --balance-edge: #504638;
  --balance-ink: #f0e9dd;
  --balance-muted: #c4b6a0;
  --balance-credit: #352b1c;
  --balance-credit-border: #534327;
  --balance-badge: #514326;
  --balance-badge-ink: #f0d79d;
  --balance-icon: #332d23;
}

.header-balance-trigger {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  padding: 4px 10px;
  border: 1px solid var(--balance-edge);
  background: var(--balance-credit);
  color: var(--balance-badge-ink);
  font-size: 14px;
  font-weight: 650;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

.header-balance-trigger:hover, .header-balance-trigger[aria-expanded='true'] { border-color: var(--mg-gold-700, #9c8344) !important; }
.header-balance-trigger[aria-expanded='true'] .header-balance-chevron { transform: rotate(180deg); }
.header-balance-trigger:focus-visible, .header-balance-close:focus-visible, .header-balance-retry:focus-visible { outline: 2px solid var(--mg-gold-700, #9c8344); outline-offset: 2px; }

.header-balance-panel {
  position: absolute;
  z-index: 55;
  inset-block-start: calc(100% + 9px);
  inset-inline-end: 0;
  width: min(310px, calc(100vw - 16px));
  padding: 10px;
  border: 1px solid var(--balance-edge);
  border-radius: 12px;
  background: var(--balance-sheet);
  color: var(--balance-ink);
  box-shadow: 0 10px 24px rgb(56 43 26 / 12%), 0 2px 5px rgb(56 43 26 / 6%);
}

.header-balance-panel::before {
  content: '';
  position: absolute;
  top: -5px;
  right: 25px;
  width: 8px;
  height: 8px;
  transform: rotate(45deg);
  border-top: 1px solid var(--balance-edge);
  border-left: 1px solid var(--balance-edge);
  background: var(--balance-sheet);
}

.header-balance-heading { display: grid; grid-template-columns: minmax(0, 1fr) fit-content(61%) auto; align-items: center; gap: 7px; padding: 0 0 7px 4px; border-bottom: 1px solid var(--balance-edge); }
.header-balance-heading h2 { margin: 0; color: var(--balance-muted); font-size: 13px; font-weight: 600; }
.header-balance-heading strong { font-size: 20px; font-weight: 675; font-variant-numeric: tabular-nums; letter-spacing: -.03em; overflow-wrap: anywhere; text-align: end; }
.header-balance-close { display: grid; width: 26px; height: 26px; place-items: center; color: var(--balance-muted); }
.header-balance-close:hover { background: var(--balance-icon); }
.header-balance-lines { display: grid; gap: 2px; margin: 8px 0 0; }
.header-balance-line { display: grid; grid-template-columns: minmax(0, 1fr) fit-content(48%); align-items: start; gap: 3px 6px; padding: 7px 8px; border: 1px solid transparent; border-radius: 8px; }
.header-balance-line dt { display: flex; flex-wrap: wrap; align-items: center; gap: 4px; font-size: 13px; font-weight: 650; }
.header-balance-line dd { margin: 0; }
.header-balance-credit { background: var(--balance-credit); border-color: var(--balance-credit-border); }
.header-balance-credit .header-balance-value { color: var(--balance-badge-ink); }
.header-balance-icon { display: grid; flex: none; width: 21px; height: 21px; place-items: center; border-radius: 5px; background: var(--balance-icon); color: var(--balance-muted); }
.header-balance-credit .header-balance-icon, .header-balance-priority { background: var(--balance-badge); color: var(--balance-badge-ink); }
.header-balance-priority { padding: 1px 4px; border-radius: 4px; font-size: 10px; line-height: 1.4; white-space: nowrap; }
.header-balance-value { align-self: center; font-size: 13px; font-weight: 700; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; text-align: end; }
.header-balance-expiry, .header-balance-note { grid-column: 1 / -1; padding-inline-start: 25px; color: var(--balance-muted); font-size: 11px; line-height: 1.45; }
.header-balance-feedback { margin-top: 8px; padding: 5px 4px; color: var(--balance-muted); font-size: 12px; }
.header-balance-retry { color: var(--balance-badge-ink); text-decoration: underline; }

.header-balance-inline .header-balance-trigger { justify-content: space-between; width: 100%; }
.header-balance-inline .header-balance-panel { position: static; width: 100%; margin-top: 7px; padding: 7px; box-shadow: none; }
.header-balance-inline .header-balance-panel::before { display: none; }
.header-balance-inline .header-balance-heading { column-gap: 4px; padding-inline-start: 1px; }
.header-balance-inline .header-balance-heading strong { font-size: 16px; }
.header-balance-inline .header-balance-line { padding-inline: 4px; }
</style>
