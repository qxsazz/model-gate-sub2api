<template>
  <div ref="root" class="relative" @keydown.esc="open = false">
    <button v-if="authStore.isAdmin" type="button" class="rounded-lg bg-gray-100 px-2 py-1 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300" :aria-expanded="open" :aria-label="t('version.currentVersion')" @click="open = !open">
      {{ appStore.currentVersion ? `v${appStore.currentVersion}` : '—' }}
    </button>
    <span v-else class="text-xs text-gray-500">{{ appStore.siteVersion ? `v${appStore.siteVersion}` : '' }}</span>
    <div v-if="open && authStore.isAdmin" class="whitespace-normal text-gray-900 dark:text-dark-100 absolute top-full left-0 z-50 mt-2 w-64 max-w-[calc(100vw-2rem)] rounded-xl border border-gray-200 bg-white p-4 text-sm shadow-lg dark:border-dark-700 dark:bg-dark-800">
      <div class="flex items-center justify-between gap-2">
        <span class="font-medium">{{ t('version.currentVersion') }}</span>
        <button type="button" class="rounded-lg px-2 py-1 text-xs hover:bg-gray-100 dark:hover:bg-dark-700" :disabled="appStore.versionLoading" @click="appStore.fetchVersion(true)">{{ t('version.refresh') }}</button>
      </div>
      <p class="mt-2 font-mono">{{ appStore.currentVersion || '—' }}</p>
      <p class="mt-3 text-xs leading-5 text-gray-500 dark:text-dark-400">{{ t('version.managedDescription') }}</p>
      <a :href="MG_BRAND.deploymentUrl" target="_blank" rel="noopener noreferrer" class="mt-4 block rounded-lg bg-gray-900 px-3 py-2 text-center text-white dark:bg-dark-600">{{ t('version.deploymentEntry') }}</a>
      <a :href="MG_BRAND.repositoryUrl" target="_blank" rel="noopener noreferrer" class="mt-3 block text-center text-xs text-gray-500">{{ t('version.repositoryEntry') }}</a>
    </div>
  </div>
</template>
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { MG_BRAND } from '@/brand/config'
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const open = ref(false)
const root = ref<HTMLElement | null>(null)
function closeOutside(event: MouseEvent) {
  if (event.target instanceof Node && !root.value?.contains(event.target)) open.value = false
}
onMounted(() => {
  if (authStore.isAdmin) void appStore.fetchVersion(false)
  document.addEventListener('click', closeOutside)
})
onBeforeUnmount(() => document.removeEventListener('click', closeOutside))
</script>
