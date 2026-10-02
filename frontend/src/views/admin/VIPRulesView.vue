<template>
  <AppLayout>
    <div class="vip-admin space-y-6" :aria-busy="busy">
      <div class="vip-toolbar">
        <div role="tablist" aria-label="VIP 权益设置" class="vip-tabs">
          <button v-for="tab in tabs" :id="`vip-tab-${tab.id}`" :key="tab.id" type="button" role="tab"
            :aria-selected="activeTab === tab.id" :aria-controls="`vip-panel-${tab.id}`"
            :tabindex="activeTab === tab.id ? 0 : -1" :data-tab="tab.id" :class="{ active: activeTab === tab.id }"
            @click="activeTab = tab.id" @keydown="navigateTabs($event, tab.id)">{{ tab.label }}</button>
        </div>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-secondary" title="刷新数据" aria-label="刷新数据" :disabled="busy" @click="requestRefresh"><Icon name="refresh" size="md" /></button>
          <button v-if="activeTab !== 'users'" type="button" data-save-rules class="btn btn-primary" :disabled="busy || !rules" @click="save"><Icon name="checkCircle" size="sm" />保存规则</button>
          <router-link v-else to="/admin/users" class="btn btn-secondary">用户管理</router-link>
        </div>
      </div>
      <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
      <p v-if="!rules" role="status" class="py-12 text-center text-sm text-gray-500">{{ busy ? '正在加载权益配置…' : '权益配置加载失败，请重试' }}</p>
      <template v-else>
        <div id="vip-panel-rules" v-show="activeTab === 'rules'" role="tabpanel" aria-labelledby="vip-tab-rules" class="space-y-6">
          <section class="vip-section">
            <h2>功能与计量</h2>
            <div class="vip-fields">
              <label class="vip-field">自动权益<span class="flex h-10 items-center gap-2"><input v-model="rules.enabled" type="checkbox" :disabled="busy" />启用自动权益</span></label>
              <label class="vip-field">专属组门槛<input v-model.number="rules.access_threshold" type="number" min="100" class="input" :disabled="busy" /></label>
              <label class="vip-field">计量单位<input :value="rules.currency" readonly class="input" /></label>
            </div>
            <div class="vip-fields mt-4">
              <label v-for="(_, currency) in rules.exchange_rates" :key="currency" class="vip-field">{{ currency }} → USD<input v-model.number="rules.exchange_rates[currency]" type="number" min="0.000001" step="0.000001" class="input" :disabled="busy" /></label>
              <div class="vip-field"><label for="vip-new-currency">新增币种</label><div class="flex gap-2"><input id="vip-new-currency" v-model="newCurrency" maxlength="3" class="input min-w-0" :disabled="busy" /><button type="button" class="btn btn-secondary" :disabled="busy" @click="addCurrency">添加</button></div></div>
            </div>
            <p class="vip-note">换算值须按实际支付口径确认。未配置的支付币种在启用 VIP 后不能创建充值订单；老订单不自动回填。</p>
          </section>
          <section class="vip-section">
            <h2>等级阶梯</h2>
            <div class="vip-table-wrap"><table class="vip-table">
              <thead><tr><th>等级</th><th>累计充值</th><th>并发目标</th><th>RPM 目标</th><th>邀请返利 %</th></tr></thead>
              <tbody><tr v-for="tier in rules.tiers" :key="tier.level">
                <td class="whitespace-nowrap font-medium">VIP {{ tier.level }}</td>
                <td><input v-model.number="tier.threshold" :data-threshold="tier.level" :aria-label="`VIP ${tier.level} 累计充值门槛`" type="number" :readonly="tier.level === 1" min="100" class="input" :disabled="busy" /></td>
                <td><input v-model.number="tier.concurrency" :aria-label="`VIP ${tier.level} 并发目标`" type="number" min="1" max="1000" class="input" :disabled="busy" /></td>
                <td><input v-model.number="tier.rpm" :aria-label="`VIP ${tier.level} RPM 目标`" type="number" min="0" max="1000" class="input" :disabled="busy" /></td>
                <td><input v-model.number="tier.rebate_percent" :aria-label="`VIP ${tier.level} 邀请返利百分比`" type="number" min="0" max="10" class="input" :disabled="busy" /></td>
              </tr></tbody>
            </table></div>
            <p class="vip-note">并发不会降低原基础额度。RPM 为 0 时沿用原有限流；当前不限流基础不被自动改为有限额度。</p>
          </section>
        </div>
        <div id="vip-panel-groups" v-show="activeTab === 'groups'" role="tabpanel" aria-labelledby="vip-tab-groups">
          <div class="vip-table-wrap vip-group-table"><table class="vip-table">
            <thead><tr><th>分组</th><th>属性 / 基础倍率</th><th>VIP 准入</th><th>倍率下限</th><th v-for="n in 5" :key="n">VIP {{ n }} 减免</th></tr></thead>
            <tbody><tr v-for="group in groups" :key="group.id">
              <td class="font-medium">{{ group.name }}</td><td class="whitespace-nowrap text-gray-500">{{ excluded(group) ? '私人 / 订阅 / 测试' : group.is_exclusive ? '专属' : '公开' }} · {{ group.rate_multiplier }}</td>
              <td><input v-model="rule(group.id).access" type="checkbox" :aria-label="`${group.name} VIP 准入`" :disabled="busy || !group.is_exclusive || excluded(group)" /></td>
              <td><input v-model.number="rule(group.id).floor" :aria-label="`${group.name} 倍率下限`" type="number" min="0" :max="group.rate_multiplier" step="0.001" :disabled="busy || group.is_exclusive || excluded(group)" class="input" /></td>
              <td v-for="n in 5" :key="n"><input v-model.number="rule(group.id).discounts[n - 1]" :data-discount="`${group.id}-${n}`" :aria-label="`${group.name} VIP ${n} 减免`" type="number" min="0" max="0.1" step="0.001" :disabled="busy || group.is_exclusive || excluded(group)" class="input" /></td>
            </tr><tr v-if="!groups.length"><td colspan="9" class="text-center text-gray-500">暂无分组</td></tr></tbody>
          </table></div>
        </div>
        <div id="vip-panel-users" v-show="activeTab === 'users'" role="tabpanel" aria-labelledby="vip-tab-users" class="space-y-6">
          <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadUser">
            <label class="vip-field">用户 ID<input v-model.number="userId" data-user-id type="number" min="1" class="input" :disabled="busy" /></label>
            <button type="submit" data-load-user class="btn btn-secondary" :disabled="busy || !validUserId"><Icon name="search" size="sm" />查看权益</button>
          </form>
          <p v-if="!userState" class="py-12 text-center text-sm text-gray-500">请选择用户查看有效权益</p>
          <template v-else>
            <div data-user-summary class="vip-user-summary">
              <div><span>用户 ID</span><strong>{{ loadedUserId }}</strong></div><div><span>累计有效充值</span><strong>${{ userState.total.toFixed(2) }}</strong></div>
              <div><span>充值成长等级</span><strong>VIP {{ userState.tier.level }}</strong></div><div><span>有效并发</span><strong>{{ userState.concurrency }}</strong></div><div><span>邀请返利</span><strong>{{ userState.rebate_percent }}%</strong></div>
            </div>
            <section class="vip-section">
              <h2>人工权益覆盖</h2>
              <div class="vip-table-wrap"><table class="vip-table"><thead><tr><th>权益</th><th>有效人工值</th><th>原因</th><th>操作</th></tr></thead>
                <tbody><tr v-for="item in userState.overrides" :key="item.benefit"><td>{{ benefits[item.benefit] }}</td><td>{{ item.value }}</td><td class="vip-reason">{{ item.reason }}</td><td><button type="button" :data-restore="item.benefit" class="btn btn-secondary btn-sm" :disabled="busy || !userMatches" @click="requestRestore(item.benefit)">恢复自动</button></td></tr>
                  <tr v-if="!userState.overrides.length"><td colspan="4" class="text-center text-gray-500">暂无人工覆盖，沿用自动权益</td></tr></tbody>
              </table></div>
              <form class="vip-override-form mt-5" @submit.prevent="setOverride">
                <div class="vip-field"><label for="vip-benefit">权益</label><Select id="vip-benefit" v-model="override.benefit" :options="benefitOptions" :disabled="busy" aria-label="覆盖权益" /></div>
                <label class="vip-field">指定值<input v-model.number="override.value" type="number" step="0.001" min="0" class="input" :disabled="busy" /></label>
                <label class="vip-field">到期时间（可选）<input v-model="expiry" type="datetime-local" class="input" :disabled="busy" /></label>
                <label class="vip-field">原因<input v-model="override.reason" class="input" :disabled="busy" /></label>
                <button type="submit" class="btn btn-primary" :disabled="busy || !userMatches">保存覆盖</button>
              </form>
              <p class="vip-note">访问资格 1 为授予候选 VIP 组、0 为禁用自动资格；等级标识 0～5；减免 0～0.1；返利 0～10。已有人工组授权和定制价格仍由原入口管理。</p>
            </section>
            <section class="vip-section"><details><summary class="cursor-pointer text-sm font-medium">历史充值初始确认</summary>
              <form class="mt-4 flex flex-wrap items-end gap-3" @submit.prevent="requestOpening">
                <label class="vip-field">确认金额<input v-model.number="openingAmount" data-opening-amount type="number" min="0" class="input" :disabled="busy" /></label>
                <label class="vip-field">核对原因<input v-model="openingReason" data-opening-reason class="input" :disabled="busy" /></label>
                <button type="submit" data-opening class="btn btn-secondary" :disabled="busy || !userMatches">记录一次性初始额</button>
              </form><p class="vip-note">每个用户只能创建一次初始记录，不改变余额。须先核对已有成长流水，避免把已计入的充值重复回填。</p>
            </details></section>
          </template>
        </div>
      </template>
      <ConfirmDialog :show="!!confirmation" :title="confirmation?.title || ''" :message="confirmation?.message || ''" @confirm="confirmAction" @cancel="confirmation = null" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { apiClient } from '@/api/client'
