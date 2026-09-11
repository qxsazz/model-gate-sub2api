<template>
  <div class="console-preview" :class="{ 'is-dark': isDark }" aria-label="控制台预览">
    <aside class="preview-sidebar">
      <div class="preview-brand"><img :src="siteLogo || '/logo.svg'" alt="" /><strong>{{ siteName }}</strong></div>
      <div class="preview-tabs" role="tablist" aria-label="控制台预览选项卡" @keydown="navigateTabs">
        <button v-for="tab in tabs" :id="'preview-tab-' + tab.id" :key="tab.id" type="button" role="tab"
          :data-tab="tab.id" :aria-selected="currentTab === tab.id" :aria-controls="'preview-panel-' + tab.id"
          :tabindex="currentTab === tab.id ? 0 : -1" :class="{ selected: currentTab === tab.id }" @click="currentTab = tab.id">
          <Icon :name="tab.icon" size="md" /><span>{{ tab.title }}</span>
        </button>
      </div>
      <button class="preview-theme" type="button" @click="$emit('toggle-theme')">
        <Icon :name="isDark ? 'sun' : 'moon'" size="md" />{{ isDark ? '浅色模式' : '深色模式' }}
      </button>
    </aside>
    <div class="preview-main">
      <header class="preview-topbar">
        <div><h3>{{ selectedTab.title }}</h3><p>{{ selectedTab.description }}</p></div>
        <div class="preview-account">
          <RouterLink to="/model-plaza" class="preview-plaza"><Icon name="grid" size="sm" />模型广场</RouterLink>
          <span class="preview-balance">$12.80</span>
          <span class="preview-avatar">访</span><span class="preview-user">访客****<small>演示账户</small></span>
        </div>
      </header>
      <div class="preview-notice">账户与调用记录为脱敏演示数据 · 不读取真实密钥 · 可用渠道来自模型广场</div>
      <div v-if="notice" class="preview-feedback" role="status">{{ notice }}<button type="button" aria-label="关闭提示" @click="notice = ''">×</button></div>
      <div :id="'preview-panel-' + currentTab" :key="currentTab + '-' + isDark" class="preview-panel" role="tabpanel" :aria-labelledby="'preview-tab-' + currentTab">
        <template v-if="active && currentTab === 'overview'">
          <div class="space-y-6">
            <UserDashboardStats :stats="dashboardStats" :balance="12.8" :is-simple="false" />
            <UserDashboardCharts :key="refreshVersion" v-model:start-date="startDate" v-model:end-date="endDate"
              v-model:granularity="granularity" :loading="false" :models="charts.models" :trend="charts.trend" @refresh="refreshDemo" />
          </div>
        </template>
        <template v-else-if="active && currentTab === 'usage'">
          <div class="preview-summary">
            <div v-for="card in usageCards" :key="card.label" class="card p-4 flex items-center gap-3">
              <div class="preview-stat-icon" :class="card.color"><Icon :name="card.icon" size="md" /></div>
              <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ card.label }}</p><strong class="text-xl">{{ card.value }}</strong><p class="text-xs text-gray-500 dark:text-gray-400">{{ card.detail }}</p></div>
            </div>
          </div>
          <div class="card p-4 my-6 flex flex-wrap items-center gap-3">
            <span class="text-sm">时间范围:</span><DateRangePicker v-model:start-date="startDate" v-model:end-date="endDate" />
            <div class="ml-auto flex items-center gap-2 text-sm"><span>粒度:</span><div class="w-28"><Select v-model="granularity" :options="[{ value: 'day', label: '按天' }, { value: 'hour', label: '按小时' }]" /></div></div>
          </div>
          <div :key="refreshVersion" class="preview-chart-grid">
            <ModelDistributionChart v-model:metric="modelMetric" :model-stats="charts.models" :show-metric-toggle="true" :enable-breakdown="false" :show-account-cost="false" />
            <GroupDistributionChart v-model:metric="groupMetric" :group-stats="charts.groups" :show-metric-toggle="true" :enable-breakdown="false" :show-account-cost="false" />
            <EndpointDistributionChart v-model:metric="endpointMetric" :endpoint-stats="charts.endpoints" :show-metric-toggle="true" :enable-breakdown="false" title="端点分布" />
            <TokenUsageTrend :trend-data="charts.trend" />
          </div>
          <div class="preview-table-card mt-6">
            <div class="preview-table-heading"><strong>使用记录</strong><input v-model="usageSearch" class="input" aria-label="搜索使用记录" placeholder="搜索模型或密钥..." /></div>
            <div class="preview-table-scroll"><table class="preview-table"><thead><tr><th>API 密钥</th><th>模型</th><th>分组</th><th>Token</th><th>实际消费</th><th>耗时</th><th>时间</th></tr></thead>
              <tbody><tr v-for="record in visibleUsage" :key="record.id"><td>{{ record.key }}</td><td>{{ record.model }}</td><td>{{ record.group }}</td><td>{{ formatTokens(record.total_tokens) }}</td><td class="preview-money">{{ money(record.actual_cost) }}</td><td>{{ (record.duration / 1000).toFixed(2) }}s</td><td>{{ record.date }}</td></tr><tr v-if="!visibleUsage.length"><td colspan="7" class="preview-empty">暂无匹配的使用记录</td></tr></tbody>
            </table></div>
            <div class="preview-pagination"><span>共 {{ searchedUsage.length }} 条</span><button class="btn btn-secondary" :disabled="usagePage === 1" @click="usagePage--">上一页</button><span>{{ usagePage }} / {{ usagePageCount }}</span><button class="btn btn-secondary" :disabled="usagePage >= usagePageCount" @click="usagePage++">下一页</button></div>
          </div>
        </template>
        <template v-else-if="currentTab === 'apikeys'">
          <div class="preview-key-actions"><button class="btn btn-secondary" aria-label="刷新演示密钥" @click="showDemoNotice('演示密钥已刷新')"><Icon name="refresh" size="md" /></button><button class="btn btn-secondary" @click="columnsOpen = !columnsOpen">列设置</button><button class="btn btn-primary" data-action="create-key" @click="showDemoNotice('预览不会创建真实密钥，请登录后创建')"><Icon name="plus" size="md" />创建密钥</button></div>
          <div v-if="columnsOpen" class="card p-4 mb-4 flex flex-wrap gap-4"><label v-for="column in optionalColumns" :key="column.id" class="flex items-center gap-2 text-sm"><input v-model="shownColumns" type="checkbox" :value="column.id" />{{ column.title }}</label></div>
          <div class="preview-key-filters"><input v-model="keySearch" class="input" aria-label="搜索密钥" placeholder="搜索名称或 Key..." /><Select v-model="keyGroup" :options="keyGroupOptions" /><Select v-model="keyStatus" :options="[{ value: '', label: '全部状态' }, { value: 'active', label: '活跃' }, { value: 'disabled', label: '已禁用' }]" /></div>
          <div class="preview-endpoint">API 端点 <span>演示</span><code>https://api.example.com</code></div>
          <div class="preview-table-card"><div class="preview-table-scroll"><table class="preview-table preview-keys-table">
            <thead><tr><th>名称</th><th>API 密钥</th><th>分组</th><th v-if="shownColumns.includes('concurrency')">当前并发</th><th>用量</th><th v-if="shownColumns.includes('expiry')">过期时间</th><th>状态</th><th v-if="shownColumns.includes('created')">创建时间</th><th>操作</th></tr></thead>
            <tbody><tr v-for="key in filteredKeys" :key="key.suffix"><td class="font-medium">{{ key.name }}</td><td><code class="preview-key">sk-demo-****{{ key.suffix }}</code></td><td><span class="preview-group" :data-platform="key.platform">{{ key.group }} <b>{{ key.rate }}x</b></span></td><td v-if="shownColumns.includes('concurrency')"><span class="preview-concurrency">0</span></td><td><div>今日: {{ money(keyUsage(key.name, true)) }}</div><div class="text-gray-500 dark:text-gray-400">近7天: {{ money(keyUsage(key.name, false)) }}</div></td><td v-if="shownColumns.includes('expiry')" class="text-gray-500 dark:text-gray-400">永久有效</td><td><span class="preview-status">活跃</span></td><td v-if="shownColumns.includes('created')" class="text-gray-500 dark:text-gray-400">{{ initialStart }} 10:00</td><td><div class="preview-row-actions"><button v-for="action in keyActions" :key="action.label" :title="action.label + '（演示）'" @click="showDemoNotice(action.label + '仅供预览，不会更改真实密钥')"><Icon :name="action.icon" size="sm" /><span>{{ action.label }}</span></button></div></td></tr><tr v-if="!filteredKeys.length"><td :colspan="6 + shownColumns.length" class="preview-empty">没有匹配的密钥</td></tr></tbody>
          </table></div></div>
        </template>
        <template v-else-if="currentTab === 'channels'">
          <div class="preview-channel-tools"><input v-model="channelSearch" class="input" aria-label="搜索渠道或模型" placeholder="搜索渠道、分组或模型..." /><button class="btn btn-secondary" aria-label="刷新渠道" :disabled="channelsLoading" @click="loadChannels"><Icon name="refresh" size="md" :class="{ 'animate-spin': channelsLoading }" /></button></div>
          <div v-if="channelsError" class="preview-error" role="alert">{{ channelsError }}<button class="btn btn-secondary" @click="loadChannels">重试</button></div>
          <div v-else class="preview-table-card preview-channels">
            <AvailableChannelsTable :rows="channelRows" :columns="channelColumns" :loading="channelsLoading" :user-group-rates="{}" pricing-key-prefix="availableChannels.pricing" no-pricing-label="暂无定价" no-models-label="暂无模型" empty-label="暂无可展示的渠道" />
          </div>
          <p class="preview-channel-note">按模型广场的公开分组展示，不包含账户专属分组；模型和价格以接口实时返回为准。</p>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import AvailableChannelsTable from '@/components/channels/AvailableChannelsTable.vue'
