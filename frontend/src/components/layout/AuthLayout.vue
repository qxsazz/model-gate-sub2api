<template>
  <div class="auth-luxury-shell relative flex min-h-screen items-center justify-center overflow-hidden p-4 sm:p-6">
    <div class="auth-luxury-atmosphere pointer-events-none absolute inset-0" aria-hidden="true"></div>

    <div class="auth-luxury-frame relative z-10 grid w-full max-w-5xl overflow-hidden lg:grid-cols-[0.82fr_1.18fr]">
      <aside class="auth-brand-panel hidden flex-col justify-between p-10 lg:flex">
        <div>
          <div v-if="settingsLoaded && siteLogo" class="auth-brand-logo mb-8">
            <img :src="siteLogo" :alt="brandName" class="h-full w-full object-contain" />
          </div>
          <p class="auth-font-mono auth-brand-kicker">MODEL-GATE / ACCESS</p>
          <h1 class="auth-font-display auth-brand-title mt-4">{{ brandName }}</h1>
          <p class="auth-brand-subtitle mt-4">AI API GATEWAY</p>
          <p class="auth-brand-authorization mt-3">MODEL-GATE 授权平台</p>
        </div>
        <div class="auth-brand-note">
          <span class="auth-brand-rule"></span>
          <p class="auth-font-mono">MODEL-GATE AUTHORIZED ACCESS</p>
        </div>
      </aside>

      <main class="auth-luxury-main">
        <div class="auth-luxury-mobile-brand mb-7 flex items-center gap-4 lg:hidden">
          <div v-if="settingsLoaded && siteLogo" class="auth-brand-logo auth-brand-logo-mobile">
            <img :src="siteLogo" :alt="brandName" class="h-full w-full object-contain" />
          </div>
          <div>
            <p class="auth-font-mono auth-brand-kicker">MODEL-GATE / ACCESS</p>
            <h1 class="auth-font-display auth-mobile-title">{{ brandName }}</h1>
          </div>
        </div>

        <div class="auth-luxury-card">
          <slot />
        </div>

        <div class="auth-luxury-footer mt-6 text-center text-sm">
          <slot name="footer" />
        </div>

        <div class="auth-luxury-copyright mt-8 text-center text-xs">
          &copy; {{ currentYear }} MODEL-GATE · Authorized by MODEL-GATE
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const brandName = 'MODEL-GATE'
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-luxury-shell {
  --auth-ink: var(--mg-ink-950, #121316);
  --auth-ink-soft: var(--mg-ink-700, #4a4740);
  --auth-gold: var(--mg-gold-700, #765f2c);
  --auth-gold-bright: var(--mg-gold-500, #c9b477);
  --auth-pearl: var(--mg-pearl-50, #fbfaf7);
  --auth-champagne: var(--mg-champagne-50, #f5f1e7);
  --auth-line: var(--mg-line-warm, #e7e1d4);
  --auth-danger: var(--mg-danger, #a75d56);
  color: var(--auth-ink-soft);
  background: var(--auth-pearl);
  font-family: 'Noto Sans SC', 'Avenir Next', 'Segoe UI', 'Microsoft YaHei', sans-serif;
}

.auth-luxury-atmosphere {
  background:
    linear-gradient(90deg, rgb(201 180 119 / 6%) 1px, transparent 1px),
    linear-gradient(rgb(201 180 119 / 6%) 1px, transparent 1px),
    linear-gradient(135deg, transparent 0 49.7%, rgb(201 180 119 / 10%) 50%, transparent 50.3%);
  background-size: 72px 72px, 72px 72px, 100% 100%;
  opacity: 0.7;
}

.auth-luxury-frame {
  border: 1px solid var(--auth-line);
  background: var(--auth-pearl);
  box-shadow: 0 24px 70px rgb(18 19 22 / 10%);
}

.auth-brand-panel {
  min-height: 38rem;
  background:
    linear-gradient(135deg, rgb(201 180 119 / 16%), transparent 38%),
    #101114;
  color: #f5f1e7;
}

.auth-brand-logo {
  width: 4.5rem;
  height: 4.5rem;
  overflow: hidden;
  border: 1px solid rgb(201 180 119 / 55%);
  border-radius: 0.75rem;
  background: #101114;
  box-shadow: 0 12px 30px rgb(0 0 0 / 24%);
}

.auth-brand-logo-mobile {
  width: 3.25rem;
  height: 3.25rem;
  flex: 0 0 auto;
}

.auth-font-display {
  font-family: 'Bodoni Moda', 'Bodoni MT', Didot, Georgia, serif;
}

.auth-font-mono {
  font-family: 'DM Mono', 'SFMono-Regular', Consolas, monospace;
}

.auth-brand-kicker {
  color: var(--auth-gold-bright);
  font-size: 0.625rem;
  font-weight: 500;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.auth-brand-title {
  color: #f5f1e7;
  font-size: clamp(2.5rem, 4vw, 4.25rem);
  font-weight: 600;
  line-height: 0.98;
}

.auth-brand-subtitle {
  max-width: 18rem;
  color: #f5f1e7;
  font-family: 'DM Mono', 'SFMono-Regular', Consolas, monospace;
  font-size: 0.75rem;
  letter-spacing: 0.1em;
}

.auth-brand-authorization {
  color: var(--auth-gold-bright);
  font-size: 0.8125rem;
  font-weight: 600;
}

.auth-brand-note {
  color: #aaa292;
  font-size: 0.625rem;
  letter-spacing: 0.08em;
}

.auth-brand-rule {
  display: block;
  width: 2.75rem;
  height: 1px;
  margin-bottom: 0.75rem;
  background: var(--auth-gold-bright);
}

.auth-luxury-main {
  min-width: 0;
  padding: clamp(1.25rem, 4vw, 3.5rem);
  background: rgb(255 255 255 / 74%);
}

.auth-luxury-card {
  max-width: 31rem;
  margin: 0 auto;
  padding: clamp(1.25rem, 4vw, 2.5rem);
  border: 1px solid var(--auth-line);
  border-radius: 0.625rem;
  background: rgb(255 255 255 / 86%);
  box-shadow: 0 14px 30px rgb(18 19 22 / 5%);
}

.auth-mobile-title {
  color: var(--auth-ink);
  font-size: 1.5rem;
  font-weight: 600;
}

.auth-luxury-shell :deep(h2) {
  color: var(--auth-ink) !important;
  font-family: 'Noto Serif SC', 'Songti SC', SimSun, Georgia, serif;
  font-size: 1.5rem;
  font-weight: 600;
  letter-spacing: 0;
}

.auth-luxury-shell :deep(p),
.auth-luxury-shell :deep(label),
.auth-luxury-shell :deep(span) {
  letter-spacing: 0;
}

.auth-luxury-shell :deep(.input-label) {
  color: var(--auth-ink-soft) !important;
  font-size: 0.8125rem;
  font-weight: 600;
}

.auth-luxury-shell :deep(.input-hint),
.auth-luxury-shell :deep(.text-gray-500),
.auth-luxury-shell :deep(.text-dark-400) {
  color: #746f65 !important;
}

.auth-luxury-shell :deep(.input) {
  border-color: var(--auth-line) !important;
  border-radius: 0.375rem;
  background: var(--auth-pearl) !important;
  color: var(--auth-ink) !important;
  font-size: 1rem;
  font-family: 'Noto Sans SC', 'Avenir Next', 'Segoe UI', 'Microsoft YaHei', sans-serif;
}

.auth-luxury-shell :deep(.input:focus) {
  border-color: var(--auth-gold) !important;
  box-shadow: 0 0 0 3px rgb(201 180 119 / 18%) !important;
}

.auth-luxury-shell :deep(.btn-primary) {
  border: 1px solid var(--auth-ink) !important;
  border-radius: 0.375rem;
  background: var(--auth-ink) !important;
  color: #f5f1e7 !important;
  box-shadow: none !important;
}

.auth-luxury-shell :deep(.btn-primary:hover:not(:disabled)) {
  border-color: #25262a !important;
  background: #25262a !important;
  color: #eadcae !important;
}

.auth-luxury-shell :deep(.btn-secondary) {
  border-color: var(--auth-line) !important;
  border-radius: 0.375rem;
  background: var(--auth-pearl) !important;
  color: var(--auth-ink-soft) !important;
  box-shadow: none !important;
}

.auth-luxury-shell :deep(.btn-secondary:hover:not(:disabled)) {
  border-color: var(--auth-gold-bright) !important;
  background: var(--auth-champagne) !important;
  color: var(--auth-gold) !important;
}

.auth-luxury-shell :deep(a) {
  color: var(--auth-gold) !important;
}

.auth-luxury-shell :deep(a:hover) {
  color: #5f4a22 !important;
}

.auth-luxury-shell :deep(.bg-gray-200),
.auth-luxury-shell :deep(.dark\:bg-dark-700) {
  background: var(--auth-line) !important;
}

.auth-luxury-shell :deep(.border-green-500),
.auth-luxury-shell :deep(.text-green-500),
.auth-luxury-shell :deep(.text-green-600),
.auth-luxury-shell :deep(.text-green-700) {
  border-color: var(--auth-gold) !important;
  color: var(--auth-gold) !important;
}

.auth-luxury-shell :deep(.bg-green-50) {
  background: var(--auth-champagne) !important;
}

.auth-luxury-shell :deep(.border-red-500),
.auth-luxury-shell :deep(.text-red-500) {
  border-color: var(--auth-danger) !important;
  color: var(--auth-danger) !important;
}

.auth-luxury-footer :deep(p) {
  color: var(--auth-ink-soft) !important;
}

.auth-luxury-copyright {
  color: #746f65;
  font-family: 'DM Mono', 'SFMono-Regular', Consolas, monospace;
}

.dark .auth-luxury-shell {
  --auth-ink: #f5f1e7;
  --auth-ink-soft: #d6cebd;
  --auth-pearl: #111214;
  --auth-champagne: #23221f;
  --auth-line: #38352f;
}

.dark .auth-luxury-main,
.dark .auth-luxury-card {
  background: #18191d;
}

@media (max-width: 639px) {
  .auth-luxury-shell {
    align-items: flex-start;
    padding: 1rem;
  }

  .auth-luxury-main {
    padding: 1rem;
  }

  .auth-luxury-card {
    padding: 1.25rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-luxury-shell *,
  .auth-luxury-shell *::before,
  .auth-luxury-shell *::after {
    transition-duration: 0.01ms !important;
  }
}
</style>
