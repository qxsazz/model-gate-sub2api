<template>
  <div class="contact-page">
    <header class="contact-header">
      <nav class="contact-nav" :aria-label="t('contact.title')">
        <router-link to="/home" class="contact-brand">
          <img :src="siteLogo || '/model-gate-mg-luxury.svg'" alt="" width="44" height="44" />
          <span>{{ siteName }}</span>
        </router-link>
        <div class="contact-nav-actions">
          <router-link to="/home" class="home-link">{{ t('contact.backHome') }}</router-link>
          <button type="button" class="theme-button" :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')" @click="toggleTheme">
            <Icon :name="isDark ? 'sun' : 'moon'" size="md" />
          </button>
          <router-link :to="consolePath" class="console-link">{{ t('contact.console') }}</router-link>
        </div>
      </nav>
    </header>

    <main class="contact-main">
      <div class="contact-intro">
        <p class="contact-eyebrow">{{ t('contact.eyebrow') }}</p>
        <h1>{{ t('contact.title') }}</h1>
        <p class="contact-description">{{ t('contact.intro') }}</p>
      </div>
      <div v-if="channels.length" class="contact-grid">
        <article v-for="channel in channels" :key="channel.id" class="contact-card" data-testid="contact-card">
          <div class="contact-card-top">
            <div class="contact-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <template v-if="channel.kind === 'qq-group'">
                  <circle cx="9" cy="8" r="3" /><path d="M3 20v-2a6 6 0 0 1 12 0v2M16 5a3 3 0 0 1 0 6M17 14a5 5 0 0 1 4 4v2" />
                </template>
                <template v-else>
                  <path d="M21 11a8 8 0 0 1-8 8H7l-5 3 2-6a8 8 0 1 1 17-5Z" /><path d="M8 11h.01M12 11h.01M16 11h.01" />
                </template>
              </svg>
            </div>
            <span class="channel-type">{{ channel.kind === 'qq-group' ? 'QQ' : channel.kind === 'wechat' ? 'WECHAT' : 'QQ' }}</span>
          </div>
          <h2>{{ t(channel.labelKey) }}</h2>
          <p class="contact-purpose">{{ t(channel.kind === 'qq-group' ? 'contact.groupDescription' : channel.id === 'wechat-owner' ? 'contact.ownerDescription' : 'contact.supportDescription') }}</p>
          <p class="contact-value" :data-testid="`contact-value-${channel.id}`">{{ channel.value }}</p>
          <p class="contact-hint">{{ t(channel.kind === 'qq-group' ? 'contact.groupHint' : channel.kind === 'wechat' ? 'contact.wechatHint' : 'contact.qqHint') }}</p>
          <div class="contact-card-footer">
            <button type="button" :data-testid="`copy-${channel.id}`" :aria-label="`${t(channel.kind === 'wechat' ? 'contact.copyWechat' : 'contact.copy')} · ${t(channel.labelKey)}`" @click="copyChannel(channel)">
              {{ t(channel.kind === 'wechat' ? 'contact.copyWechat' : 'contact.copy') }}
              <Icon name="copy" size="sm" />
            </button>
          </div>
        </article>
      </div>
      <p v-else class="contact-empty">{{ t('contact.empty') }}</p>
      <p class="copy-status" role="status" aria-live="polite">{{ feedback }}</p>
      <p class="contact-note">{{ t('contact.note') }}</p>
    </main>
    <footer class="contact-footer">© {{ new Date().getFullYear() }} {{ siteName }}</footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import { useClipboard } from '@/composables/useClipboard'
import { contactChannels, type ContactChannel } from '@/content/contactChannels'
import { sanitizeUrl } from '@/utils/url'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()
const channels = computed(() => contactChannels.filter(channel => channel.value.trim()))
const siteName = computed(() => appStore.siteName === 'Sub2API' ? 'MODEL-GATE' : appStore.siteName || 'MODEL-GATE')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const consolePath = computed(() => !authStore.isAuthenticated ? '/login' : authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
const isDark = ref(document.documentElement.classList.contains('dark'))
const feedback = ref('')
let copyVersion = 0

async function copyChannel(channel: ContactChannel) {
  const version = ++copyVersion
  const success = await copyToClipboard(channel.value)
  if (version === copyVersion) feedback.value = t(success ? 'contact.copied' : 'contact.copyFailed')
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) void appStore.fetchPublicSettings()
})
</script>