import { getVIP, getVIPRules, saveVIPRules, saveVIPOverride, clearVIPOverride, createVIPOpening, type VIPRules, type VIPSnapshot, type VIPOverride } from '@/api/vip'
import type { Group } from '@/types'

const appStore = useAppStore()
const tabs = [{ id: 'rules', label: '等级与规则' }, { id: 'groups', label: '分组权益' }, { id: 'users', label: '用户权益' }]
const activeTab = ref('rules')
const rules = ref<VIPRules | null>(null)
const groups = ref<Group[]>([])
const busy = ref(false)
const error = ref('')
const userId = ref(0)
const loadedUserId = ref(0)
const userState = ref<VIPSnapshot | null>(null)
const newCurrency = ref('')
const expiry = ref('')
const openingAmount = ref(0)
const openingReason = ref('')
const override = ref<VIPOverride>({ benefit: 'badge', value: 1, reason: '', expires_at: null })
const benefits: Record<string, string> = { discount: '普通组减免', access: '自动专属资格', concurrency: '并发', rpm: 'RPM', rebate: '邀请返利比例', badge: '身份标识' }
const benefitOptions = Object.entries(benefits).map(([value, label]) => ({ value, label }))
const validUserId = computed(() => Number.isInteger(userId.value) && userId.value > 0)
const userMatches = computed(() => validUserId.value && userId.value === loadedUserId.value)
const savedRules = ref('')
const confirmation = ref<{ title: string; message: string; execute: () => Promise<void> } | null>(null)
const excluded = (group: Group) => ['zth-plus', 'zth-pro', 'ceshi', 'ceshi-gemini'].includes(group.name) || group.subscription_type === 'subscription'

