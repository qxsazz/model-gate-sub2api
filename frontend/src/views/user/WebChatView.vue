<template>
  <AppLayout>
    <div class="webchat" :aria-label="t('webChat.title')">
      <div v-if="errorKey" class="chat-error" role="alert">
        <span>{{ errorKey === 'gateway' ? gatewayMessage : t('webChat.errors.' + errorKey) }}</span>
        <button type="button" :disabled="!!operation" @click="refresh">{{ t('webChat.reload') }}</button>
      </div>

      <div class="chat-workspace" :class="{ 'workspace-welcome': showWelcome }">
        <aside class="chat-history" data-testid="chat-history" :aria-label="t('webChat.history')">
          <button type="button" class="mobile-history-toggle" aria-controls="chat-history-list" :aria-expanded="showHistory" @click="showHistory = !showHistory">
            <Icon name="chatBubble" size="sm" />{{ t('webChat.history') }}
            <Icon :name="showHistory ? 'chevronUp' : 'chevronDown'" size="sm" />
          </button>
          <button type="button" class="chat-new" :disabled="!!operation || !keyId" @click="startChat">
            <Icon name="plus" size="sm" />{{ t('webChat.newChat') }}
          </button>
          <h2 class="desktop-history-title">{{ t('webChat.history') }}</h2>
          <div id="chat-history-list" class="history-list" :class="{ 'mobile-open': showHistory }">
            <p v-if="optionsLoaded && !sessions.length">{{ t('webChat.noHistory') }}</p>
            <button v-for="item in sessions" :key="item.id" type="button" :disabled="mutating"
              :aria-current="item.id === selectedId ? 'page' : undefined" @click="viewSession(item.id)">
              <span>{{ item.title || t('webChat.untitled') }}</span>
              <time :datetime="item.updated_at">{{ formatDate(item.updated_at) }}</time>
            </button>
          </div>
        </aside>

        <div class="settings-slot">
          <button type="button" class="settings-trigger" data-testid="chat-settings" aria-haspopup="dialog"
            :aria-expanded="showSettings" @click="showSettings = true">
            <Icon name="cog" size="md" aria-hidden="true" />
            <span>{{ t('webChat.settings') }}</span>
            <small>{{ activeKeyName || t('webChat.chooseKey') }}</small>
          </button>
        </div>

        <section class="conversation" :aria-label="t('webChat.title')">
          <header v-if="selectedId" class="conversation-header">
            <h2>{{ session?.title || t('webChat.history') }}</h2>
            <button v-if="session" type="button" class="chat-icon-button" :disabled="!!operation"
              :aria-label="t('webChat.delete')" :title="t('webChat.delete')" @click="deleteTarget = session.id">
              <Icon name="trash" size="sm" />
            </button>
          </header>
          <div v-if="chatReady && !needsReload" class="guide-slot" :class="{ 'guide-welcome': showWelcome }">
            <h2 v-if="showWelcome" class="welcome-title">{{ t('webChat.welcome') }}</h2>
            <details class="chat-guide" :open="showWelcome">
              <summary>
                <Icon name="book" size="sm" aria-hidden="true" />
                {{ t('webChat.guide.title') }}
                <Icon name="chevronDown" size="xs" class="guide-chevron" aria-hidden="true" />
              </summary>
              <dl>
                <dt>{{ t('webChat.guide.setup') }}</dt><dd>{{ t('webChat.guide.setupHint') }}</dd>
                <dt>{{ t('webChat.guide.text') }}</dt><dd>{{ t('webChat.guide.textHint') }}</dd>
                <dt>{{ t('webChat.guide.image') }}</dt><dd>{{ t('webChat.guide.imageHint') }}</dd>
              </dl>
            </details>
          </div>
          <div v-show="!showWelcome" ref="timeline" class="chat-timeline" role="log" aria-live="polite" :aria-label="t('webChat.title')">
            <p v-if="operation === 'keys' || operation === 'options' || operation === 'session'" class="chat-status" role="status">{{ t('webChat.loading') }}</p>
            <div v-else-if="keysLoaded && !keys.length" class="chat-empty">
              <span class="chat-mark" aria-hidden="true"><Icon name="key" size="lg" /></span>
              <h3>{{ t('webChat.noKeys') }}</h3><p>{{ t('webChat.noKeysHint') }}</p>
              <RouterLink to="/keys" class="manage-link">{{ t('webChat.manageKeys') }} <Icon name="arrowRight" size="sm" /></RouterLink>
            </div>
            <div v-else-if="keysLoaded && !keyId" class="chat-empty">
              <span class="chat-mark" aria-hidden="true"><Icon name="key" size="lg" /></span>
              <h3>{{ t('webChat.chooseKey') }}</h3><p>{{ t('webChat.selectKeyHint') }}</p>
              <button type="button" class="manage-link" aria-haspopup="dialog" @click="showSettings = true">{{ t('webChat.openSettings') }} <Icon name="arrowRight" size="sm" /></button>
            </div>
            <div v-else-if="keyId && optionsLoaded && !models.length && !selectedId" class="chat-empty">
              <h3>{{ t('webChat.noModelsTitle') }}</h3><p>{{ t('webChat.noModels') }}</p>
              <button type="button" class="manage-link" aria-haspopup="dialog" @click="showSettings = true">{{ t('webChat.openSettings') }} <Icon name="arrowRight" size="sm" /></button>
            </div>
            <article v-for="(turn, i) in session?.messages" :key="i" class="chat-turn" :class="{ 'user-turn': turn.role === 'user' }" data-testid="chat-message">
              <strong>{{ turn.role === 'user' ? t('webChat.you') : t('webChat.assistant') }}</strong>
              <p v-if="turn.content">{{ turn.content }}</p>
              <ChatFileList v-if="turn.attachments?.length" :files="turn.attachments" data-testid="chat-message-attachments"
                @open="openOriginal" />
            </article>
            <template v-if="operation === 'send'">
              <article class="chat-turn user-turn"><strong>{{ t('webChat.you') }}</strong><p v-if="pending">{{ pending }}</p>
                <ChatFileList v-if="pendingFiles.length" :files="pendingFiles" />
              </article>
              <p class="chat-status" role="status">{{ t('webChat.waiting') }}</p>
            </template>
          </div>
          <form class="chat-composer" @submit.prevent="send">
            <ChatFileList v-if="attachments.length" :files="attachments" removable :disabled="readingFiles || !!operation"
              data-testid="chat-draft-attachments" @remove="removeFile" />
            <p v-if="readingFiles" class="file-status" role="status">{{ t('webChat.files.reading') }}</p>
            <p v-if="fileError" class="file-status" role="alert">{{ t('webChat.files.errors.' + fileError) }}</p>
            <textarea v-model="draft" data-testid="chat-draft" :aria-label="t('webChat.draft')" :placeholder="t(attachments.length ? 'webChat.imageQuestion' : 'webChat.placeholder')"
              rows="2" :disabled="!chatReady || !!operation || needsReload"
              :aria-invalid="tooLong || undefined" :aria-describedby="tooLong ? 'chat-length-error' : attachments.length ? 'chat-attachment-hint' : undefined"
              @keydown="composerKeydown" />
            <div class="composer-row">
              <input ref="fileInput" type="file" multiple hidden data-testid="chat-file-input" :accept="chatFileAccept" @change="selectFiles" />
              <button data-testid="chat-attach" type="button" class="chat-attach" :disabled="!chatReady || !!operation || readingFiles || needsReload"
                :aria-describedby="chatReady ? 'chat-attachment-hint' : undefined" :title="t('webChat.files.hint')" @click="fileInput?.click()">
                <Icon name="plus" size="sm" aria-hidden="true" /><span>{{ t('webChat.files.attach') }}</span>
              </button>
              <label class="model-picker">
                <span class="sr-only">{{ t('webChat.model') }}</span>
                <span class="model-value" aria-hidden="true">{{ model || t('webChat.chooseModel') }}</span>
                <Icon name="chevronDown" size="xs" aria-hidden="true" />
                <select v-model="model" data-testid="chat-model" :disabled="!!operation || !optionsLoaded || !models.length"
                  :title="model || t('webChat.chooseModel')">
                  <option disabled value="">{{ t('webChat.chooseModel') }}</option>
                  <optgroup v-for="[family, items] in modelGroups" :key="family" :label="family || t('webChat.otherModels')">
                    <option v-for="item in items" :key="item" :value="item">{{ item }}</option>
                  </optgroup>
                </select>
              </label>
              <button data-testid="chat-send" type="submit" class="chat-send" :disabled="!canSend" :aria-label="t('webChat.send')" :title="t('webChat.send')">
                <Icon name="arrowUp" size="md" aria-hidden="true" />
              </button>
            </div>
            <small v-if="chatReady" id="chat-attachment-hint" class="file-hint">{{ t(attachments.length ? 'webChat.files.readyHint' : 'webChat.files.hint') }}</small>
            <small v-else-if="selectedId && optionsLoaded && !model" class="file-hint">{{ t(models.length ? 'webChat.selectModelHint' : 'webChat.noModels') }}</small>
          </form>
          <p v-if="tooLong" id="chat-length-error" class="length-error" role="status">{{ t('webChat.tooLong') }}</p>
        </section>
      </div>
      <p v-if="originalError" class="file-status" role="alert">{{ t('webChat.files.errors.original') }}</p>
      <footer class="chat-footer">{{ t('webChat.retention') }}</footer>
    </div>
    <BaseDialog :show="showSettings" :title="t('webChat.settings')" width="narrow" close-on-click-outside @close="showSettings = false">
      <label class="key-field text-gray-900 dark:text-gray-100">
        <span>{{ t('webChat.key') }}</span>
        <select class="input" data-testid="chat-key" :value="keyId ?? ''" :disabled="mutating || !keys.length" @change="changeKey">
          <option disabled value="">{{ t('webChat.chooseKey') }}</option>
          <option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name || '#' + key.id }}</option>
        </select>
      </label>
      <p class="key-hint text-gray-600 dark:text-gray-300">{{ t('webChat.keyHint') }}</p>
      <template #footer>
        <RouterLink to="/keys" class="btn btn-secondary">{{ t('webChat.manageKeys') }}</RouterLink>
        <button type="button" class="btn btn-secondary" :disabled="!!operation" @click="refresh"><Icon name="refresh" size="sm" />{{ t('webChat.reload') }}</button>
      </template>
    </BaseDialog>
    <BaseDialog :show="!!imagePreview" :title="imagePreview?.name || ''" @close="closeOriginal">
      <img v-if="imagePreview" :src="imagePreview.url" :alt="imagePreview.name" class="original-image" />
      <template #footer>
        <a v-if="imagePreview" class="btn btn-secondary" :href="imagePreview.url" :download="imagePreview.name">{{ t('webChat.files.save') }}</a>
      </template>
    </BaseDialog>
    <ConfirmDialog :show="!!deleteTarget" :title="t('webChat.deleteTitle')" :message="t('webChat.deleteHint')"
      :confirm-text="t('webChat.delete')" danger @confirm="confirmRemoval" @cancel="deleteTarget = ''" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ChatFileList from '@/components/webChat/ChatFileList.vue'
