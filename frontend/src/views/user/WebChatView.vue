<template>
  <AppLayout>
    <div class="webchat" :aria-label="t('webChat.title')">
      <div class="chat-settings">
        <label class="chat-select">
          <span>{{ t('webChat.key') }}</span>
          <select data-testid="chat-key" :value="keyId ?? ''" :disabled="mutating || !keys.length" @change="changeKey">
            <option disabled value="">{{ t('webChat.chooseKey') }}</option>
            <option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name || '#' + key.id }}</option>
          </select>
        </label>
        <label class="chat-select">
          <span>{{ t('webChat.model') }}</span>
          <select v-model="model" data-testid="chat-model" :disabled="mutating || !optionsLoaded || !models.length">
            <option disabled value="">{{ t('webChat.chooseModel') }}</option>
            <option v-for="item in models" :key="item.id" :value="item.id">{{ item.id }}</option>
          </select>
        </label>
        <button type="button" class="chat-icon-button" :disabled="!!operation" :aria-label="t('webChat.reload')" :title="t('webChat.reload')" @click="refresh">
          <Icon name="refresh" size="sm" />
        </button>
      </div>

      <div v-if="errorKey" class="chat-error" role="alert">
        <span>{{ t('webChat.errors.' + errorKey) }}</span>
        <button type="button" :disabled="!!operation" @click="refresh">{{ t('webChat.reload') }}</button>
      </div>

      <div class="chat-workspace">
        <aside class="chat-history" data-testid="chat-history" :aria-label="t('webChat.history')">
          <h2 class="desktop-history-title">{{ t('webChat.history') }}</h2>
          <button type="button" class="mobile-history-toggle" aria-controls="chat-history-list" :aria-expanded="showHistory" @click="showHistory = !showHistory">
            <Icon name="chatBubble" size="sm" />{{ t('webChat.history') }}
            <Icon :name="showHistory ? 'chevronUp' : 'chevronDown'" size="sm" />
          </button>
          <button type="button" class="chat-new" :disabled="!!operation || !keyId" @click="startChat">
            <Icon name="plus" size="sm" />{{ t('webChat.newChat') }}
          </button>
          <div id="chat-history-list" class="history-list" :class="{ 'mobile-open': showHistory }">
            <p v-if="optionsLoaded && !sessions.length">{{ t('webChat.noHistory') }}</p>
            <button v-for="item in sessions" :key="item.id" type="button" :disabled="mutating"
              :aria-current="item.id === selectedId ? 'page' : undefined" @click="viewSession(item.id)">
              <span>{{ item.title || t('webChat.untitled') }}</span>
              <time :datetime="item.updated_at">{{ formatDate(item.updated_at) }}</time>
            </button>
          </div>
        </aside>

        <section class="conversation" :aria-label="t('webChat.title')">
          <header class="conversation-header">
            <h2>{{ session?.title || (selectedId ? t('webChat.history') : t('webChat.newChat')) }}</h2>
            <button v-if="session" type="button" class="chat-icon-button" :disabled="!!operation"
              :aria-label="t('webChat.delete')" :title="t('webChat.delete')" @click="deleteTarget = session.id">
              <Icon name="trash" size="sm" />
            </button>
          </header>
          <div ref="timeline" class="chat-timeline" role="log" aria-live="polite" :aria-label="t('webChat.title')">
            <p v-if="operation === 'keys' || operation === 'options' || operation === 'session'" class="chat-status" role="status">{{ t('webChat.loading') }}</p>
            <div v-else-if="keysLoaded && !keys.length" class="chat-empty">
              <span class="chat-mark" aria-hidden="true"><Icon name="key" size="lg" /></span>
              <h3>{{ t('webChat.noKeys') }}</h3><p>{{ t('webChat.noKeysHint') }}</p>
              <RouterLink to="/keys" class="manage-link">{{ t('webChat.manageKeys') }} <Icon name="arrowRight" size="sm" /></RouterLink>
            </div>
            <div v-else-if="!selectedId && !pending" class="chat-empty">
              <span class="chat-mark" aria-hidden="true"><Icon name="chatBubble" size="lg" /></span>
              <h3>{{ t('webChat.startTitle') }}</h3>
              <p>{{ optionsLoaded && !models.length ? t('webChat.noModels') : t('webChat.startHint') }}</p>
            </div>
            <article v-for="(turn, i) in session?.messages" :key="i" class="chat-turn" :class="{ 'user-turn': turn.role === 'user' }" data-testid="chat-message">
              <strong>{{ turn.role === 'user' ? t('webChat.you') : t('webChat.assistant') }}</strong>
              <p>{{ turn.content }}</p>
            </article>
            <template v-if="pending">
              <article class="chat-turn user-turn"><strong>{{ t('webChat.you') }}</strong><p>{{ pending }}</p></article>
              <p class="chat-status" role="status">{{ t('webChat.waiting') }}</p>
            </template>
          </div>
          <form class="chat-composer" @submit.prevent="send">
            <textarea v-model="draft" data-testid="chat-draft" :aria-label="t('webChat.draft')" :placeholder="t('webChat.placeholder')"
              rows="2" :disabled="!keyId || !optionsLoaded || !model || !!operation || needsReload"
              :aria-invalid="tooLong || undefined" :aria-describedby="tooLong ? 'chat-length-error' : undefined"
              @keydown="composerKeydown" />
            <button data-testid="chat-send" type="submit" class="chat-send" :disabled="!canSend" :title="t('webChat.send')">
              <Icon name="arrowUp" size="sm" /><span>{{ t('webChat.send') }}</span>
            </button>
          </form>
          <p v-if="tooLong" id="chat-length-error" class="length-error" role="status">{{ t('webChat.tooLong') }}</p>
        </section>
      </div>
      <footer class="chat-footer">
        <span>{{ t('webChat.retention') }}</span>
        <a href="https://openwebui.com/" target="_blank" rel="noopener noreferrer">Powered by Open WebUI</a>
      </footer>
    </div>
    <ConfirmDialog :show="!!deleteTarget" :title="t('webChat.deleteTitle')" :message="t('webChat.deleteHint')"
      :confirm-text="t('webChat.delete')" danger @confirm="confirmRemoval" @cancel="deleteTarget = ''" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useWebChat } from '@/composables/useWebChat'