function rule(id: number) {
  const result = rules.value!.groups.find(group => group.group_id === id)
  if (!result) throw new Error('分组规则缺失')
  return result
}
function navigateTabs(event: KeyboardEvent, id: string) {
  const index = tabs.findIndex(tab => tab.id === id)
  const next = event.key === 'ArrowRight' ? (index + 1) % tabs.length : event.key === 'ArrowLeft' ? (index + tabs.length - 1) % tabs.length : event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : -1
  if (next < 0) return
  event.preventDefault()
  activeTab.value = tabs[next].id
  document.getElementById(`vip-tab-${activeTab.value}`)?.focus()
}
function addCurrency() {
  const currency = newCurrency.value.toUpperCase()
  if (!/^[A-Z]{3}$/.test(currency) || !rules.value) { appStore.showError('请输入三位币种代码'); return }
  if (currency in rules.value.exchange_rates) { appStore.showError('该币种已存在'); return }
  rules.value.exchange_rates[currency] = 1
  newCurrency.value = ''
}
async function action(work: () => Promise<void>, message?: string) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try { await work(); if (message) appStore.showSuccess(message) }
  catch (cause) {
    const apiError = cause as { message?: unknown }
    error.value = typeof apiError?.message === 'string' ? apiError.message : '操作失败，请重试'
    appStore.showError(error.value)
  } finally { busy.value = false }
}
async function loadRules() {
  await action(async () => {
    const [config, all] = await Promise.all([getVIPRules(), apiClient.get<Group[]>('/admin/groups/all')])
    config.exchange_rates ||= { USD: 1 }
    for (const group of all.data) {
      if (!config.groups.some(item => item.group_id === group.id)) config.groups.push({ group_id: group.id, private: excluded(group), access: false, floor: group.rate_multiplier, discounts: [0, 0, 0, 0, 0] })
    }
    rules.value = config
    groups.value = all.data
    savedRules.value = JSON.stringify(config)
  })
}
function requestRefresh() {
  if (activeTab.value === 'users') { if (validUserId.value) void loadUser(); return }
  if (rules.value && JSON.stringify(rules.value) !== savedRules.value) {
    confirmation.value = { title: '重新加载规则', message: '存在尚未保存的规则修改，刷新将放弃这些修改。是否继续？', execute: loadRules }
  } else void loadRules()
}
async function save() {
  if (!rules.value) return
  await action(async () => { await saveVIPRules(rules.value!); savedRules.value = JSON.stringify(rules.value) }, 'VIP 规则已保存')
}
async function loadUser() {
  if (!validUserId.value) return
  const id = userId.value
  userState.value = null
  loadedUserId.value = 0
  await action(async () => { userState.value = await getVIP(id); loadedUserId.value = id })
}
function requestRestore(benefit: string) {
  if (!userMatches.value || busy.value) return
  const id = loadedUserId.value
  confirmation.value = { title: '恢复自动权益', message: `将移除用户 ID ${id} 的“${benefits[benefit]}”人工覆盖，并按现有 VIP 规则重新计算。是否继续？`, execute: async () => {
    await action(async () => { await clearVIPOverride(id, benefit); userState.value = await getVIP(id) }, '已恢复自动权益')
  } }
}
async function setOverride() {
  if (!userMatches.value) return
  const id = loadedUserId.value
  await action(async () => {
    await saveVIPOverride(id, { ...override.value, expires_at: expiry.value ? new Date(expiry.value).toISOString() : null })
    userState.value = await getVIP(id)
  }, '人工权益覆盖已保存')
}
function requestOpening() {
  if (!userMatches.value || busy.value) return
  const id = loadedUserId.value, amount = openingAmount.value, reason = openingReason.value
  confirmation.value = { title: '确认历史充值初始额', message: `为用户 ID ${id} 记录 $${Number(amount).toFixed(2)} 的一次性初始额，不改变余额。请确认已核对历史流水、不会重复计入充值。`, execute: async () => {
    await action(async () => { await createVIPOpening(id, amount, reason); userState.value = await getVIP(id) }, '历史充值初始额已记录')
  } }
}
async function confirmAction() {
  const pending = confirmation.value
  confirmation.value = null
  if (pending && !busy.value) await pending.execute()
}
onMounted(loadRules)
</script>

