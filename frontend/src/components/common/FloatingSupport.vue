<template>
  <Teleport to="body">
    <div v-if="visible" ref="widget" class="floating-support" :class="{ 'support-payment-route': route.path.startsWith('/purchase') || route.path.startsWith('/payment') }">
      <section v-if="open" id="mg-support-panel" ref="panel" class="support-panel" role="dialog" aria-labelledby="mg-support-title" data-testid="support-panel">
        <header class="support-heading">
          <div><p class="support-eyebrow">SUPPORT · MG</p><h2 id="mg-support-title">{{ t('contact.floatingTitle') }}</h2></div>
          <button type="button" class="support-close" :aria-label="t('contact.close')" @click="close(true)"><Icon name="x" size="md" /></button>
        </header>
        <p class="support-description">{{ t('contact.floatingIntro') }}</p>
        <div class="support-channels">
          <div v-for="channel in channels" :key="channel.id" class="support-channel">
            <div class="support-channel-details"><span class="support-label">{{ t(channel.labelKey) }}</span><span class="support-value">{{ channel.value }}</span></div>
            <button type="button" class="support-copy" :data-testid="`support-copy-${channel.id}`" :aria-label="`${t('contact.copy')} · ${t(channel.labelKey)}`" @click="copyChannel(channel)"><Icon name="copy" size="sm" /><span>{{ t('contact.floatingCopy') }}</span></button>
          </div>
        </div>
        <p class="support-feedback" role="status" aria-live="polite">{{ feedback }}</p>
        <router-link to="/contact" class="support-complete" @click="close(false)">{{ t('contact.fullPage') }}<Icon name="arrowRight" size="sm" /></router-link>
      </section>
      <button ref="trigger" type="button" class="support-trigger" data-testid="support-trigger" :aria-expanded="open" aria-controls="mg-support-panel" @click="toggle">
        <span class="support-monogram" aria-hidden="true">MG</span>
        <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M4 14v-3a8 8 0 0 1 16 0v3M20 17v2a2 2 0 0 1-2 2h-3"/><rect x="3" y="11" width="4" height="7" rx="2"/><rect x="17" y="11" width="4" height="7" rx="2"/></svg>
        <span>{{ t('contact.floatingTrigger') }}</span>
      </button>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { contactChannels, type ContactChannel } from '@/content/contactChannels'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'

const route = useRoute()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const open = ref(false)
const blocked = ref(false)
const feedback = ref('')
const widget = ref<HTMLElement | null>(null)
const panel = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const channels = computed(() => contactChannels.filter(c => c.value.trim()).sort((a, b) => Number(a.kind === 'qq-group') - Number(b.kind === 'qq-group')))
const publicSupportPaths = ['/home', '/login', '/register', '/email-verify', '/payment/qrcode', '/payment/result', '/payment/stripe', '/payment/airwallex']
const visible = computed(() => channels.value.length > 0 && !blocked.value && !route.path.startsWith('/admin') && !['/contact', '/payment/stripe-popup'].includes(route.path) && (
  publicSupportPaths.includes(route.path) || route.meta.requiresAuth === true
))
let observer: MutationObserver | undefined
let copyVersion = 0

function close(restoreFocus = false) {
  open.value = false
  feedback.value = ''
  copyVersion++
  if (restoreFocus && visible.value) trigger.value?.focus()
}

async function toggle() {
  if (open.value) { close(false); return }
  open.value = true
  await nextTick()
  panel.value?.querySelector<HTMLButtonElement>('button')?.focus()
}

async function copyChannel(channel: ContactChannel) {
  const version = ++copyVersion
  const success = await copyToClipboard(channel.value)
  if (open.value && version === copyVersion) feedback.value = t(success ? 'contact.copied' : 'contact.copyFailed')
}

function onPointerDown(event: Event) {
  if (open.value && event.target instanceof Node && !widget.value?.contains(event.target)) close(false)
}
function onKeyDown(event: KeyboardEvent) {
  if (open.value && event.key === 'Escape') { event.preventDefault(); close(true) }
}
watch(() => route.fullPath, () => close(false))
watch(visible, value => { if (!value) close(false) })