import { getModelPlaza, type ModelPlazaGroup } from '@/api/modelPlaza'
import type { UserAvailableChannel } from '@/api/channels'
import { formatDateLocalInput, formatTokensK as formatTokens } from '@/utils/format'
import { createPreviewRecords, previewCharts, previewDashboardStats, previewKeys, summarizePreview } from './previewData'

const props = defineProps<{ isDark: boolean; siteName: string; siteLogo: string; active: boolean }>()
defineEmits<{ 'toggle-theme': [] }>()
const tabs = [
  { id: 'overview', title: '仪表盘', icon: 'grid', description: '欢迎回来！这是您账户的概览。' },
  { id: 'apikeys', title: 'API 密钥', icon: 'key', description: '管理您的 API 密钥和访问令牌' },
  { id: 'usage', title: '使用记录', icon: 'chart', description: '查看和分析您的 API 使用历史' },
  { id: 'channels', title: '可用渠道', icon: 'cube', description: '查看公开渠道分组及其支持的模型、定价' }
] as const
const currentTab = ref('overview')
const selectedTab = computed(() => tabs.find(tab => tab.id === currentTab.value) || tabs[0])
const today = formatDateLocalInput(new Date())
const initialDate = new Date()
initialDate.setDate(initialDate.getDate() - 6)
const initialStart = formatDateLocalInput(initialDate)
const startDate = ref(initialStart)
const endDate = ref(today)
const granularity = ref('day')
const records = createPreviewRecords()
const periodRecords = computed(() => records.filter(record => record.date.slice(0, 10) >= startDate.value && record.date.slice(0, 10) <= endDate.value))
const charts = computed(() => previewCharts(periodRecords.value, granularity.value))
const summary = computed(() => summarizePreview(periodRecords.value))
const dashboardStats = previewDashboardStats(records, today)
const modelMetric = ref<'tokens' | 'actual_cost'>('tokens')
const groupMetric = ref<'tokens' | 'actual_cost'>('tokens')
const endpointMetric = ref<'tokens' | 'actual_cost'>('tokens')
const money = (value: number) => '$' + value.toFixed(4)
const usageCards = computed(() => [
  { label: '总请求数', value: summary.value.requests.toLocaleString(), detail: '所选范围内', icon: 'document', color: 'blue' },
  { label: '总 Token', value: formatTokens(summary.value.total_tokens), detail: '输入: ' + formatTokens(summary.value.input_tokens) + ' / 输出: ' + formatTokens(summary.value.output_tokens), icon: 'cube', color: 'amber' },
  { label: '总消费', value: money(summary.value.actual_cost), detail: '标准 ' + money(summary.value.cost), icon: 'dollar', color: 'green' },
  { label: '平均耗时', value: (periodRecords.value.length ? periodRecords.value.reduce((sum, record) => sum + record.duration, 0) / periodRecords.value.length / 1000 : 0).toFixed(2) + 's', detail: '平均时间', icon: 'clock', color: 'purple' }
 ] as const)