import { webChatAPI, type ChatAttachment } from '@/api/webChat'
import { chatFileAccept, isChatImage } from '@/utils/webChatAttachments'
import { useWebChat } from '@/composables/useWebChat'

const { t, locale } = useI18n()
const { attachments, pendingFiles, readingFiles, fileError, addFiles, removeFile, keys, models, sessions, keyId, model, selectedId, session, draft, pending, errorKey, gatewayMessage,
  operation, keysLoaded, optionsLoaded, mutating, tooLong, chatReady, canSend, needsReload,
  refresh, chooseKey, newChat, openSession, send, remove } = useWebChat()
const activeKeyName = computed(() => {
  const key = keys.value.find(item => item.id === keyId.value)
  return key ? key.name || '#' + key.id : ''
})
// Family headings are for display only. Send the gateway's original ID, including custom aliases.
const modelGroups = computed(() => {
  const groups = new Map<string, string[]>()
  for (const { id } of models.value) {
    const name = id.split('/').pop()?.toLowerCase() || ''
    let family = ''
    if (/^(?:chatgpt|gpt|o[1-9](?:-|$))/.test(name)) family = 'GPT'
    else if (name.startsWith('gemini')) family = 'Gemini'
    else if (name.startsWith('claude')) family = 'Claude'
    const items = groups.get(family)
    if (items) items.push(id)
    else groups.set(family, [id])
  }
  return groups
})
const showWelcome = computed(() => chatReady.value && !selectedId.value && !operation.value && !needsReload.value)
const timeline = ref<HTMLElement>()
const showHistory = ref(false)
const showSettings = ref(false)
const deleteTarget = ref('')
const fileInput = ref<HTMLInputElement>()
const imagePreview = ref<{ name: string; url: string } | null>(null)
const originalError = ref(false)
let originalRequest: AbortController | undefined

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  void addFiles(files)
}

