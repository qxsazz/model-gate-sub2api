import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { extractApiErrorCode } from '@/utils/apiError'
import { webChatAPI, type ChatKey, type ChatModel, type ChatSession, type ChatSessionSummary } from '@/api/webChat'
import { readChatFile, validateChatFiles, type DraftChatFile } from '@/utils/webChatAttachments'

type Operation = 'keys' | 'options' | 'session' | 'send' | 'delete'
const maxMessageBytes = 32 * 1024
const encoder = new TextEncoder()

export function useWebChat() {
  const keys = ref<ChatKey[]>([])
  const models = ref<ChatModel[]>([])
  const sessions = ref<ChatSessionSummary[]>([])
  const keyId = ref<number | null>(null)
  const model = ref('')
  const selectedId = ref('')
  const session = ref<ChatSession | null>(null)
  const attachments = ref<DraftChatFile[]>([])
  const pendingFiles = ref<DraftChatFile[]>([])
  const readingFiles = ref(false)
  const fileError = ref('')
  let reading: AbortController | undefined
  const draft = ref('')
  const pending = ref('')
  const errorKey = ref('')
  const operation = ref<Operation | ''>('')
  const keysLoaded = ref(false)
  const optionsLoaded = ref(false)
  const needsReload = ref(false)
  const mutating = computed(() => operation.value === 'send' || operation.value === 'delete')
  const tooLong = computed(() => encoder.encode(draft.value.trim()).byteLength > maxMessageBytes)
  const chatReady = computed(() => Boolean(keyId.value && optionsLoaded.value && models.value.some(item => item.id === model.value)))
  const canSend = computed(() => Boolean(
    chatReady.value && !operation.value && !needsReload.value &&
    (!selectedId.value || selectedId.value === session.value?.id) &&
    (draft.value.trim() || attachments.value.length) && !tooLong.value && !readingFiles.value
  ))
  let request: AbortController | undefined
  let expirationTimer: ReturnType<typeof setTimeout> | undefined

  function clearFiles() {
    reading?.abort()
    readingFiles.value = false
    attachments.value = []
    pendingFiles.value = []
    fileError.value = ''
  }

  async function addFiles(files: File[]) {
    if (!files.length || !chatReady.value || needsReload.value || operation.value || readingFiles.value) return
    fileError.value = validateChatFiles(attachments.value, files)
    if (fileError.value) return
    const controller = new AbortController()
    reading = controller
    readingFiles.value = true
    try {
      const items = await Promise.all(files.map(file => readChatFile(file, controller.signal)))
      if (!controller.signal.aborted) attachments.value = [...attachments.value, ...items]
    } catch (error) {
      if (!controller.signal.aborted) fileError.value = error instanceof Error && error.message === 'type' ? 'type' : 'read'
    } finally {
      if (!controller.signal.aborted) readingFiles.value = false
    }
  }

  function removeFile(index: number) {
    if (operation.value || readingFiles.value) return
    attachments.value.splice(index, 1)
    fileError.value = ''
  }

  function resetChat() {
    clearFiles()
    selectedId.value = ''
    session.value = null
    pending.value = ''
    draft.value = ''
    needsReload.value = false
  }

  function resetKey(id: number | null) {
    keyId.value = id
    models.value = []
    sessions.value = []
    model.value = ''
    optionsLoaded.value = false
    resetChat()
  }

  function handleError(error: unknown, action: Operation) {
    const e = error as { reason?: string; status?: number } | null
    switch (extractApiErrorCode(error)) {
      case 'WEBCHAT_KEY_UNAVAILABLE':
        keys.value = keys.value.filter(k => k.id !== keyId.value)
        resetKey(null)
        errorKey.value = 'key'
        return
      case 'WEBCHAT_SESSION_NOT_FOUND':
        clearFiles()
        sessions.value = sessions.value.filter(s => s.id !== selectedId.value)
        session.value = null
        needsReload.value = true
        errorKey.value = 'expired'
        return
      case 'WEBCHAT_SESSION_CONFLICT':
        needsReload.value = true
        errorKey.value = 'conflict'
        return
      case 'WEBCHAT_MODEL_ERROR':
        errorKey.value = 'model'
        return
      case 'WEBCHAT_MODEL_UNAVAILABLE':
        models.value = models.value.filter(item => item.id !== model.value)
        model.value = ''
        errorKey.value = 'modelUnavailable'
        return
      case 'WEBCHAT_UPSTREAM_ERROR':
        errorKey.value = 'service'
        return
      case 'WEBCHAT_TIMEOUT':
        errorKey.value = 'timeout'
        return
      case 'WEBCHAT_ATTACHMENT_INVALID':
        errorKey.value = 'attachment'
        return
      case 'WEBCHAT_PAYLOAD_TOO_LARGE':
        errorKey.value = 'payload'
        return
      case 'WEBCHAT_ATTACHMENT_BUSY':
        errorKey.value = 'processing'
        return
      case 'WEBCHAT_HISTORY_LIMIT':
        needsReload.value = true
        errorKey.value = 'limit'
        return
    }
    if (e?.status === 402) errorKey.value = 'quota'
    else if (e?.status === 403) errorKey.value = 'access'
    else if (e?.status === 429) errorKey.value = 'rate'
    else errorKey.value = action === 'send' ? 'send' : 'failed'
  }

  async function run(action: Operation, work: (signal: AbortSignal) => Promise<void>) {
    request?.abort()
    const controller = new AbortController()
    request = controller
    operation.value = action
    errorKey.value = ''
    try {
      await work(controller.signal)
    } catch (error) {
      if (!controller.signal.aborted) handleError(error, action)
    } finally {
      if (!controller.signal.aborted) operation.value = ''
    }
  }

  async function loadOptions(id: number, signal: AbortSignal) {
    const [available, history] = await Promise.all([webChatAPI.models(id, signal), webChatAPI.sessions(id, signal)])
    if (signal.aborted) return
    models.value = available
    model.value = available.some(item => item.id === model.value) ? model.value : (available[0]?.id ?? '')
    sessions.value = history
    optionsLoaded.value = true
  }

  async function refresh() {
    if (mutating.value) return
    await run('keys', async signal => {
      const available = await webChatAPI.keys(signal)
      if (signal.aborted) return
      const id = available.find(key => key.id === keyId.value)?.id ??
        (!keysLoaded.value && available.length === 1 ? available[0]?.id : undefined) ?? null
      keysLoaded.value = true
      keys.value = available
      if (keyId.value !== id) resetKey(id)
      if (!id) return
      optionsLoaded.value = false
      await loadOptions(id, signal)
      if (signal.aborted) return
      if (selectedId.value) {
        if (!sessions.value.some(item => item.id === selectedId.value)) {
          clearFiles()
          session.value = null
          needsReload.value = true
          errorKey.value = 'expired'
          return
        }
        needsReload.value = true
        const detail = await webChatAPI.session(id, selectedId.value, signal)
        if (!signal.aborted) session.value = detail
      }
      if (!signal.aborted) needsReload.value = false
    })
  }

  function chooseKey(id: number) {
    if (mutating.value || !keys.value.some(k => k.id === id)) return
    resetKey(id)
    return run('options', signal => loadOptions(id, signal))
  }

  function newChat() {
    if (operation.value) return
    resetChat()
    errorKey.value = ''
  }

  function openSession(id: string) {
    const key = keyId.value
    if (!key || !optionsLoaded.value || mutating.value) return
    resetChat()
    selectedId.value = id
    return run('session', async signal => {
      const detail = await webChatAPI.session(key, id, signal)
      if (!signal.aborted) {
        session.value = detail
        if (detail.model) model.value = models.value.some(item => item.id === detail.model) ? detail.model : ''
      }
    })
  }

  function send() {
    const key = keyId.value
    if (!key || !canSend.value) return
    const content = draft.value.trim()
    const id = selectedId.value
    const files = attachments.value
    const payload = { key_id: key, model: model.value, content, ...(id && { session_id: id }),
      ...(files.length > 0 && { attachments: files.map(({ name, mime_type, data }) => ({ name, mime_type, data })) }) }
    pendingFiles.value = files
    attachments.value = []
    fileError.value = ''
    pending.value = content
    draft.value = ''
    return run('send', async signal => {
      try {
        const result = await webChatAPI.send(payload, signal)
        if (signal.aborted) return
        session.value = result
        needsReload.value = false
        errorKey.value = ''
        selectedId.value = result.id
        const { id, title, model, updated_at, expires_at } = result
        sessions.value = [{ id, title, model, updated_at, expires_at }, ...sessions.value.filter(s => s.id !== id)]
      } catch (error) {
        if (!signal.aborted) {
          draft.value = content
          attachments.value = files
        }
        throw error
      } finally {
        if (!signal.aborted) {
          pending.value = ''
          pendingFiles.value = []
        }
      }
    })
  }

  function remove() {
    const key = keyId.value, id = session.value?.id
    if (!key || !id || operation.value) return
    return run('delete', async signal => {
      await webChatAPI.remove(key, id, signal)
      if (signal.aborted) return
      sessions.value = sessions.value.filter(s => s.id !== id)
      resetChat()
    })
  }

  function expireHistory() {
    clearTimeout(expirationTimer)
    const now = Date.now()
    const remaining = sessions.value.filter(item => Date.parse(item.expires_at) > now)
    if (remaining.length !== sessions.value.length) {
      sessions.value = remaining
      if (session.value && !remaining.some(item => item.id === session.value?.id)) {
        clearFiles()
        session.value = null
        needsReload.value = true
        errorKey.value = 'expired'
      }
    }
    if (remaining.length) {
      const next = Math.min(...remaining.map(item => Date.parse(item.expires_at)))
      expirationTimer = setTimeout(expireHistory, Math.min(next - now, 2_147_483_647))
    }
  }
  watch(sessions, expireHistory)
  onMounted(refresh)
  onBeforeUnmount(() => { request?.abort(); clearFiles(); clearTimeout(expirationTimer) })
  return { attachments, pendingFiles, readingFiles, fileError, addFiles, removeFile, keys, models, sessions, keyId, model, selectedId, session, draft, pending, errorKey,
    operation, keysLoaded, optionsLoaded, mutating, tooLong, chatReady, canSend, needsReload,
    refresh, chooseKey, newChat, openSession, send, remove }
}
