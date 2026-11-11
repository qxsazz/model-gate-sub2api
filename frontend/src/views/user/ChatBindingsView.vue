<template>
  <AppLayout>
    <div class="chat-bindings mx-auto max-w-[950px] space-y-6">
      <header class="flex items-center justify-between gap-4">
        <div><p class="brand-accent text-xs tracking-widest">MODEL-GATE</p><h1 class="mt-2 text-2xl font-semibold">群聊绑定</h1></div>
        <button class="btn btn-secondary" :disabled="busy || loading" @click="load">刷新</button>
      </header>
      <p v-if="error" class="card p-5 text-red-600 dark:text-red-400" role="alert">{{ error }}</p>
      <p v-if="loading" role="status">正在读取绑定信息…</p>
      <section v-else-if="challenge" class="card space-y-5 p-6">
        <h2 class="text-lg font-semibold">确认绑定当前账号</h2>
        <p class="break-words">当前登录：<strong data-account>{{ auth.user?.username || auth.user?.email }}</strong><span v-if="auth.user?.username" class="ml-2">{{ auth.user?.email }}</span></p>
        <dl class="identity-grid">
          <dt>来源</dt><dd>{{ platformName(challenge.identity.platform) }}</dd>
          <dt>Bot 实例</dt><dd>{{ challenge.identity.bot_id }}</dd>
          <dt>群标识</dt><dd>{{ challenge.identity.group_id }}</dd>
          <dt>你的群身份</dt><dd>{{ challenge.identity.sender_id }}</dd>
        </dl>
        <p v-if="challenge.state === 'completed'" role="status">绑定已完成，可以回到原群发送 /签到。</p>
        <p v-else-if="expired" role="status">此绑定请求已失效，请回到原群重新发送 /绑定。</p>
        <template v-else>
          <p class="text-sm text-gray-600 dark:text-gray-400">仅在你刚刚向上面的 Bot 发送了 /绑定 时继续。确认码只用于当前群和当前身份，请勿转发给其他人。</p>
          <template v-if="!authorization">
            <label class="flex items-start gap-3"><input v-model="consent" type="checkbox" class="mt-1" data-consent />我已核对当前账号和群聊来源，确认绑定。</label>
            <button class="btn btn-primary" data-authorize :disabled="busy || !consent || !auth.user" @click="authorize">{{ busy ? '正在生成…' : '获取一次性确认码' }}</button>
          </template>
          <div v-else class="confirmation-code space-y-3 rounded-xl border p-5">
            <p>回到发起绑定的同一个群，使用同一身份发送：</p>
            <code class="block break-all text-lg" data-code>/绑定 {{ authorization.code }}</code>
            <button class="btn btn-secondary" @click="copyCode">{{ copied ? '已复制' : '复制绑定命令' }}</button>
          </div>
          <p class="text-sm text-gray-500">剩余 {{ remaining }} 秒 · 确认码仅可使用一次</p>
        </template>
      </section>
      <section class="card space-y-5 p-6">
        <h2 class="text-lg font-semibold">已绑定的群聊</h2>
        <p class="text-sm text-gray-600 dark:text-gray-400">QQ、微信和网站共享每日签到记录。签到资格以网站当前规则为准，现金奖励仍受活动范围和预算限制。</p>
        <p v-if="!loading && bindings.length === 0">尚未绑定。请在已接入的群里向 Bot 发送 /绑定，再打开返回的链接。</p>
        <article v-for="binding in bindings" :key="binding.id" class="binding-row rounded-xl border p-4">
          <div class="min-w-0 space-y-2"><strong>{{ platformName(binding.identity.platform) }}</strong><p class="break-all text-sm">群：{{ binding.identity.group_id }}</p><p class="break-all text-sm">Bot：{{ binding.identity.bot_id }}</p><p class="break-all text-sm">身份：{{ binding.identity.sender_id }}</p></div>
          <button class="btn btn-secondary shrink-0" :disabled="busy" :data-revoke="binding.id" @click="revoking = binding">解除绑定</button>
        </article>
        <router-link to="/profile" class="brand-accent">返回个人资料</router-link>
      </section>
      <ConfirmDialog :show="!!revoking" title="解除群聊绑定" message="解除后，该群身份发送的新签到消息将失效；已有签到记录会保留。" confirm-text="解除绑定" danger @confirm="revoke" @cancel="revoking = null" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAuthStore } from '@/stores/auth'