function closeOriginal() {
  originalRequest?.abort()
  if (imagePreview.value) URL.revokeObjectURL(imagePreview.value.url)
  imagePreview.value = null
  originalError.value = false
}

async function openOriginal(file: ChatAttachment) {
  const key = keyId.value, id = session.value?.id
  if (!key || !id) return
  closeOriginal()
  const controller = new AbortController()
  originalRequest = controller
  try {
    const bytes = await webChatAPI.attachment(key, id, file.id, controller.signal)
    if (controller.signal.aborted) return
    const image = isChatImage(file.mime_type)
    const url = URL.createObjectURL(new Blob([bytes], { type: image ? file.mime_type : 'application/octet-stream' }))
    if (image) imagePreview.value = { name: file.name, url }
    else {
      const link = document.createElement('a')
      link.href = url
      link.download = file.name
      document.body.append(link)
      link.click()
      link.remove()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    }
  } catch {
    if (!controller.signal.aborted) originalError.value = true
  }
}

watch([keyId, () => session.value?.id], closeOriginal, { flush: 'sync' })
onBeforeUnmount(closeOriginal)
const dateFormat = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }))
function formatDate(value: string) { return dateFormat.value.format(new Date(value)) }
function changeKey(event: Event) {
  const id = Number((event.target as HTMLSelectElement).value)
  if (id !== keyId.value) void chooseKey(id)
  showSettings.value = false
}
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
.chat-error, .conversation-header, .composer-row { display: flex; align-items: center; gap: 12px; }
.key-field { display: grid; gap: 8px; font-weight: 600; }
.key-field select { width: 100%; min-width: 0; font-weight: 400; }
.key-hint { margin: 10px 0 0; font-size: 13px; line-height: 1.6; }
.chat-icon-button { display: inline-grid; place-items: center; width: 36px; height: 36px; border: 1px solid var(--mg-line-warm); border-radius: 6px; color: var(--mg-ink-700); background: var(--mg-surface); }
.webchat button:disabled, .webchat select:disabled { opacity: .5; cursor: not-allowed; }
.chat-workspace {
  display: grid; grid-template: 'history conversation' minmax(0, 1fr) 'settings conversation' auto / clamp(180px, 22%, 250px) minmax(0, 1fr);
  height: clamp(440px, calc(100dvh - 175px), 1000px); border: 1px solid var(--mg-line-warm); border-radius: 16px; overflow: hidden; background: var(--mg-surface);
}
.chat-history, .settings-slot { background: var(--mg-gold-50); border-right: 1px solid var(--mg-line-warm); }
.chat-history { grid-area: history; min-width: 0; min-height: 0; padding: 16px 12px; display: flex; flex-direction: column; gap: 10px; }
.settings-slot { grid-area: settings; min-width: 0; padding: 8px; border-top: 1px solid var(--mg-line-warm); }
.settings-trigger { width: 100%; display: flex; align-items: center; gap: 8px; padding: 8px; min-height: 40px; border-radius: 9px; font-size: 14px; }
.settings-trigger > svg { flex: none; color: var(--mg-gold-700); }
.settings-trigger small { margin-left: auto; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--mg-muted); font-size: 12px; }
.settings-trigger:hover, .chat-new:hover:enabled, .chat-attach:hover:enabled, .model-picker:has(select:hover:enabled) { background: var(--mg-champagne-50); }
.chat-history h2, .conversation-header h2 { font-size: 15px; line-height: 1.5; font-weight: 650; }
.chat-new, .mobile-history-toggle, .manage-link { display: inline-flex; align-items: center; justify-content: center; gap: 6px; }
.chat-new { flex: none; justify-content: flex-start; gap: 8px; border: 1px solid var(--mg-line-warm); border-radius: 9px; background: var(--mg-surface); padding: 9px 10px; font-size: 14px; }
.chat-history .desktop-history-title { margin: 12px 8px 0; font-size: 12px; color: var(--mg-muted); }
.history-list > p { padding: 2px 8px; }
.mobile-history-toggle { display: none; }
.history-list { overflow-y: auto; min-height: 0; }
.history-list > p, .chat-status { color: var(--mg-muted); font-size: 13px; line-height: 1.6; }
.history-list button { display: grid; width: 100%; gap: 3px; padding: 9px 10px; border-radius: 6px; text-align: left; }
.history-list button[aria-current], .history-list button:hover { background: var(--mg-champagne-50); }
.history-list button span, .conversation-header h2 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.history-list button span { font-size: 13px; }
.history-list time { color: var(--mg-muted); font-size: 11px; }
.conversation { grid-area: conversation; display: flex; flex-direction: column; min-width: 0; min-height: 0; }
.conversation-header { justify-content: space-between; min-height: 52px; flex: none; padding: 9px 20px; border-bottom: 1px solid var(--mg-line-warm); }
.chat-timeline { flex: 1; min-height: 0; overflow: auto; overscroll-behavior: contain; padding: 20px max(20px, calc((100% - 48rem) / 2)); display: flex; flex-direction: column; gap: 14px; }
.chat-empty { margin: auto; max-width: 410px; padding: 14px; text-align: center; }
.chat-mark { display: inline-grid; place-items: center; width: 48px; height: 48px; border-radius: 12px; color: var(--mg-gold-700); background: var(--mg-champagne-50); }
.chat-empty h3 { font-family: Georgia, 'Noto Serif SC', serif; font-weight: 600; font-size: clamp(19px, 2.1vw, 27px); margin: 12px 0 8px; }
.chat-empty p { color: var(--mg-muted); font-size: 13px; line-height: 1.8; }
.guide-slot { min-height: 0; max-height: 45%; overflow-y: auto; padding: 3px 16px 0; }
.guide-slot.guide-welcome { flex: 1; display: flex; flex-direction: column; align-items: center; max-height: none; padding: 16px; gap: 16px; }
.welcome-title { flex: none; margin-top: auto; text-align: center; font-family: Georgia, 'Noto Serif SC', serif; font-size: clamp(22px, 2.4vw, 34px); font-weight: 550; }
.chat-guide { width: fit-content; max-width: 100%; font-size: 13px; color: var(--mg-ink-700); }
.guide-welcome .chat-guide { flex: none; margin-bottom: auto; }
.chat-guide[open] { width: min(100%, 32rem); border: 1px solid var(--mg-line-warm); border-radius: 10px; background: var(--mg-gold-50); }
.chat-guide summary { display: flex; align-items: center; gap: 8px; min-height: 36px; padding: 4px 8px; cursor: pointer; font-weight: 600; color: var(--mg-ink-900); list-style: none; }
.chat-guide summary::-webkit-details-marker { display: none; }
.chat-guide summary:focus-visible { outline: 2px solid var(--mg-gold-500); outline-offset: -2px; border-radius: 8px; }
.chat-guide summary svg { color: var(--mg-gold-700); }
.chat-guide[open] summary { padding: 8px 14px; border-bottom: 1px solid var(--mg-line-warm); }
.guide-chevron { margin-left: auto; }
.chat-guide[open] .guide-chevron { transform: rotate(180deg); }
.chat-guide dl { display: grid; grid-template-columns: auto 1fr; align-items: baseline; gap: 9px 14px; padding: 12px 14px; line-height: 1.6; }
.chat-guide dt { color: var(--mg-ink-900); font-weight: 600; }
.manage-link { margin-top: 12px; text-decoration: underline; text-underline-offset: 3px; }
.chat-turn { max-width: 90%; align-self: flex-start; }
.chat-turn strong { display: block; font-size: 11px; color: var(--mg-muted); margin-bottom: 5px; }
.chat-turn p { font-size: 14px; line-height: 1.7; overflow-wrap: anywhere; white-space: pre-wrap; }
.user-turn { align-self: flex-end; padding: 10px 14px; background: var(--mg-champagne-50); border-radius: 12px 12px 2px 12px; }
.chat-composer {
  display: grid; gap: 6px; align-self: center; width: min(48rem, calc(100% - 32px)); margin: 0 0 16px; padding: 12px;
  background: var(--mg-surface); border: 1px solid var(--mg-line-warm); border-radius: 22px; box-shadow: 0 3px 14px rgb(33 26 12 / 5%);
}
.composer-row { gap: 8px; min-width: 0; }
.chat-attach { flex: none; display: inline-flex; align-items: center; gap: 5px; min-height: 38px; padding: 4px 8px; border-radius: 18px; color: var(--mg-ink-700); font-size: 13px; }
.model-picker { position: relative; display: flex; align-items: center; gap: 5px; min-width: 0; max-width: min(55%, 22rem); min-height: 38px; margin-left: auto; padding: 6px 8px; border-radius: 18px; }
.model-value { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; font-weight: 600; }
.model-picker svg { flex: none; }
.model-picker select { position: absolute; inset: 0; width: 100%; height: 100%; opacity: 0; color: var(--mg-ink-900); background: var(--mg-surface); cursor: pointer; }
.model-picker:has(select:disabled) { color: var(--mg-muted); }
.webchat .model-picker select:disabled { opacity: 0; }
.model-picker:has(select:focus-visible) { outline: 2px solid var(--mg-gold-500); }
.chat-composer .file-list { max-height: 96px; overflow-y: auto; }
.chat-turn .file-list { margin-top: 5px; }
.file-hint { padding: 0 6px; color: var(--mg-muted); font-size: 11px; overflow-wrap: anywhere; }
.file-status { margin: 2px 4px; color: var(--mg-muted); font-size: 12px; }
.original-image { display: block; max-width: 100%; max-height: 65dvh; margin: 0 auto; object-fit: contain; }
.chat-composer:focus-within { border-color: var(--mg-gold-500); }
.chat-composer textarea { width: 100%; min-width: 0; max-height: 140px; padding: 4px 6px; resize: vertical; font-size: 14px; color: inherit; background: transparent; }
.chat-composer textarea:focus-visible { outline: 2px solid var(--mg-gold-500); border-radius: 3px; }
.chat-send { flex: none; display: grid; place-items: center; width: 40px; height: 40px; border-radius: 50%; color: var(--mg-surface); background: var(--mg-ink-900); }
.webchat :is(button, select):focus-visible, .key-field select:focus-visible { outline: 2px solid var(--mg-gold-500, #c9b477); outline-offset: -2px; }
.chat-footer { padding: 10px calc(7rem + env(safe-area-inset-right, 0px)) 0 0; color: var(--mg-muted); font-size: 12px; }
.chat-error button { text-decoration: underline; text-underline-offset: 3px; }
.chat-error { justify-content: space-between; margin-bottom: 12px; padding: 10px 12px; border: 1px solid var(--mg-line-warm); border-radius: 6px; color: var(--mg-ink-900); background: var(--mg-champagne-50); font-size: 13px; }
.chat-error button { flex: none; }
.length-error { margin: -8px 18px 10px; color: var(--mg-danger); font-size: 12px; }
@media (max-width: 760px) {
  .chat-workspace { grid-template: 'history' auto 'conversation' minmax(0, 1fr) 'settings' auto / minmax(0, 1fr); height: clamp(440px, calc(100dvh - 165px), 900px); }
  .chat-workspace.workspace-welcome { height: auto; min-height: clamp(440px, calc(100dvh - 165px), 900px); grid-template-rows: auto 1fr auto; }
  .workspace-welcome .guide-welcome { flex: 1 0 auto; overflow: visible; }
  .chat-history { flex: none; flex-flow: row wrap; align-items: center; justify-content: space-between; gap: 8px; padding: 10px 12px; border: 0; border-bottom: 1px solid var(--mg-line-warm); }
  .desktop-history-title { display: none; }
  .mobile-history-toggle { display: inline-flex; font-size: 14px; }
  .settings-slot { border-right: 0; padding: 3px 8px; }
  .settings-trigger { width: fit-content; max-width: 100%; }
  .settings-trigger small { margin-left: 4px; }
  .history-list { display: none; width: 100%; max-height: 130px; }
  .history-list.mobile-open { display: block; }
  .conversation { flex: 1; }
  .conversation-header { padding: 6px 12px; min-height: 42px; }
  .chat-timeline { padding: 12px; }
  .chat-composer { width: calc(100% - 16px); margin-bottom: 8px; padding: 8px; border-radius: 16px; }
  .composer-row { gap: 4px; }
  .model-picker { max-width: 52%; }
  .chat-footer { padding-bottom: 50px; }
}
@media (max-width: 360px) {
  .chat-attach { width: 40px; justify-content: center; }
  .chat-attach span { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); }
  .model-picker { max-width: calc(100% - 88px); }
}
</style>
