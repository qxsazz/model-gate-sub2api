<template>
  <button
    v-if="status && (status.checked_in || !accessBlocked)"
    type="button"
    data-testid="header-checkin"
    class="header-checkin"
    :class="{ 'is-complete': status.checked_in }"
    :disabled="busy || status.checked_in"
    :aria-busy="busy"
    :aria-label="label"
    :title="label"
    @click="checkIn"
  >
    <Icon v-if="status.checked_in" name="check" size="sm" aria-hidden="true" />
    <template v-else>
      <Icon name="calendar" size="sm" class="checkin-calendar" aria-hidden="true" />
      <span class="checkin-dot" aria-hidden="true" />
    </template>
    <span class="checkin-label">{{ label }}</span>
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useEventBus } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { changeAchievement, getAchievementCheckinStatus, type AchievementCheckinStatus } from '@/api/achievements'
import { achievementCheckinEvent } from '@/components/achievements/checkinEvents'
import { achievementRechargeAccess, isAchievementAccessRestricted } from '@/components/achievements/rechargeAccess'
import { useCheckinDayRefresh } from '@/components/achievements/useCheckinDayRefresh'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ userId: number }>()
const app = useAppStore()
const auth = useAuthStore()
const { t } = useI18n()
const checkins = useEventBus(achievementCheckinEvent)
const status = ref<AchievementCheckinStatus | null>(null)
const busy = ref(false)
const accessBlocked = computed(() => status.value?.checkin_exempt !== true && achievementRechargeAccess(status.value, true).blocked)
const label = computed(() => {
  if (status.value?.checked_in) return t('nav.checkin.done')
  return t(busy.value ? 'nav.checkin.pending' : 'nav.checkin.action')
})

let active = true
let statusRequest = 0
let loadingStatus = false
let refreshQueued = false
let attempt: { date: string; key: string } | undefined

async function refreshStatus(queueIfBusy = false) {
  if (!active || document.hidden) return
  if (busy.value || loadingStatus) {
    if (queueIfBusy) refreshQueued = true
    return
  }
  loadingStatus = true
  refreshQueued = false
  const request = ++statusRequest
  try {
    const next = await getAchievementCheckinStatus()
    if (!active || request !== statusRequest) return
    status.value = next
  } catch {
    // Keep the last confirmed state on a transient refresh failure.
  } finally {
    loadingStatus = false
    if (refreshQueued) void refreshStatus()
  }
}

async function checkIn() {
  const current = status.value
  if (!current || current.checked_in || busy.value || accessBlocked.value) return

  const userId = props.userId
  busy.value = true
  statusRequest++
  let refreshNeeded = false
  try {
    if (attempt?.date !== current.date) attempt = { date: current.date, key: crypto.randomUUID() }
    const receipt = await changeAchievement('checkin', { date: current.date, request_key: attempt.key })
    if (auth.user?.id !== userId) return
    if (active) {
      if (status.value?.date === receipt.day) status.value = { ...status.value, checked_in: true }
      else refreshNeeded = true
    }
    checkins.emit({ userId, date: receipt.day })
    app.showSuccess(t(receipt.replayed ? 'nav.checkin.alreadyDone' : 'nav.checkin.success'))
    void auth.refreshUser().catch(() => {
      // The check-in is already recorded even if the balance refresh fails.
    })
  } catch (error) {
    if (!active || auth.user?.id !== userId) return
    app.showError(extractApiErrorMessage(error, t('nav.checkin.failed')))
    const code = extractApiErrorCode(error)
    if (isAchievementAccessRestricted(error) || code === 'ACHIEVEMENT_ACCOUNT_UNAVAILABLE' || code === 'ACHIEVEMENT_DATE_CHANGED') {
      status.value = null
      refreshNeeded = true
    }
  } finally {
    busy.value = false
    if (refreshNeeded || refreshQueued || !status.value) void refreshStatus(true)
  }
}

checkins.on((event) => {
  if (!event || event.userId !== props.userId || busy.value) return
  if (status.value?.date === event.date) {
    statusRequest++
    status.value = { ...status.value, checked_in: true }
  } else {
    void refreshStatus(true)
  }
})

useCheckinDayRefresh((dayChanged) => {
  if (dayChanged) {
    statusRequest++
    status.value = null
  }
  void refreshStatus(dayChanged)
})

// Balance payment completion and the existing periodic auth refresh both replace the user snapshot.
watch(() => auth.user, (user) => {
  if (user?.id !== props.userId || !accessBlocked.value || status.value?.checked_in) return
  void refreshStatus(true)
})

onMounted(() => { void refreshStatus() })
onBeforeUnmount(() => {
  active = false
})
</script>

<style scoped>
.header-checkin {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  gap: 7px;
  min-width: 88px;
  height: 36px;
  padding-inline: 9px;
  border: 1px solid #e8d8af;
  border-radius: 8px;
  background: #f7f0df;
  color: #765517;
  font-size: 0.875rem;
  font-weight: 650;
  white-space: nowrap;
  transition: background-color 150ms, border-color 150ms;
}
.header-checkin:hover:not(:disabled) { background: #f1e6c9; border-color: #d8c18e; }
.header-checkin:focus-visible { outline: 2px solid #9c7326; outline-offset: 2px; }
.header-checkin:disabled { cursor: default; }
.header-checkin[aria-busy="true"] .checkin-dot { opacity: 0.55; }
.header-checkin.is-complete { background: #f4f1e9; border-color: #e6dfd1; color: #6c624e; }
.checkin-calendar { display: none; }
.checkin-dot {
  width: 6px;
  height: 6px;
  flex: none;
  border-radius: 50%;
  background: #d39a2c;
  box-shadow: 0 0 0 3px #e9c87947;
}
.dark .header-checkin { color: #ead08c; background: #302719; border-color: #594729; }
.dark .header-checkin:hover:not(:disabled) { background: #3e311d; border-color: #735b30; }
.dark .header-checkin.is-complete { color: #c1b49a; background: #28251f; border-color: #443d31; }

@media (max-width: 639px) {
  .header-checkin { min-width: 78px; gap: 5px; padding-inline: 7px; font-size: 0.8125rem; }
}
@media (max-width: 380px) {
  .header-checkin { position: relative; min-width: 36px; padding-inline: 0; }
  .checkin-label { display: none; }
  .checkin-calendar { display: block; }
  .checkin-dot { position: absolute; top: 5px; right: 5px; }
}
@media (prefers-reduced-motion: reduce) {
  .header-checkin { transition: none; }
}
</style>