const notice = ref('')
const refreshVersion = ref(0)
function showDemoNotice(message: string) { notice.value = message }
function refreshDemo() { refreshVersion.value++; notice.value = '演示图表已刷新，未请求真实账户数据' }
async function navigateTabs(event: KeyboardEvent) {
  if (!['ArrowDown', 'ArrowUp', 'ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const tabList = event.currentTarget as HTMLElement | null
  const index = tabs.findIndex(tab => tab.id === currentTab.value)
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : (index + (['ArrowDown', 'ArrowRight'].includes(event.key) ? 1 : -1) + tabs.length) % tabs.length
  currentTab.value = tabs[next].id
  await nextTick()
  tabList?.querySelector<HTMLButtonElement>('[aria-selected="true"]')?.focus()
}
const keySearch = ref('')
const keyGroup = ref('')
const keyStatus = ref('')
const columnsOpen = ref(false)
const shownColumns = ref(['concurrency', 'expiry', 'created'])
const optionalColumns = [{ id: 'concurrency', title: '当前并发' }, { id: 'expiry', title: '过期时间' }, { id: 'created', title: '创建时间' }]
const keyActions = [{ label: '使用密钥', icon: 'terminal' }, { label: '导入', icon: 'upload' }, { label: '禁用', icon: 'ban' }, { label: '编辑', icon: 'edit' }, { label: '删除', icon: 'trash' }] as const
const keyGroupOptions = [{ value: '', label: '全部分组' }, ...[...new Set(previewKeys.map(key => key.group))].map(group => ({ value: group, label: group }))]
const filteredKeys = computed(() => previewKeys.filter(key => keyStatus.value !== 'disabled' && (!keyGroup.value || key.group === keyGroup.value) && (key.name + ' sk-demo-****' + key.suffix).toLowerCase().includes(keySearch.value.trim().toLowerCase())))
const keyUsage = (name: string, daily: boolean) => summarizePreview(records.filter(record => record.key === name && (!daily || record.date.startsWith(today)))).actual_cost
const usageSearch = ref('')
const usagePage = ref(1)
const searchedUsage = computed(() => periodRecords.value.filter(record => (record.model + record.key).toLowerCase().includes(usageSearch.value.trim().toLowerCase())))
const usagePageCount = computed(() => Math.max(1, Math.ceil(searchedUsage.value.length / 10)))
const visibleUsage = computed(() => searchedUsage.value.slice((usagePage.value - 1) * 10, usagePage.value * 10))
watch([usageSearch, startDate, endDate], () => { usagePage.value = 1 })
const channels = ref<ModelPlazaGroup[]>([])
const channelSearch = ref('')
const channelsLoading = ref(false)
const channelsLoaded = ref(false)
const channelsError = ref('')
let channelRequest: AbortController | null = null
const channelColumns = { name: '渠道 / 分组', description: '描述', platform: '平台', groups: '公开分组', supportedModels: '支持模型' }
const channelRows = computed<UserAvailableChannel[]>(() => {
  const query = channelSearch.value.trim().toLowerCase()
  return channels.value.filter(group => !group.is_exclusive && (group.name + ' ' + group.description + ' ' + group.platform + ' ' + group.models.map(model => model.name).join(' ')).toLowerCase().includes(query))
    .map(group => ({ name: group.name, description: group.description, platforms: [...new Set(group.models.length ? group.models.map(model => model.platform || group.platform) : [group.platform])].map(platform => ({ platform, groups: [group], supported_models: group.models.filter(model => (model.platform || group.platform) === platform) })) }))
})
async function loadChannels() {
  if (channelsLoading.value) return
  channelRequest = new AbortController()
  channelsLoading.value = true
  channelsError.value = ''
  try {
    const response = await getModelPlaza({ signal: channelRequest.signal })
    channels.value = response.groups || []
    channelsLoaded.value = true
  } catch (error) {
    if (channelRequest.signal.aborted) return
    channels.value = []
    const status = (error as { response?: { status?: number } }).response?.status
    channelsError.value = status === 401 || status === 403 ? '模型广场需要登录或访问权限，登录后可查看。' : status === 404 ? '模型广场暂未开放，暂无公开渠道可展示。' : '模型广场加载失败，请稍后重试。'
  } finally { channelsLoading.value = false }
}
watch([currentTab, () => props.active], () => {
  if (props.active && currentTab.value === 'channels' && !channelsLoaded.value) void loadChannels()
})
onBeforeUnmount(() => channelRequest?.abort())
</script>

<style scoped>
.console-preview { --preview-bg: #f5f7fb; --preview-surface: #fff; --preview-side: #fff; --preview-header: #f0f3f8; --preview-border: #dce3ec; --preview-muted: #596779; --preview-text: #182233; display: grid; grid-template-columns: 220px minmax(0, 1fr); overflow: hidden; width: 100%; border: 1px solid var(--preview-border); border-radius: 18px; background: var(--preview-bg); color: var(--preview-text); box-shadow: 0 24px 80px #17203312; text-align: left; }
.console-preview.is-dark { --preview-bg: #050c19; --preview-surface: #101c2c; --preview-side: #0f1729; --preview-header: #1b283a; --preview-border: #293648; --preview-muted: #9ba8b8; --preview-text: #f1f5f9; box-shadow: 0 24px 80px #0004; }
.preview-sidebar { display: flex; flex-direction: column; min-width: 0; background: var(--preview-side); border-right: 1px solid var(--preview-border); }
.preview-brand { min-height: 65px; padding: 14px 16px; display: flex; align-items: center; gap: 10px; border-bottom: 1px solid var(--preview-border); font-size: 20px; }
.preview-brand img { width: 34px; height: 34px; object-fit: contain; border-radius: 8px; }
.preview-brand strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-tabs { padding: 16px 10px; display: flex; flex-direction: column; gap: 5px; }
.preview-tabs button, .preview-theme { display: flex; align-items: center; gap: 12px; padding: 13px 15px; font-size: 14px; border-radius: 10px; text-align: left; transition: background-color .2s, color .2s; }
.preview-tabs button:hover { background: #14b8a614; }
.preview-tabs button.selected { background: #14b8a617; color: #008b80; }
.console-preview.is-dark .preview-tabs button.selected { color: #00e2cd; }
.preview-tabs button:focus-visible, .preview-row-actions button:focus-visible { outline: 2px solid #14b8a6; outline-offset: 2px; }
.preview-theme { margin-top: auto; border-top: 1px solid var(--preview-border); border-radius: 0; }
.preview-theme svg { color: #d79c0b; }
.preview-main { min-width: 0; background: radial-gradient(ellipse at 38% 0%, #06b6d40b, transparent 65%), var(--preview-bg); }
.preview-topbar { min-height: 65px; padding: 10px 24px; display: flex; align-items: center; justify-content: space-between; gap: 16px; background: var(--preview-header); border-bottom: 1px solid var(--preview-border); }
.preview-topbar h3 { font-size: 19px; font-weight: 700; line-height: 1.3; }
.preview-topbar p { font-size: 12px; color: var(--preview-muted); margin-top: 3px; }
.preview-account { display: flex; align-items: center; gap: 12px; flex-shrink: 0; font-size: 13px; }
.preview-plaza { display: flex; align-items: center; gap: 6px; color: var(--preview-muted); }
.preview-balance { padding: 6px 12px; border-radius: 12px; color: #008b80; background: #14b8a610; font-weight: 700; }
.console-preview.is-dark .preview-balance { color: #5eead4; }
.preview-avatar { display: grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; background: #e5eee2; color: #355d3b; font-weight: 600; }
.preview-user small { display: block; font-size: 11px; color: var(--preview-muted); }
.preview-notice { padding: 9px 24px; border-bottom: 1px solid var(--preview-border); color: var(--preview-muted); font-size: 11px; }
.preview-feedback { display: flex; justify-content: space-between; align-items: center; padding: 12px 24px; font-size: 13px; background: #14b8a615; }
.preview-feedback button { padding: 0 8px; font-size: 20px; }
.preview-panel { min-width: 0; min-height: 660px; max-height: 790px; overflow: auto; overscroll-behavior: contain; padding: 24px; animation: preview-enter .25s ease-out; }
.preview-panel :deep(.card) { background: var(--preview-surface); border-color: var(--preview-border); border-radius: 16px; }
.preview-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.preview-stat-icon { padding: 10px; border-radius: 9px; }
.preview-stat-icon.blue { background: #3b82f619; color: #3b82f6; }
.preview-stat-icon.amber { background: #f59e0b19; color: #d88d00; }
.preview-stat-icon.green { background: #10b98119; color: #10b981; }
.preview-stat-icon.purple { background: #a855f719; color: #a855f7; }
.preview-chart-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
.preview-key-actions { display: flex; justify-content: flex-end; gap: 12px; margin: 4px 0 24px; }
.preview-key-filters { display: grid; grid-template-columns: minmax(180px, 260px) 170px 150px; gap: 12px; margin-bottom: 12px; }
.preview-endpoint { display: inline-flex; flex-wrap: wrap; align-items: center; gap: 10px; border: 1px solid var(--preview-border); background: var(--preview-header); border-radius: 8px; padding: 6px 10px; margin-bottom: 24px; color: var(--preview-muted); font-size: 11px; }
.preview-endpoint span { color: #008b80; background: #14b8a618; padding: 0 4px; border-radius: 3px; }
.preview-table-card { background: var(--preview-surface); border: 1px solid var(--preview-border); border-radius: 16px; overflow: hidden; }
.preview-table-scroll { overflow-x: auto; }
.preview-table { width: 100%; border-collapse: collapse; font-size: 13px; white-space: nowrap; }
.preview-table thead { background: var(--preview-header); color: var(--preview-muted); }
.preview-table th { padding: 17px 18px; text-align: left; font-weight: 500; }
.preview-table td { padding: 21px 18px; border-top: 1px solid var(--preview-border); }
.preview-table tbody tr { transition: background-color .2s; }
.preview-table tbody tr:hover { background: #14b8a608; }
.preview-key { font-size: 11px; color: #008b80; background: #14b8a612; padding: 4px 6px; border-radius: 4px; }
.console-preview.is-dark .preview-key { color: #00e2cd; }
.preview-group { display: inline-flex; align-items: center; gap: 8px; padding: 4px 6px; font-size: 11px; color: #08854e; background: #10b98112; border-radius: 5px; }
.preview-group[data-platform="anthropic"] { color: #b57a00; background: #f59e0b12; }
.preview-group[data-platform="gemini"] { color: #058bb3; background: #06b6d412; }
.preview-group[data-platform="grok"] { color: var(--preview-text); background: #94a3b812; }
.console-preview.is-dark .preview-group[data-platform="anthropic"] { color: #fbbf24; }
.console-preview.is-dark .preview-group[data-platform="openai"] { color: #34d399; }
.preview-group b { background: #94a3b81c; border-radius: 4px; padding: 0 4px; }
.preview-concurrency { padding: 5px 10px; border-radius: 4px; background: var(--preview-header); font-weight: 700; }
.preview-status { color: #008b80; background: #14b8a610; border-radius: 12px; padding: 3px 9px; font-size: 11px; }
.console-preview.is-dark .preview-status { color: #2dd4bf; }
.preview-row-actions { display: flex; gap: 12px; color: var(--preview-muted); }
.preview-row-actions button { display: flex; align-items: center; flex-direction: column; gap: 5px; font-size: 10px; transition: color .2s; }
.preview-row-actions button:hover { color: #0d9488; }
.preview-empty { text-align: center; color: var(--preview-muted); height: 150px; }
.preview-channel-tools { display: flex; align-items: center; justify-content: space-between; gap: 16px; margin: 4px 0 24px; }
.preview-channel-tools input { max-width: 320px; }
.preview-channels :deep(table) { table-layout: auto; }
.preview-channels :deep(th:first-child) { width: 150px; }
.preview-channels :deep(th:nth-child(2)) { width: 130px; }
.preview-channels :deep(th:nth-child(3)) { width: 120px; }
.preview-channels :deep(th:nth-child(4)) { width: 27%; }
.preview-channels :deep(td) { padding-top: 18px; padding-bottom: 18px; }
.preview-channel-note { margin-top: 14px; color: var(--preview-muted); font-size: 12px; }
.preview-error { padding: 24px; display: flex; gap: 16px; flex-wrap: wrap; align-items: center; color: var(--preview-muted); background: var(--preview-surface); border: 1px solid var(--preview-border); border-radius: 12px; }
.preview-table-heading { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; justify-content: space-between; padding: 16px; }
.preview-table-heading input { max-width: 260px; }
.preview-money { color: #059669; }
.preview-pagination { padding: 12px 16px; display: flex; align-items: center; justify-content: flex-end; gap: 12px; font-size: 12px; color: var(--preview-muted); }
@keyframes preview-enter { from { opacity: .4; transform: translateY(5px); } to { opacity: 1; transform: translateY(0); } }
@media (max-width: 1279px) { .console-preview { grid-template-columns: 174px minmax(0, 1fr); } .preview-brand { font-size: 16px; } .preview-panel { padding: 16px; } .preview-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); } .preview-chart-grid { grid-template-columns: minmax(0, 1fr); } .preview-panel :deep(.lg\:grid-cols-2) { grid-template-columns: minmax(0, 1fr); } .preview-plaza { display: none; } }
@media (max-width: 767px) { .console-preview { grid-template-columns: minmax(0, 1fr); } .preview-sidebar { border-right: 0; } .preview-brand, .preview-theme { display: none; } .preview-tabs { flex-direction: row; padding: 8px; gap: 4px; border-bottom: 1px solid var(--preview-border); } .preview-tabs button { justify-content: center; flex: 1; gap: 5px; padding: 10px 4px; font-size: 12px; white-space: nowrap; } .preview-tabs svg { width: 15px; height: 15px; } .preview-topbar { padding: 12px; } .preview-topbar h3 { font-size: 17px; } .preview-balance, .preview-avatar, .preview-user { display: none; } .preview-notice { padding: 9px 12px; } .preview-panel { min-height: 560px; max-height: 720px; padding: 12px; } .preview-key-filters { grid-template-columns: 1fr 1fr; } .preview-key-filters > input { grid-column: 1 / -1; } .preview-summary { gap: 8px; } .preview-summary > div { padding: 12px; } .preview-summary strong { font-size: 17px; } .preview-stat-icon { display: none; } }
@media (max-width: 1535px) {
  .preview-chart-grid { grid-template-columns: minmax(0, 1fr); }
  .preview-panel :deep(.lg\:grid-cols-2) { grid-template-columns: minmax(0, 1fr); }
}
@media (prefers-reduced-motion: reduce) { .preview-panel { animation: none; } .preview-tabs button, .preview-row-actions button { transition: none; } }
@media (min-width: 1024px) {
  .console-preview { grid-template-columns: 240px minmax(0, 1fr); }
  .preview-brand { font-size: 18px; }
  .preview-panel :deep(.space-y-6 > :not([hidden]) ~ :not([hidden])) { margin-top: 16px; }
  .preview-panel :deep(.h-48) { height: 160px; }
  .preview-panel :deep(.w-48) { width: 160px; }
  .preview-panel { height: var(--preview-panel-height, 800px); min-height: 0; max-height: var(--preview-panel-height, 800px); padding: 20px; }
  .preview-chart-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
  .preview-panel :deep(.lg\:grid-cols-2) { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .preview-summary { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}
</style>
