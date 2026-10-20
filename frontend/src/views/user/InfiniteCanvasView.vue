<template>
  <AppLayout>
    <section class="canvas-workspace">
      <iframe ref="frame" src="/canvas-app/image" title="MODEL-GATE 无限画布" class="canvas-frame" allow="clipboard-write; fullscreen" />
      <div v-if="saveError" role="alert" class="canvas-save-error">{{ status }}</div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { apiClient } from '@/api/client'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const auth = useAuthStore()
const frame = ref<HTMLIFrameElement>()
const status = ref('正在连接画布…')
const saveError = ref(false)
const theme = () => document.documentElement.classList.contains('dark') ? 'dark' : 'light'
const pending = new Set<AbortController>()
let observer: MutationObserver | undefined
let active = true
async function receive(event: MessageEvent) {
  if (!active || event.origin !== location.origin || event.source !== frame.value?.contentWindow) return
  const msg = event.data
  if (msg?.type === 'mg-canvas-action') {
    if (msg.action === 'saved') { status.value = '已保存到云端'; saveError.value = false }
    if (msg.action === 'save-error') { status.value = `云端保存失败：${msg.value}`; saveError.value = true }
    if (msg.action === 'media-error') status.value = `素材已过期、被屏蔽或暂时无法读取：${msg.value}`
    if (msg.action === 'create-key') {
      const purpose = ['image', 'video', 'text'].includes(msg.value) ? msg.value : 'image'
      void router.push({ path: '/keys', query: { create: purpose, return_to: 'canvas' } })
    }
    return
  }
  if (msg?.type !== 'mg-canvas-request' || typeof msg.id !== 'string') return
  const source = event.source as Window
  const reply = (value?: unknown, error?: string) => { if (active) source.postMessage({ type: 'mg-canvas-response', id: msg.id, value, error }, location.origin) }
  if (msg.path === 'context') { reply({ userId: auth.user?.id, theme: theme(), keys: [] }); status.value = '正在加载云端项目'; return }
  const method = String(msg.method || 'GET').toUpperCase()
  const taskPath = typeof msg.path === 'string' && (/^\/canvas\/tasks(?:\/[a-f0-9-]+)?$/.test(msg.path) && method === 'GET' || /^\/canvas\/tasks\/[a-f0-9-]+\/cancel$/.test(msg.path) && method === 'POST' || /^\/canvas\/tasks\/[a-f0-9-]+\/\d+\/(?:images\/(?:generations|edits)|gemini\/models\/[A-Za-z0-9][A-Za-z0-9._-]*:generateContent)$/.test(msg.path) && method === 'PUT')
  // No administrative or arbitrary host API is reachable through the child bridge.
  if (!taskPath && (typeof msg.path !== 'string' || !/^\/canvas\/(capabilities|state\/[^/]+|storage\/[^/]+|assets\/[a-f0-9-]+\/content|gateway\/\d+\/(?:models|responses|audio\/speech|gemini\/models(?:\/[A-Za-z0-9][A-Za-z0-9._-]*:(?:generateContent|streamGenerateContent))?|images\/(?:generations|edits)|videos\/(?:generations|[^/?]+(?:\/content)?)))$/.test(msg.path) || !['GET', 'PUT', 'POST'].includes(method))) { reply(undefined, '不允许的画布请求'); return }
  const controller = new AbortController(); if (!(taskPath && method === 'PUT')) pending.add(controller)
  try {
    const gateway = msg.path.startsWith('/canvas/gateway/')
    const binaryBody = msg.body instanceof Blob
    let body = msg.body
    let multipart = false
    if (body && typeof body === 'object' && Array.isArray(body.form)) {
      const form = new FormData()
      for (const [key, value] of body.form) form.append(key, value)
      body = form; multipart = true
    }
    const result = await apiClient.request({ url: msg.path, method, data: body, signal: controller.signal,
      responseType: msg.binary ? 'blob' : 'json',
      // Generation uses the existing gateway timeout rather than the panel's 30 second timeout.
      timeout: gateway ? 0 : undefined,
      headers: { ...(multipart ? { 'Content-Type': undefined } : binaryBody ? { 'Content-Type': msg.body.type || 'application/octet-stream' } : {}), ...(typeof msg.taskMeta === 'string' ? { 'X-Canvas-Task-Meta': msg.taskMeta } : {}) },
      validateStatus: gateway ? () => true : undefined,
    })
    if (gateway) reply({ data: result.data, status: result.status, headers: { 'content-type': result.headers['content-type'] || '' } })
    else reply(result.data)
  } catch (error) { reply(undefined, error instanceof Error ? error.message : '请求失败，请重试') }
  finally { pending.delete(controller) }
}
watch(() => auth.user?.id, () => { active = false; pending.forEach((c) => c.abort()); if (frame.value) frame.value.src = 'about:blank' })
onMounted(() => {
  window.addEventListener('message', receive)
  observer = new MutationObserver(() => frame.value?.contentWindow?.postMessage({ type: 'mg-canvas-theme', theme: theme() }, location.origin))
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => { active = false; pending.forEach((c) => c.abort()); observer?.disconnect(); window.removeEventListener('message', receive) })
</script>

<style scoped>
.canvas-workspace { overflow: hidden; border: 1px solid var(--mg-line-warm, #e7e1d4); border-radius: 12px; background: var(--mg-surface, #fff); }
.canvas-save-error { padding: 8px 16px; color: #b42318; background: #fff4ed; font-size: 12px; }
.canvas-frame { display: block; width: 100%; height: calc(100dvh - 145px); min-height: 620px; border: 0; }
</style>
