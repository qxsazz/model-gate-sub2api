<template>
  <AppLayout>
    <section class="canvas-workspace">
      <iframe ref="frame" src="/canvas-app/image" title="MODEL-GATE 无限画布" class="canvas-frame" allow="clipboard-write; fullscreen" />
      <div v-if="saveError" role="alert" class="canvas-save-error">{{ status }}</div>
      <div v-if="showCreate" class="canvas-dialog-backdrop" @click.self="showCreate = false">
        <section class="canvas-dialog" role="dialog" aria-modal="true" aria-label="创建画布Key">
          <h2>创建{{ purpose === 'video' ? '生视频' : '生图' }} Key</h2>
          <p>请选择支持对应模型的分组，创建后画布会自动刷新。</p>
          <label>名称<input v-model="keyName" class="input" /></label>
          <label>分组<select v-model="groupId" class="input"><option :value="null" disabled>请选择分组</option><option v-for="group in eligibleGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
          <p v-if="!eligibleGroups.length">暂无可用分组，请先获取对应分组权限或订阅。</p>
          <p v-if="createError" class="text-red-600" role="alert">{{ createError }}</p>
          <div class="flex justify-end gap-3"><button class="btn btn-secondary" @click="showCreate = false">取消</button><button class="btn btn-primary" :disabled="creating || !groupId" @click="createKey">{{ creating ? '创建中…' : '创建 Key' }}</button></div>
        </section>
      </div>
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { apiClient } from '@/api/client'
import { keysAPI } from '@/api/keys'
import { useAuthStore } from '@/stores/auth'
import type { Group } from '@/types'

const auth = useAuthStore()
const frame = ref<HTMLIFrameElement>()
const status = ref('正在连接画布…')
const saveError = ref(false)
const showCreate = ref(false)
const purpose = ref('image')
const groups = ref<Group[]>([])
const groupId = ref<number | null>(null)
const keyName = ref('无限画布')
const creating = ref(false)
const createError = ref('')
const eligibleGroups = computed(() => groups.value.filter((group) => group.status === 'active' && group.allow_image_generation && (purpose.value === 'video' ? ['grok', 'composite'].includes(group.platform) : ['openai', 'grok', 'gemini', 'antigravity', 'composite'].includes(group.platform))))
const theme = () => document.documentElement.classList.contains('dark') ? 'dark' : 'light'
const pending = new Set<AbortController>()
let observer: MutationObserver | undefined
let active = true
async function createKey() {
  if (!groupId.value) return
  creating.value = true; createError.value = ''
  try {
    await keysAPI.create(keyName.value, groupId.value)
    showCreate.value = false
    frame.value?.contentWindow?.postMessage({ type: 'mg-canvas-refresh' }, location.origin)
  } catch (error) { createError.value = error instanceof Error ? error.message : '创建失败，请重试' }
  finally { creating.value = false }
}
async function receive(event: MessageEvent) {
  if (!active || event.origin !== location.origin || event.source !== frame.value?.contentWindow) return
  const msg = event.data
  if (msg?.type === 'mg-canvas-action') {
    if (msg.action === 'saved') { status.value = '已保存到云端'; saveError.value = false }
    if (msg.action === 'save-error') { status.value = `云端保存失败：${msg.value}`; saveError.value = true }
    if (msg.action === 'media-error') status.value = `素材已过期、被屏蔽或暂时无法读取：${msg.value}`
    if (msg.action === 'create-key') {
      purpose.value = msg.value === 'video' ? 'video' : 'image'; keyName.value = `无限画布${purpose.value === 'video' ? '生视频' : '生图'}`
      createError.value = ''; groupId.value = null; showCreate.value = true
      try { const { data } = await apiClient.get<Group[]>('/groups/available'); groups.value = data; groupId.value = eligibleGroups.value[0]?.id ?? null }
      catch { createError.value = '分组加载失败，请重新打开重试' }
    }
    return
  }
  if (msg?.type !== 'mg-canvas-request' || typeof msg.id !== 'string') return
  const source = event.source as Window
  const reply = (value?: unknown, error?: string) => { if (active) source.postMessage({ type: 'mg-canvas-response', id: msg.id, value, error }, location.origin) }
  if (msg.path === 'context') { reply({ userId: auth.user?.id, theme: theme(), keys: [] }); status.value = '正在加载云端项目'; return }
  const method = String(msg.method || 'GET').toUpperCase()
  // No administrative or arbitrary host API is reachable through the child bridge.
  if (typeof msg.path !== 'string' || !/^\/canvas\/(capabilities|state\/[^/]+|storage\/[^/]+|assets\/[a-f0-9-]+\/content|gateway\/\d+\/(?:models|responses|audio\/speech|gemini\/models(?:\/[A-Za-z0-9][A-Za-z0-9._-]*:(?:generateContent|streamGenerateContent))?|images\/(?:generations|edits)|videos\/(?:generations|[^/?]+(?:\/content)?)))$/.test(msg.path) || !['GET', 'PUT', 'POST'].includes(method)) { reply(undefined, '不允许的画布请求'); return }
  const controller = new AbortController(); pending.add(controller)
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
      headers: multipart ? { 'Content-Type': undefined } : binaryBody ? { 'Content-Type': msg.body.type || 'application/octet-stream' } : undefined,
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
.canvas-dialog-backdrop { position: fixed; inset: 0; z-index: 70; display: grid; place-items: center; background: #0008; padding: 20px; }
.canvas-dialog { width: 100%; max-width: 480px; background: var(--mg-surface, white); color: var(--mg-ink-900, #24252a); border-radius: 12px; padding: 24px; }
.canvas-dialog h2 { font-size: 18px; font-weight: 600; } .canvas-dialog label { display: grid; gap: 6px; margin: 16px 0; }
</style>