onMounted(() => {
  const syncDialog = () => { blocked.value = document.body.classList.contains('modal-open') || document.body.style.overflow === 'hidden' }
  syncDialog()
  observer = new MutationObserver(syncDialog)
  observer.observe(document.body, { attributes: true, attributeFilter: ['class', 'style'] })
  document.addEventListener('pointerdown', onPointerDown)
  document.addEventListener('keydown', onKeyDown)
})
onBeforeUnmount(() => {
  observer?.disconnect()
  document.removeEventListener('pointerdown', onPointerDown)
  document.removeEventListener('keydown', onKeyDown)
  copyVersion++
})
</script>

<style scoped>
.floating-support {
  --support-surface: #fff;
  --support-ink: #121316;
  --support-muted: #746f65;
  --support-line: #e7e1d4;
  --support-gold: #9c8344;
  --support-soft: #f5f1e7;
  --support-bottom: calc(24px + env(safe-area-inset-bottom, 0px) + var(--support-bottom-offset, 0px));
  position: fixed; right:24px; bottom:var(--support-bottom); z-index:40;
  color:var(--support-ink); font-family:'Noto Sans SC','Microsoft YaHei',sans-serif;
}
.dark .floating-support { --support-surface:#18191d; --support-ink:#f5f1e7; --support-muted:#aaa292; --support-line:#38352f; --support-gold:#d7c487; --support-soft:#23221f; }
.modal-open .floating-support { display:none; }
.support-trigger { display:flex; align-items:center; gap:10px; min-height:50px; padding:0 18px; border:1px solid var(--support-line); border-radius:999px; background:var(--support-surface); color:var(--support-ink); box-shadow:0 6px 24px rgb(18 19 22 / 12%); font-size:13px; font-weight:500; }
.support-monogram { color:var(--support-gold); font:500 16px Georgia,serif; border-right:1px solid var(--support-line); padding-right:10px; }
.support-panel { position:absolute; bottom:calc(100% + 14px); right:0; width:360px; max-width:calc(100vw - 48px); max-height:calc(100dvh - var(--support-bottom) - 160px); overflow-y:auto; overscroll-behavior:contain; padding:20px 24px 0; background:var(--support-surface); border:1px solid var(--support-line); border-radius:14px; box-shadow:0 16px 48px rgb(18 19 22 / 16%); }
.support-heading { display:flex; align-items:start; justify-content:space-between; gap:12px; position:sticky; top:0; background:var(--support-surface); z-index:1; padding-bottom:6px; }
.support-eyebrow { color:var(--support-gold); font:11px 'DM Mono',Consolas,monospace; margin-bottom:10px; }
.support-heading h2 { font:500 21px/1.5 'Noto Serif SC','Songti SC',Georgia,serif; }
.support-close { display:grid; place-items:center; min-width:36px; min-height:36px; border-radius:6px; color:var(--support-muted); }
.support-description { color:var(--support-muted); font-size:12px; line-height:1.8; margin:10px 0 16px; }
.support-channel { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:12px 0; border-top:1px solid var(--support-line); }
.support-channel-details { min-width:0; }
.support-label { display:block; color:var(--support-muted); font-size:12px; line-height:1.6; }
.support-value { display:block; user-select:text; font:500 15px/1.6 'DM Mono',Consolas,monospace; overflow-wrap:anywhere; }
.support-copy { display:flex; align-items:center; gap:5px; flex-shrink:0; padding:8px 11px; border:1px solid var(--support-line); border-radius:999px; font-size:12px; }
.support-feedback { min-height:18px; color:var(--support-gold); font-size:12px; line-height:1.6; margin:6px 0 12px; }
.support-complete { display:flex; align-items:center; justify-content:space-between; gap:10px; border-top:1px solid var(--support-line); padding:16px 0; color:var(--support-gold); font-size:12px; }
.support-trigger:hover, .support-copy:hover, .support-close:hover { color:var(--support-gold); background:var(--support-soft); }
.floating-support button:focus-visible, .floating-support a:focus-visible { outline:2px solid var(--support-gold); outline-offset:3px; }
@media(max-width:640px) {
  .floating-support { right:16px; --support-bottom:calc(16px + env(safe-area-inset-bottom, 0px) + var(--support-bottom-offset, 0px)); }
  .floating-support.support-payment-route { --support-bottom-offset:80px; }
  .support-panel { width:340px; max-width:calc(100vw - 32px); padding:18px 20px 0; }
  .support-trigger { min-height:46px; padding:0 14px; }
  .support-copy, .support-close { min-height:44px; }
}
</style>