<style scoped>
.vip-admin { min-width: 0; }
.vip-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.vip-tabs { display: flex; gap: 24px; border-bottom: 1px solid var(--mg-line-warm); }
.vip-tabs button { padding: 12px 0; border-bottom: 2px solid transparent; font-size: 14px; white-space: nowrap; color: var(--mg-muted); }
.vip-tabs button.active { border-color: var(--mg-gold-700); color: var(--mg-ink-900); font-weight: 600; }
.vip-tabs button:focus-visible { outline: 2px solid var(--mg-gold-700); outline-offset: 4px; }
.vip-section { padding: 20px 0; border-top: 1px solid var(--mg-line-warm); }
.vip-section h2 { margin-bottom: 16px; font-size: 16px; font-weight: 600; }
.vip-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 240px)); gap: 16px; }
.vip-field { display: flex; flex-direction: column; gap: 8px; font-size: 13px; min-width: 0; }
.vip-field .input { width: 100%; }
.vip-field .btn { flex-shrink: 0; white-space: nowrap; }
.vip-note { margin-top: 14px; font-size: 12px; line-height: 1.7; color: var(--mg-muted); }
.vip-table-wrap { overflow-x: auto; border: 1px solid var(--mg-line-warm); border-radius: 6px; background: var(--mg-surface); }
.vip-table { width: 100%; text-align: left; font-size: 13px; }
.vip-table th { white-space: nowrap; font-weight: 500; }
.vip-table th, .vip-table td { padding: 12px 16px; border-bottom: 1px solid var(--mg-line-warm); }
.vip-table tbody tr:last-child td { border-bottom: 0; }
.vip-table .input { min-width: 88px; width: 100%; max-width: 200px; }
.vip-table .btn { white-space: nowrap; }
.vip-group-table .vip-table { min-width: 1180px; }
.vip-group-table .input { width: 92px; }
.vip-user-summary { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 20px; padding: 18px 0; border-block: 1px solid var(--mg-line-warm); }
.vip-user-summary span { display: block; margin-bottom: 8px; font-size: 12px; color: var(--mg-muted); }
.vip-user-summary strong { font-size: 15px; font-weight: 500; overflow-wrap: anywhere; }
.vip-override-form { display: grid; grid-template-columns: minmax(140px, 1fr) 110px minmax(210px, 1fr) minmax(180px, 2fr) auto; align-items: end; gap: 16px; }
.vip-reason { min-width: 160px; max-width: 400px; overflow-wrap: anywhere; }
@media (max-width: 1100px) { .vip-override-form { grid-template-columns: repeat(2, minmax(0, 1fr)); } .vip-user-summary { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
@media (max-width: 640px) { .vip-tabs { gap: 20px; } .vip-tabs button { font-size: 13px; } .vip-fields, .vip-override-form { grid-template-columns: minmax(0, 1fr); } .vip-user-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } .vip-table th, .vip-table td { padding: 10px 12px; } }
</style>
