<template>
  <Teleport to="body">
    <div v-if="visible" ref="bounds" class="support-bounds">
      <div ref="widget" class="floating-support" :style="placement?.style" :class="{
        'support-moved': placement,
        'support-panel-below': placement?.below,
        'support-payment-route': route.path.startsWith('/purchase') || route.path.startsWith('/payment')
      }">
        <section v-if="open" id="mg-support-panel" ref="panel" class="support-panel" role="dialog" aria-labelledby="mg-support-title" data-testid="support-panel">
          <header class="support-heading">
            <h2 id="mg-support-title">{{ t('contact.floatingTitle') }}</h2>
            <button type="button" class="support-close" :aria-label="t('contact.close')" @click="close(true)"><Icon name="x" size="md" /></button>
          </header>
          <div class="support-channels">
            <div v-for="channel in channels" :key="channel.id" class="support-channel">
              <div class="support-channel-details"><span class="support-label">{{ t(channel.labelKey) }}</span><span class="support-value">{{ channel.value }}</span></div>
              <button type="button" class="support-copy" :data-testid="`support-copy-${channel.id}`" :aria-label="`${t('contact.copy')} · ${t(channel.labelKey)}`" @click="copyChannel(channel)"><Icon name="copy" size="sm" /><span>{{ t('contact.floatingCopy') }}</span></button>
            </div>
          </div>
          <p class="support-feedback" role="status" aria-live="polite">{{ feedback }}</p>
          <router-link to="/contact" class="support-complete" @click="close(false)">{{ t('contact.fullPage') }}<Icon name="arrowRight" size="sm" /></router-link>
        </section>
        <button ref="trigger" type="button" class="support-trigger" data-testid="support-trigger" :aria-label="t('contact.floatingTrigger')" :aria-expanded="open" aria-controls="mg-support-panel"
          @pointerdown="startDrag" @pointermove="moveDrag" @pointerup="endDrag" @touchmove.prevent
          @pointercancel="endDrag" @lostpointercapture="endDrag" @click="onTriggerClick">
          <span class="support-monogram" aria-hidden="true">MG</span>
          <svg width="19" height="19" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M4 14v-3a8 8 0 0 1 16 0v3M20 17v2a2 2 0 0 1-2 2h-3"/><rect x="3" y="11" width="4" height="7" rx="2"/><rect x="17" y="11" width="4" height="7" rx="2"/></svg>
          <span class="support-trigger-label" aria-hidden="true">{{ t('contact.floatingTrigger') }}</span>
        </button>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useResizeObserver } from '@vueuse/core'
import { contactChannels, type ContactChannel } from '@/content/contactChannels'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'

const route = useRoute()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const open = ref(false)
const blocked = ref(false)
const feedback = ref('')
const bounds = ref<HTMLElement | null>(null)
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

const placement = ref<{
  x: number
  y: number
  below: boolean
  style: Record<string, string>
} | null>(null)
let drag: { id: number; startX: number; startY: number; x: number; y: number; moved: boolean } | null = null
let suppressPointerClick = false

function placeTrigger(x: number, y: number) {
  if (!bounds.value || !trigger.value) return
  const area = bounds.value.getBoundingClientRect()
  const button = trigger.value.getBoundingClientRect()
  x = Math.max(0, Math.min(x, area.width - button.width))
  y = Math.max(0, Math.min(y, area.height - button.height))
  const below = area.height - y - button.height
  placement.value = {
    x, y, below: below > y,
    style: {
      '--support-x': `${x}px`,
      '--support-y': `${y}px`,
      '--support-area-width': `${area.width}px`,
      '--trigger-width': `${button.width}px`,
      '--room-right': `${area.width - x}px`,
      '--panel-room': `${Math.max(0, Math.max(y, below) - 10)}px`
    }
  }
}

function startDrag(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0 || drag || !trigger.value || !bounds.value) return
  const area = bounds.value.getBoundingClientRect()
  const button = trigger.value.getBoundingClientRect()
  drag = { id: event.pointerId, startX: event.clientX, startY: event.clientY, x: button.left - area.left, y: button.top - area.top, moved: false }
  suppressPointerClick = false
  trigger.value.setPointerCapture(event.pointerId)
}

function moveDrag(event: PointerEvent) {
  if (!drag || drag.id !== event.pointerId) return
  const dx = event.clientX - drag.startX
  const dy = event.clientY - drag.startY
  if (!drag.moved && Math.hypot(dx, dy) < 6) return
  drag.moved = true
  placeTrigger(drag.x + dx, drag.y + dy)
}

function endDrag(event: PointerEvent) {
  if (drag?.id !== event.pointerId) return
  const released = event.type === 'pointerup'
  const touchTap = released && event.pointerType === 'touch' && !drag.moved
  suppressPointerClick = drag.moved || !released || touchTap
  drag = null
  if (trigger.value?.hasPointerCapture(event.pointerId)) trigger.value.releasePointerCapture(event.pointerId)
  // Some touch browsers omit the compatibility click on the first tap following a drag.
  if (touchTap) void toggle()
}

useResizeObserver([bounds, trigger], () => {
  if (placement.value) placeTrigger(placement.value.x, placement.value.y)
})