import { authorizeChatBinding, getChatBindingChallenge, listChatBindings, revokeChatBinding, type ChatBinding, type ChatBindingAuthorization, type ChatBindingChallenge } from '@/api/chatBindings'

const route = useRoute()
const auth = useAuthStore()
const challengeID = computed(() => typeof route.params.challengeId === 'string' ? route.params.challengeId : '')
const bindings = ref<ChatBinding[]>([])
const challenge = ref<ChatBindingChallenge | null>(null)
const authorization = ref<ChatBindingAuthorization | null>(null)
const revoking = ref<ChatBinding | null>(null)
const loading = ref(true)
const busy = ref(false)
const consent = ref(false)
const copied = ref(false)
const error = ref('')
const now = ref(Date.now())
let sequence = 0
let timer: ReturnType<typeof setInterval> | undefined
const remaining = computed(() => Math.max(0, Math.ceil((Date.parse(challenge.value?.expires_at || '') - now.value) / 1000)) || 0)
const expired = computed(() => challenge.value?.state === 'expired' || remaining.value === 0)
const platformName = (platform: string) => platform === 'qq_official' ? 'QQ 官方群机器人' : '微信群机器人'
function message(e: unknown, fallback: string) { return e && typeof e === 'object' && 'message' in e && typeof e.message === 'string' ? e.message : fallback }
async function load() {
  const request = ++sequence
  loading.value = true
  error.value = ''
  consent.value = false
  authorization.value = null
  copied.value = false
  try {
    await auth.refreshUser().catch(() => { throw new Error('无法确认当前登录账号，请重新登录。') })
    if (request !== sequence) return
    const [items, preview] = await Promise.all([listChatBindings(), challengeID.value ? getChatBindingChallenge(challengeID.value) : Promise.resolve(null)])
    if (request !== sequence) return
    if (preview && preview.account_id !== auth.user?.id) throw new Error('登录账号已变化，请刷新并重新确认。')
    bindings.value = items
    challenge.value = preview
    now.value = Date.now()
    if (preview?.state === 'completed' || preview?.state === 'expired') authorization.value = null
  } catch (e) { if (request === sequence) { challenge.value = null; error.value = message(e, '读取绑定信息失败，请重试。') } }
  finally { if (request === sequence) loading.value = false }
}
async function authorize() {
  if (!consent.value || busy.value || expired.value || !auth.user || !challenge.value?.account_confirmation) return
  const request = sequence
  busy.value = true
  error.value = ''
  try { const result = await authorizeChatBinding(challengeID.value, challenge.value.account_confirmation); if (request === sequence) authorization.value = result }
  catch (e) { if (request === sequence) error.value = message(e, '生成确认码失败，请重试。') }
  finally { busy.value = false }
}
async function copyCode() {
  if (!authorization.value || expired.value) return
  try { await navigator.clipboard.writeText(`/绑定 ${authorization.value.code}`); copied.value = true }
  catch { error.value = '无法复制，请手动复制上面的绑定命令。' }
}
async function revoke() {
  if (!revoking.value || busy.value) return
  busy.value = true
  error.value = ''
  try { await revokeChatBinding(revoking.value.id); revoking.value = null; await load() }
  catch (e) { error.value = message(e, '解除绑定失败，请重试。') }
  finally { busy.value = false }
}
watch([challengeID, () => auth.user?.id], () => { authorization.value = null; consent.value = false; copied.value = false; bindings.value = []; revoking.value = null; void load() })
onMounted(() => {
  timer = setInterval(() => { now.value = Date.now(); if (expired.value) authorization.value = null }, 1000)
  void load()
})
onUnmounted(() => { sequence++; clearInterval(timer); authorization.value = null })
</script>

<style scoped>
.brand-accent { color: var(--mg-gold-700, #9c8344); }
.confirmation-code { border-color: var(--mg-line-warm, #e7e1d4); background: var(--mg-gold-50, #fcfaf4); }
.binding-row { border-color: var(--mg-line-warm, #e7e1d4); }
input[type="checkbox"] { accent-color: var(--mg-gold-700, #9c8344); }
.identity-grid { display: grid; grid-template-columns: max-content minmax(0, 1fr); gap: .75rem 1rem; }
.identity-grid dt { opacity: .65; }
.identity-grid dd { overflow-wrap: anywhere; }
.binding-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
@media (max-width: 520px) { .binding-row { align-items: flex-start; flex-direction: column; } }
</style>
