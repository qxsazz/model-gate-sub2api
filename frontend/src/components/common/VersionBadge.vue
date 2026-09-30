<template>
  <div ref="root" class="relative" @keydown.esc="open = false">
    <button v-if="authStore.isAdmin" type="button" class="rounded-lg bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300" :aria-expanded="open" :aria-label="t('version.currentVersion')" @click="togglePanel">
      {{ appStore.currentVersion ? `v${appStore.currentVersion}` : '—' }}
      <span v-if="appStore.upstreamInfo?.has_update" data-testid="upstream-update-dot" class="ml-1 inline-block h-1.5 w-1.5 rounded-full bg-amber-500" :title="t('version.upstreamAvailable')"></span>
    </button>
    <span v-else class="text-xs text-gray-500">{{ appStore.siteVersion ? `v${appStore.siteVersion}` : '' }}</span>
    <div v-if="open && authStore.isAdmin" class="whitespace-normal text-gray-900 dark:text-dark-100 absolute top-full left-0 z-50 mt-2 w-72 max-w-[calc(100vw-2rem)] rounded-xl border border-gray-200 bg-white p-4 text-sm shadow-lg dark:border-dark-700 dark:bg-dark-800">
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium">{{ t('version.currentVersion') }}</span>
        <button type="button" class="rounded-lg px-2 py-1 text-xs hover:bg-gray-100 dark:hover:bg-dark-700" :disabled="appStore.versionLoading" @click="appStore.fetchVersion(true)">{{ t('version.refresh') }}</button>
      </div>
      <p class="mt-2 font-mono">{{ appStore.currentVersion || '—' }}</p>
      <div class="mt-3 border-t border-gray-200 pt-3 text-xs dark:border-dark-700">
        <div class="flex justify-between gap-2"><span>{{ t('version.upstreamBaseline') }}</span><span class="font-mono">{{ appStore.upstreamInfo?.baseline_version || '—' }}</span></div>
        <div class="mt-2 flex justify-between gap-2"><span>{{ t('version.upstreamLatest') }}</span><span class="font-mono">{{ appStore.upstreamInfo?.latest_version || '—' }}</span></div>
        <p class="mt-2 leading-5" :class="appStore.upstreamInfo?.has_update ? 'text-amber-700 dark:text-amber-400' : 'text-gray-500 dark:text-dark-400'">{{ upstreamStatus }}</p>
        <p v-if="appStore.upstreamInfo?.checked_at" class="mt-1 break-words text-gray-500 dark:text-dark-400">{{ t('version.upstreamCheckedAt') }} {{ checkedAt }}</p>
        <a v-if="releaseUrl" :href="releaseUrl" target="_blank" rel="noopener noreferrer" class="mt-2 inline-block underline underline-offset-4">{{ t('version.upstreamReleaseNotes') }}</a>
      </div>
      <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('version.managedDescription') }}</p>
      <a :href="MG_BRAND.deploymentUrl" target="_blank" rel="noopener noreferrer" class="mt-4 block rounded-lg bg-gray-900 px-3 py-2 text-center text-white dark:bg-dark-600">{{ t('version.deploymentEntry') }}</a>
      <a :href="MG_BRAND.repositoryUrl" target="_blank" rel="noopener noreferrer" class="mt-3 block text-center text-xs text-gray-500">{{ t('version.repositoryEntry') }}</a>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { MG_BRAND } from '@/brand/config'
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const open = ref(false)
const root = ref<HTMLElement | null>(null)
const upstreamStatus = computed(() => {
  const upstream = appStore.upstreamInfo
  if (upstream?.status === 'checking') return t('version.upstreamChecking')
  if (upstream?.status === 'stale') return t('version.upstreamStale')
  if (upstream?.status !== 'ok') return t('version.upstreamUnavailable')
  return t(upstream.has_update ? 'version.upstreamAvailable' : 'version.upstreamCurrent')
})
const checkedAt = computed(() => {
  const date = new Date(appStore.upstreamInfo?.checked_at || '')
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString()
})
const releaseUrl = computed(() => {
  const url = appStore.upstreamInfo?.release_url || ''
  return url.startsWith('https://github.com/Wei-Shaw/sub2api/releases/tag/') ? url : ''
})
function togglePanel() {
  open.value = !open.value
  if (open.value) void appStore.fetchVersion(false)
}
function closeOutside(event: MouseEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) open.value = false
}
onMounted(() => {
  if (authStore.isAdmin) void appStore.fetchVersion(false)
  document.addEventListener('click', closeOutside)
})
onBeforeUnmount(() => document.removeEventListener('click', closeOutside))
</script>