function onTriggerClick(event: MouseEvent) {
  if (suppressPointerClick && event.detail !== 0) {
    suppressPointerClick = false
    return
  }
  void toggle()
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
watch(visible, value => {
  if (!value) {
    drag = null
    suppressPointerClick = false
    close(false)
  }
})

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
.support-bounds {
  position:fixed; z-index:40; pointer-events:none;
  inset:max(8px, env(safe-area-inset-top)) max(8px, env(safe-area-inset-right)) max(8px, env(safe-area-inset-bottom)) max(8px, env(safe-area-inset-left));
}
.floating-support.support-moved { position:absolute; left:var(--support-x); top:var(--support-y); right:auto; bottom:auto; }
.support-moved .support-panel {
  left:0; right:auto; max-width:var(--support-area-width);
  max-height:min(var(--support-panel-limit, 360px), var(--panel-room));
  transform:translateX(clamp(calc(0px - var(--support-x)), calc(var(--trigger-width) - 100%), calc(var(--room-right) - 100%)));
}
.support-panel-below .support-panel { top:calc(100% + 10px); bottom:auto; }
.floating-support {
  --support-surface: #fff;
  --support-ink: #121316;
  --support-muted: #746f65;
  --support-line: #e7e1d4;
  --support-gold: #9c8344;
  --support-soft: #f5f1e7;
  --support-bottom: calc(24px + env(safe-area-inset-bottom, 0px) + var(--support-bottom-offset, 0px));
  position: fixed; right:24px; bottom:var(--support-bottom); pointer-events:auto;
  color:var(--support-ink); font-family:'Noto Sans SC','Microsoft YaHei',sans-serif;
}
.dark .floating-support { --support-surface:#18191d; --support-ink:#f5f1e7; --support-muted:#aaa292; --support-line:#38352f; --support-gold:#d7c487; --support-soft:#23221f; }
.modal-open .floating-support { display:none; }
.support-trigger { touch-action:none; user-select:none; cursor:grab; display:flex; align-items:center; gap:0; min-height:50px; padding:0 18px; border:1px solid var(--support-line); border-radius:999px; background:var(--support-surface); color:var(--support-ink); box-shadow:0 6px 24px rgb(18 19 22 / 12%); font-size:13px; font-weight:500; }
.support-trigger:active { cursor:grabbing; }
/* Keep the native tap target and the pointer-capture target identical on touchscreens. */
.support-trigger > * { pointer-events:none; }
.support-trigger > svg { flex-shrink:0; margin-left:10px; }
.support-trigger-label { overflow:hidden; max-width:0; opacity:0; margin-left:0; white-space:nowrap; transition:max-width 180ms ease, opacity 180ms ease, margin-left 180ms ease; }
.support-trigger:focus-visible .support-trigger-label { max-width:12em; opacity:1; margin-left:10px; }
@media(hover:hover) and (pointer:fine) {
  .support-trigger:hover .support-trigger-label { max-width:12em; opacity:1; margin-left:10px; }
}
@media(prefers-reduced-motion:reduce) {
  .support-trigger-label { transition:none; }
}
.support-monogram { color:var(--support-gold); font:500 16px Georgia,serif; border-right:1px solid var(--support-line); padding-right:10px; }
.support-panel { position:absolute; bottom:calc(100% + 10px); right:0; width:300px; max-width:calc(100vw - 48px); max-height:min(360px, calc(100dvh - var(--support-bottom) - 160px)); overflow-y:auto; overscroll-behavior:contain; padding:14px 16px 0; background:var(--support-surface); border:1px solid var(--support-line); border-radius:12px; box-shadow:0 12px 32px rgb(18 19 22 / 12%); }
.support-heading { display:flex; align-items:center; justify-content:space-between; gap:8px; position:sticky; top:0; background:var(--support-surface); z-index:1; padding-bottom:8px; }
.support-heading h2 { font:500 17px/1.4 'Noto Serif SC','Songti SC',Georgia,serif; }
.support-close { display:grid; place-items:center; min-width:32px; min-height:32px; border-radius:6px; color:var(--support-muted); }
.support-channel { display:flex; align-items:center; justify-content:space-between; gap:8px; padding:7px 0; border-top:1px solid var(--support-line); }
.support-channel-details { min-width:0; }
.support-label { display:block; color:var(--support-muted); font-size:11.5px; line-height:1.4; }
.support-value { display:block; user-select:text; font:500 13.5px/1.5 'DM Mono',Consolas,monospace; overflow-wrap:anywhere; }
.support-copy { display:flex; align-items:center; gap:4px; flex-shrink:0; padding:6px 8px; border:1px solid var(--support-line); border-radius:999px; font-size:12px; }
.support-feedback { color:var(--support-gold); font-size:12px; line-height:1.6; margin:6px 0; }
.support-feedback:empty { margin:0; height:0; }
.support-complete { display:flex; align-items:center; justify-content:space-between; gap:8px; border-top:1px solid var(--support-line); padding:10px 0; color:var(--support-gold); font-size:12px; }
.support-trigger:hover, .support-copy:hover, .support-close:hover { color:var(--support-gold); background:var(--support-soft); }
.floating-support button:focus-visible, .floating-support a:focus-visible { outline:2px solid var(--support-gold); outline-offset:3px; }
@media(max-width:640px) {
  .floating-support { --support-panel-limit:min(340px, 48dvh); right:16px; --support-bottom:calc(16px + env(safe-area-inset-bottom, 0px) + var(--support-bottom-offset, 0px)); }
  .floating-support.support-payment-route { --support-bottom-offset:80px; }
  .support-panel { width:280px; max-width:calc(100vw - 32px); max-height:min(340px, 48dvh, calc(100dvh - var(--support-bottom) - 160px)); padding:12px 14px 0; }
  .support-trigger { min-height:46px; padding:0 14px; }
  .support-copy, .support-close { min-height:44px; }
}
</style>