<style scoped>
.contact-page {
  --contact-bg: #fbfaf7;
  --contact-surface: #fff;
  --contact-ink: #121316;
  --contact-muted: #746f65;
  --contact-line: #e7e1d4;
  --contact-gold: #9c8344;
  --contact-soft: #f5f1e7;
  min-height: 100svh;
  display: flex;
  flex-direction: column;
  background: var(--contact-bg);
  color: var(--contact-ink);
  font-family: 'Noto Sans SC', 'Microsoft YaHei', sans-serif;
}
.dark .contact-page {
  --contact-bg: #111214;
  --contact-surface: #18191d;
  --contact-ink: #f5f1e7;
  --contact-muted: #aaa292;
  --contact-line: #38352f;
  --contact-gold: #d7c487;
  --contact-soft: #23221f;
}
.contact-header { border-bottom: 1px solid var(--contact-line); background: var(--contact-surface); }
.contact-nav { max-width: 1200px; margin: auto; padding: 20px 40px; display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.contact-brand { display: flex; align-items: center; gap: 14px; min-width: 0; flex: 1; }
.contact-brand img { border-radius: 8px; object-fit: contain; flex-shrink: 0; }
.contact-brand span { font: 600 24px 'Bodoni Moda', Georgia, serif; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.contact-nav-actions { display: flex; align-items: center; gap: 20px; flex-shrink: 0; }
.home-link { font-size: 13px; color: var(--contact-muted); }
.theme-button { padding: 10px; border: 1px solid var(--contact-line); border-radius: 6px; }
.console-link { padding: 11px 18px; background: #121316; color: #f5f1e7; border: 1px solid #38352f; border-radius: 6px; font-size: 13px; }
.contact-main { width: min(1120px, 100%); margin: 0 auto; padding: 70px 40px 40px; flex: 1; }
.contact-intro { margin-bottom: 44px; }
.contact-eyebrow { color: var(--contact-gold); font: 500 12px 'DM Mono', Consolas, monospace; margin-bottom: 18px; }
h1 { font: 600 clamp(36px, 5vw, 60px)/1.2 'Noto Serif SC', 'Songti SC', Georgia, serif; margin: 0 0 24px; }
.contact-description { color: var(--contact-muted); font-size: 16px; line-height: 1.9; }
.contact-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
.contact-card { padding: 28px 30px 0; border: 1px solid var(--contact-line); border-radius: 12px; background: var(--contact-surface); box-shadow: 0 2px 6px rgb(18 19 22 / 3%); }
.contact-card-top { display: flex; align-items: center; justify-content: space-between; margin-bottom: 22px; }
.contact-icon { color: var(--contact-gold); background: var(--contact-soft); padding: 12px; border: 1px solid var(--contact-line); border-radius: 8px; }
.contact-icon svg { display: block; width: 26px; height: 26px; }
.channel-type { color: var(--contact-muted); font: 11px 'DM Mono', Consolas, monospace; }
h2 { font: 600 22px 'Noto Serif SC', 'Songti SC', Georgia, serif; margin-bottom: 12px; }
.contact-purpose, .contact-hint { color: var(--contact-muted); font-size: 13px; line-height: 1.8; }
.contact-value { margin: 22px 0 8px; font: 500 22px/1.5 'DM Mono', Consolas, monospace; overflow-wrap: anywhere; user-select: text; }
.contact-card-footer { border-top: 1px solid var(--contact-line); margin-top: 22px; }
.contact-card-footer button { display: flex; align-items: center; justify-content: space-between; width: 100%; padding: 18px 0; color: var(--contact-gold); font-size: 13px; font-weight: 600; text-align: left; }
.copy-status { min-height: 24px; margin: 20px 0 8px; color: var(--contact-gold); font-size: 13px; }
.contact-note, .contact-empty { color: var(--contact-muted); font-size: 12px; line-height: 1.9; }
.contact-footer { border-top: 1px solid var(--contact-line); padding: 24px; text-align: center; color: var(--contact-muted); font: 11px 'DM Mono', Consolas, monospace; }
button:focus-visible, a:focus-visible { outline: 2px solid var(--contact-gold); outline-offset: 4px; }
button:hover, .home-link:hover { color: var(--contact-gold); }
@media (max-width: 640px) {
  .contact-nav { padding: 16px 20px; gap: 12px; }
  .contact-brand span { font-size: 19px; }
  .contact-brand img { width: 36px; height: 36px; }
  .contact-nav-actions { gap: 10px; }
  .home-link { display: none; }
  .contact-main { padding: 42px 20px 32px; }
  .contact-intro { margin-bottom: 30px; }
  .contact-grid { grid-template-columns: 1fr; gap: 18px; }
  .contact-card { padding: 24px 24px 0; }
  .contact-description { font-size: 14px; }
}
</style>