const { t, locale } = useI18n()
const { keys, models, sessions, keyId, model, selectedId, session, draft, pending, errorKey,
  operation, keysLoaded, optionsLoaded, mutating, tooLong, canSend, needsReload,
  refresh, chooseKey, newChat, openSession, send, remove } = useWebChat()
const timeline = ref<HTMLElement>()
const showHistory = ref(false)
const deleteTarget = ref('')
const dateFormat = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }))
function formatDate(value: string) { return dateFormat.value.format(new Date(value)) }
function changeKey(event: Event) { void chooseKey(Number((event.target as HTMLSelectElement).value)) }
function startChat() { showHistory.value = false; newChat() }
function viewSession(id: string) { showHistory.value = false; void openSession(id) }
function confirmRemoval() {
  const id = deleteTarget.value
  deleteTarget.value = ''
  if (id === session.value?.id) void remove()
}
function composerKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !event.isComposing) {
    event.preventDefault()
    void send()
  }
}
watch([session, pending], async () => {
  await nextTick()
  if (timeline.value) timeline.value.scrollTop = timeline.value.scrollHeight
})
</script>

<style scoped>
.webchat {
  color: var(--mg-ink-900);
  max-width: 1360px;
  margin: 0 auto;
}
.chat-settings, .chat-footer, .chat-error, .conversation-header, .chat-composer { display: flex; align-items: center; gap: 12px; }
.chat-settings { flex-wrap: wrap; gap: 12px 20px; margin-bottom: 16px; }
.chat-select { display: flex; align-items: center; gap: 8px; min-width: 0; font-size: 13px; }
.chat-select > span { color: var(--mg-muted); white-space: nowrap; }
.chat-select select {
  max-width: min(33vw, 320px); width: 230px; min-width: 0; padding: 8px;
  color: var(--mg-ink-900); background: var(--mg-surface); border: 1px solid var(--mg-line-warm); border-radius: 6px;
}
.chat-settings .chat-icon-button { margin-left: auto; }
.chat-icon-button { display: inline-grid; place-items: center; width: 36px; height: 36px; border: 1px solid var(--mg-line-warm); border-radius: 6px; color: var(--mg-ink-700); background: var(--mg-surface); }
.webchat button:disabled, .webchat select:disabled { opacity: .5; cursor: not-allowed; }
.chat-workspace { display: grid; grid-template-columns: minmax(170px, 22%) minmax(0, 1fr); min-height: 375px; height: clamp(440px, calc(100dvh - 250px), 900px); border: 1px solid var(--mg-line-warm); border-radius: 12px; overflow: hidden; background: var(--mg-surface); }
.chat-history { min-width: 0; padding: 18px 14px; display: flex; flex-direction: column; gap: 12px; background: var(--mg-gold-50); border-right: 1px solid var(--mg-line-warm); }
.chat-history h2, .conversation-header h2 { font-size: 15px; line-height: 1.5; font-weight: 650; }
.chat-new, .mobile-history-toggle, .manage-link { display: inline-flex; align-items: center; justify-content: center; gap: 6px; }
.chat-new { border: 1px solid var(--mg-line-warm); border-radius: 6px; background: var(--mg-surface); padding: 7px 10px; font-size: 13px; }
.mobile-history-toggle { display: none; }
.history-list { overflow-y: auto; min-height: 0; }
.history-list > p, .chat-status { color: var(--mg-muted); font-size: 13px; line-height: 1.6; }
.history-list button { display: grid; width: 100%; gap: 3px; padding: 9px 10px; border-radius: 6px; text-align: left; }
.history-list button[aria-current], .history-list button:hover { background: var(--mg-champagne-50); }
.history-list button span, .conversation-header h2 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.history-list button span { font-size: 13px; }
.history-list time { color: var(--mg-muted); font-size: 11px; }
.conversation { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.conversation-header { justify-content: space-between; min-height: 52px; flex: none; padding: 9px 20px; border-bottom: 1px solid var(--mg-line-warm); }
.chat-timeline { flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 20px; display: flex; flex-direction: column; gap: 14px; }
.chat-empty { margin: auto; max-width: 410px; padding: 14px; text-align: center; }
.chat-mark { display: inline-grid; place-items: center; width: 48px; height: 48px; border-radius: 12px; color: var(--mg-gold-700); background: var(--mg-champagne-50); }
.chat-empty h3 { font-family: Georgia, 'Noto Serif SC', serif; font-weight: 600; font-size: clamp(19px, 2.1vw, 27px); margin: 12px 0 8px; }
.chat-empty p { color: var(--mg-muted); font-size: 13px; line-height: 1.8; }
.manage-link { margin-top: 12px; text-decoration: underline; text-underline-offset: 3px; }
.chat-turn { max-width: 90%; align-self: flex-start; }
.chat-turn strong { display: block; font-size: 11px; color: var(--mg-muted); margin-bottom: 5px; }
.chat-turn p { font-size: 14px; line-height: 1.7; overflow-wrap: anywhere; white-space: pre-wrap; }
.user-turn { align-self: flex-end; padding: 10px 14px; background: var(--mg-champagne-50); border-radius: 12px 12px 2px 12px; }
.chat-composer { margin: 0 16px 14px; padding: 8px; align-items: end; border: 1px solid var(--mg-line-warm); border-radius: 9px; }
.chat-composer:focus-within { border-color: var(--mg-gold-500); }
.chat-composer textarea { min-width: 0; flex: 1; max-height: 140px; padding: 4px 6px; resize: vertical; font-size: 14px; color: inherit; background: transparent; }
.chat-composer textarea:focus-visible { outline: 2px solid var(--mg-gold-500); border-radius: 3px; }
.chat-send { display: inline-flex; align-items: center; justify-content: center; gap: 4px; padding: 7px 11px; border-radius: 6px; color: #fbfaf7; background: #24252a; font-weight: 600; font-size: 13px; }
.dark .chat-send { background: #e3d4af; color: #241f14; }
.chat-footer { justify-content: flex-start; flex-wrap: wrap; gap: 6px 14px; padding: 10px calc(7rem + env(safe-area-inset-right, 0px)) 0 0; color: var(--mg-muted); font-size: 12px; }
.chat-footer a, .chat-error button { text-decoration: underline; text-underline-offset: 3px; }
.chat-error { justify-content: space-between; margin-bottom: 12px; padding: 10px 12px; border: 1px solid var(--mg-line-warm); border-radius: 6px; color: var(--mg-ink-900); background: var(--mg-champagne-50); font-size: 13px; }
.chat-error button { flex: none; }
.length-error { margin: -8px 18px 10px; color: var(--mg-danger); font-size: 12px; }
@media (max-width: 760px) {
  .chat-select { flex: 1 1 0; flex-direction: column; align-items: stretch; gap: 3px; }
  .chat-select select { max-width: 100%; width: 100%; }
  .chat-settings { align-items: flex-end; gap: 8px; }
  .chat-workspace { display: flex; flex-direction: column; height: clamp(440px, calc(100dvh - 220px), 900px); }
  .chat-history { flex: none; flex-flow: row wrap; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 12px; border: 0; border-bottom: 1px solid var(--mg-line-warm); }
  .desktop-history-title { display: none; }
  .mobile-history-toggle { display: inline-flex; font-size: 14px; }
  .history-list { display: none; width: 100%; max-height: 130px; }
  .history-list.mobile-open { display: block; }
  .conversation { flex: 1; }
  .conversation-header { padding: 6px 12px; min-height: 42px; }
  .chat-timeline { padding: 12px; }
  .chat-composer { margin: 0 10px 10px; }
  .chat-footer { align-items: flex-start; flex-direction: column; padding-bottom: 50px; }
}
</style>
