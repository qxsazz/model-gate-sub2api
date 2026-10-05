<template>
  <AppLayout
    ><div class="achievement-admin space-y-6" :aria-busy="busy">
      <div class="ach-admin-toolbar">
        <div
          class="achievement-admin-tabs"
          role="tablist"
          aria-label="成就与签到管理"
        >
          <button
            v-for="t in tabs"
            :id="'achievement-tab-' + t.id"
            :key="t.id"
            :data-tab="t.id"
            type="button"
            role="tab"
            :aria-selected="tab === t.id"
            :class="{ active: tab === t.id }"
            @click="chooseTab(t.id)"
          >
            {{ t.label }}
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            aria-label="刷新数据"
            :disabled="busy"
            @click="refresh"
          >
            <Icon name="refresh" size="md" /></button
          ><button
            v-if="tab === 'rules'"
            type="button"
            class="btn btn-primary"
            :disabled="busy || !config"
            @click="save"
          >
            保存规则</button
          ><router-link to="/achievements" class="btn btn-secondary"
            >查看成就册</router-link
          >
        </div>
      </div>
      <p
        v-if="error"
        role="alert"
        class="text-sm text-red-600 dark:text-red-400"
      >
        {{ error }}
      </p>
      <p v-if="!config" role="status" class="py-12 text-center text-gray-500">
        正在读取成就规则…
      </p>
      <template v-else>
        <div v-show="tab === 'rules'" role="tabpanel" class="space-y-6">
          <section class="ach-admin-section">
            <h2>用户即时领取</h2>
            <div class="ach-admin-fields">
              <div class="ach-admin-field">
                <label for="daily-cash">每日签到金额奖励</label
                ><Select
                  id="daily-cash"
                  v-model="dailyMode"
                  :options="enabledOptions"
                  :disabled="busy"
                  aria-label="每日签到金额奖励"
                />
              </div>
              <div class="ach-admin-field">
                <label for="milestone-cash">成就金额奖励</label
                ><Select
                  id="milestone-cash"
                  v-model="milestoneMode"
                  :options="enabledOptions"
                  :disabled="busy"
                  aria-label="成就金额奖励"
                />
              </div>
              <div class="ach-admin-field">
                <label for="cash-scope">开放范围</label
                ><Select
                  id="cash-scope"
                  v-model="cashScope"
                  :options="[
                    { value: 'all', label: '所有符合条件的账户' },
                    { value: 'allowlist', label: '指定账户（测试或灰度）' },
                  ]"
                  :disabled="busy"
                  aria-label="奖励开放范围"
                />
              </div>
            </div>
            <label v-if="cashScope === 'allowlist'" class="ach-admin-field mt-4"
              >指定账户 ID<textarea
                v-model="allowlist"
                class="input"
                rows="2"
                :disabled="busy"
                placeholder="多个 ID 用逗号或空格分隔"
              />
            </label>
            <p class="ach-admin-note">
              用户签到或点击领取后立即入账，没有等待后台统一发放的奖励池。重复请求不重复入账，佩戴与领奖分别保存。
            </p>
            <p class="ach-admin-note">
              签到金额奖励面向开放范围内的正常账户，无需近期充值或消费。成就金额奖励仍需近
              30
              日有效充值或余额计费使用；管理员补签及授予属于单独的人工更正操作。
            </p>
          </section>
          <section class="ach-admin-section">
            <h2>实时发奖限制</h2>
            <div class="ach-admin-fields">
              <div class="ach-admin-field">
                <label for="budget-mode">日/月金额上限</label
                ><Select
                  id="budget-mode"
                  v-model="budgetMode"
                  :options="[
                    { value: 'off', label: '不设置日/月上限' },
                    { value: 'on', label: '启用日/月金额上限' },
                  ]"
                  :disabled="busy"
                  aria-label="日月金额上限"
                />
              </div>
              <label class="ach-admin-field"
                >每日金额上限（USD）<input
                  v-model.number="config.daily_budget"
                  class="input"
                  type="number"
                  min="0"
                  max="1000"
                  step=".01"
                  :disabled="busy || budgetMode === 'off'" /></label
              ><label class="ach-admin-field"
                >每月金额上限（USD）<input
                  v-model.number="config.monthly_budget"
                  class="input"
                  type="number"
                  min="0"
                  max="10000"
                  step=".01"
                  :disabled="busy || budgetMode === 'off'"
              /></label>
            </div>
            <p class="ach-admin-note">
              上限是可选实时风控，不是后台结算周期。开启后超限会立即返回未发放原因，不积累到后台等待发放。
            </p>
          </section>
          <section class="ach-admin-section">
            <h2>VIP 每日签到权益</h2>
            <div class="ach-admin-table-wrap">
              <table class="ach-admin-table">
                <thead>
                  <tr>
                    <th>等级</th>
                    <th>单日金额</th>
                    <th>入账方式</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="(value, index) in config.daily_rewards"
                    :key="index"
                  >
                    <td>{{ index ? 'VIP ' + index : '普通会员' }}</td>
                    <td>{{ money(value) }}</td>
                    <td>即时到账</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="ach-admin-note">
              按实际充值成长等级计算，人工展示等级不提高金额。历史日期按北京时间日终等级核验，当日按当前等级核验，不用当前等级代替历史等级。
            </p>
          </section>
        </div>
        <div v-show="tab === 'users'" role="tabpanel" class="space-y-6">
          <form
            class="flex flex-wrap items-end gap-3"
            @submit.prevent="loadUser"
          >
            <label class="ach-admin-field"
              >用户 ID<input
                v-model.number="userId"
                class="input"
                data-user-id
                type="number"
                min="1"
                :disabled="busy" /></label
            ><button
              class="btn btn-secondary"
              data-load-user
              type="submit"
              :disabled="busy || !validUser"
            >
              查看账户
            </button>
          </form>
          <p v-if="!state" class="py-10 text-center text-gray-500">
            请选择账户查看签到与成就记录。
          </p>
          <template v-else>
            <div class="ach-admin-summary">
              <div>
                <span>用户</span
                ><strong
                  >{{ state.user.username || state.user.email }} · ID
                  {{ loadedId }}</strong
                >
              </div>
              <div>
                <span>余额</span
                ><strong>{{ money(state.user.balance) }}</strong>
              </div>
              <div>
                <span>最长连续签到</span><strong>{{ state.longest }} 天</strong>
              </div>
              <div>
                <span>累计签到</span><strong>{{ state.total_days }} 天</strong>
              </div>
            </div>
            <label class="ach-admin-field"
              >操作原因<input
                v-model="reason"
                class="input"
                data-operation-reason
                maxlength="500"
                :disabled="busy"
                placeholder="请说明核验或更正原因"
            /></label>
            <p class="ach-admin-note">
              可用补签卡
              {{ state.card_balance ?? 0 }}
              张。活动勋章奖励补签卡，系列集齐后另领金额。
            </p>
            <section class="ach-admin-section">
              <h2>补签并即时补发</h2>
              <div class="flex flex-wrap items-end gap-3">
                <label class="ach-admin-field"
                  >补签日期<input
                    v-model="backfillDate"
                    class="input"
                    data-backfill-date
                    type="date"
                    :max="latestDate"
                    :disabled="busy || !userMatches"
                    @change="preview = null" /></label
                ><button
                  type="button"
                  class="btn btn-secondary"
                  data-preview-backfill
                  :disabled="busy || !userMatches || !backfillDate"
                  @click="previewBackfill"
                >
                  核验历史权益</button
                ><button
                  type="button"
                  class="btn btn-primary"
                  data-backfill
                  :disabled="
                    busy ||
                    !userMatches ||
                    !preview ||
                    preview.existing ||
                    !preview.policy_known ||
                    preview.day !== backfillDate ||
                    !reason.trim()
                  "
                  @click="requestBackfill"
                >
                  确认补签与补发
                </button>
              </div>
              <p v-if="preview" class="ach-admin-note">
                {{
                  preview.existing
                    ? '该日已有记录，不能重复补发'
                    : preview.policy_known
                      ? '历史 VIP ' +
                        preview.tier +
                        ' · 应发 ' +
                        money(preview.gross)
                      : '该日历史权益无法完整核验，不能补发'
                }}{{
                  preview.reason ? ' · ' + previewReason(preview.reason) : ''
                }}
              </p>
              <p class="ach-admin-note">
                补签与金额在同一事务内保存，并重算连续签到。已有日期不重复入账；如有待追回金额，补发可能先抵扣，以回执净到账为准。
              </p>
            </section>
            <section class="ach-admin-section">
              <h2>成就与徽章管理</h2>
              <div class="ach-admin-fields mb-4">
                <div class="ach-admin-field">
                  <label for="grant-mode">授予时的奖励</label
                  ><Select
                    id="grant-mode"
                    v-model="grantMode"
                    :options="[
                      { value: 'unlock', label: '授予徽章，由用户领取金额' },
                      { value: 'with_reward', label: '授予并立即发放对应奖励' },
                    ]"
                    :disabled="busy"
                    aria-label="授予时的奖励"
                  />
                </div>
                <div class="ach-admin-field">
                  <label for="revoke-mode">取消时的奖励</label
                  ><Select
                    id="revoke-mode"
                    v-model="revokeMode"
                    :options="[
                      {
                        value: 'badge_only',
                        label: '只取消徽章，保留已发金额',
                      },
                      { value: 'reclaim', label: '同时追回该枚已发奖励' },
                    ]"
                    :disabled="busy"
                    aria-label="取消时的奖励"
                  />
                </div>
              </div>
              <div class="ach-admin-table-wrap">
                <table class="ach-admin-table ach-admin-medals">
                  <thead>
                    <tr>
                      <th>徽章</th>
                      <th>进度</th>
                      <th>状态</th>
                      <th>奖励</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="m in state.medals" :key="m.key">
                      <td>
                        <strong>{{ m.name }}</strong
                        ><span class="ach-admin-note block">{{ m.key }}</span>
                      </td>
                      <td>
                        {{ m.progress.toLocaleString() }} /
                        {{ m.target.toLocaleString() }}
                      </td>
                      <td>
                        {{
                          m.manual === 'revoked'
                            ? '管理员已取消'
                            : m.manual === 'granted'
                              ? '管理员授予'
                              : m.unlocked
                                ? '已解锁'
                                : m.preview
                                  ? '篇章预告'
                                  : '尚未达成'
                        }}<span
                          v-if="state.equipment === m.key"
                          class="ach-admin-note block"
                          >正在佩戴</span
                        >
                      </td>
                      <td>
                        {{
                          m.card_reward
                            ? m.card_reward + ' 张补签卡'
                            : money(m.reward)
                        }}<span class="ach-admin-note block">{{
                          m.card_reward
                            ? m.card_claim
                              ? m.card_claim.reclaimed
                                ? '已领取 · 有卡片追回'
                                : '已领取'
                              : '未领取'
                            : m.claim?.revoked_at
                              ? '已追回'
                              : m.claim
                                ? '已领取'
                                : '未领取'
                        }}</span>
                        <span v-if="m.card_claim" class="ach-admin-note block"
                          >已用 {{ m.card_claim.used }} · 已追回
                          {{ m.card_claim.reclaimed }} · 可用
                          {{
                            m.card_claim.amount -
                            m.card_claim.used -
                            m.card_claim.reclaimed
                          }}</span
                        >
                      </td>
                      <td>
                        <div class="flex flex-wrap gap-2">
                          <button
                            :data-grant="m.key"
                            type="button"
                            class="btn btn-secondary btn-sm"
                            :disabled="busy || !userMatches || !reason.trim()"
                            @click="requestMedal('grant', m)"
                          >
                            授予</button
                          ><button
                            :data-revoke="m.key"
                            type="button"
                            class="btn btn-secondary btn-sm"
                            :disabled="
                              busy ||
                              !userMatches ||
                              !reason.trim() ||
                              (!m.unlocked && !m.manual)
                            "
                            @click="requestMedal('revoke', m)"
                          >
                            取消</button
                          ><button
                            v-if="m.manual"
                            type="button"
                            class="btn btn-secondary btn-sm"
                            :disabled="busy || !userMatches || !reason.trim()"
                            @click="requestMedal('restore', m)"
                          >
                            恢复自动判定
                          </button>
                        </div>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <p class="ach-admin-note">
                取消后立即停止佩戴及领取，直到授予或恢复自动判定。每枚金额终身只发一次，重新授予不重复发奖。操作原因及金额处理均记录审计。
              </p>
            </section>
            <section v-if="state.series?.length" class="ach-admin-section">
              <h2>系列收集与金额回执</h2>
              <div class="ach-admin-table-wrap">
                <table class="ach-admin-table">
                  <thead>
                    <tr>
                      <th>系列 / 进度</th>
                      <th>系列总额</th>
                      <th>旧活动已领</th>
                      <th>本次原额 / 净到账 / 抵扣</th>
                      <th>状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="s in state.series" :key="s.key">
                      <td>{{ s.name }} · {{ s.collected }}/{{ s.total }}</td>
                      <td>{{ money(s.reward) }}</td>
                      <td>
                        {{
                          money(s.claim?.prior_amount ?? s.prior_amount ?? 0)
                        }}
                      </td>
                      <td>
                        {{
                          s.claim
                            ? money(s.claim.gross) +
                              ' / ' +
                              money(s.claim.net) +
                              ' / ' +
                              money(s.claim.offset_amount)
                            : '—'
                        }}
                      </td>
                      <td>
                        {{
                          s.claim?.revoked_at
                            ? '已追回，领取资格保留'
                            : s.claim
                              ? '已领取'
                              : s.preview
                                ? '后续篇章待开放'
                                : s.unlocked
                                  ? '已集齐，未领取'
                                  : '未集齐'
                        }}
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </template>
        </div>
        <div v-show="tab === 'audit'" role="tabpanel">
          <div class="ach-admin-table-wrap">
            <table class="ach-admin-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>操作人 / 账户</th>
                  <th>操作</th>
                  <th>徽章 / 日期</th>
                  <th>金额 / 补签卡回执</th>
                  <th>原因</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="r in audit" :key="r.id">
                  <td>{{ r.created_at }}</td>
                  <td>{{ r.actor_id }} / {{ r.user_id }}</td>
                  <td>{{ actionNames[r.action] || r.action }}</td>
                  <td>{{ r.key || r.day || '—' }}</td>
                  <td>
                    {{ money(r.response?.gross || 0) }} /
                    {{ money(r.response?.net || 0) }}
                    <span
                      v-if="r.response?.cards_awarded"
                      class="ach-admin-note block"
                      >发放 {{ r.response.cards_awarded }} 张补签卡</span
                    >
                    <span
                      v-if="r.response?.cards_reclaimed"
                      class="ach-admin-note block"
                      >回收 {{ r.response.cards_reclaimed }} 张补签卡</span
                    >
                    <span
                      v-if="
                        r.response?.series_recovered ||
                        r.response?.series_debt_added
                      "
                      class="ach-admin-note block"
                      >系列扣回 {{ money(r.response.series_recovered || 0) }} ·
                      待追回
                      {{ money(r.response.series_debt_added || 0) }}</span
                    >
                  </td>
                  <td class="ach-admin-reason">{{ r.reason }}</td>
                </tr>
                <tr v-if="!audit.length">
                  <td colspan="6" class="text-center text-gray-500">
                    暂无操作记录
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>
      <ConfirmDialog
        :show="!!confirmation"
        :title="confirmation?.title || ''"
        :message="confirmation?.message || ''"
        :danger="confirmation?.action === 'revoke'"
        @confirm="confirmAction"
        @cancel="confirmation = null"
      /></div
  ></AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { formatMoneyFixed } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import {
  getAchievementConfig,
  saveAchievementConfig,
  getAdminAchievements,
  previewAchievementBackfill,
  adminAchievementAction,
  getAchievementAudit,
  type AchievementConfig,
  type AdminAchievementState,
  type BackfillPreview,
  type AchievementAdminBody,
  type AchievementAudit,
  type Medal,
} from '@/api/achievements'
const app = useAppStore(),
  config = ref<AchievementConfig | null>(null),
  tab = ref('rules'),
  busy = ref(false),
  error = ref(''),
  allowlist = ref(''),
  userId = ref(0),
  loadedId = ref(0),
  state = ref<AdminAchievementState | null>(null),
  reason = ref(''),
  backfillDate = ref(''),
  preview = ref<BackfillPreview | null>(null),
  grantMode = ref('unlock'),
  revokeMode = ref('badge_only'),
  audit = ref<AchievementAudit[]>([])
const tabs = [
    { id: 'rules', label: '奖励规则' },
    { id: 'users', label: '用户与补签' },
    { id: 'audit', label: '操作记录' },
  ],
  enabledOptions = [
    { value: 'on', label: '开启 · 用户操作即时到账' },
    { value: 'off', label: '关闭金额奖励' },
  ]
const dailyMode = computed({
  get: () => (config.value?.cash_enabled ? 'on' : 'off'),
  set: (v: string) => {
    if (config.value) config.value.cash_enabled = v === 'on'
  },
})
const milestoneMode = computed({
  get: () => (config.value?.milestone_cash_enabled ? 'on' : 'off'),
  set: (v: string) => {
    if (config.value) config.value.milestone_cash_enabled = v === 'on'
  },
})
const budgetMode = computed({
  get: () => (config.value?.budget_enabled ? 'on' : 'off'),
  set: (v: string) => {
    if (config.value) config.value.budget_enabled = v === 'on'
  },
})
const cashScope = computed({
  get: () => config.value?.cash_scope || 'all',
  set: (v: string) => {
    if (config.value && (v === 'all' || v === 'allowlist'))
      config.value.cash_scope = v
  },
})
const validUser = computed(
    () => Number.isSafeInteger(userId.value) && userId.value > 0,
  ),
  userMatches = computed(
    () => validUser.value && userId.value === loadedId.value && !!state.value,
  )
const latestDate = computed(() => (state.value ? state.value.date : ''))
type Pending = {
  id: number
  title: string
  message: string
  action: 'backfill' | 'grant' | 'revoke' | 'restore'
  body: AchievementAdminBody
}
const confirmation = ref<Pending | null>(null),
  requestKeys = new Map<string, string>()
const actionNames: Record<string, string> = {
  backfill: '补签与补发',
  grant: '授予成就',
  revoke: '取消成就',
  restore: '恢复自动判定',
}
const money = (v: number) => formatMoneyFixed(v, '$')
const err = (e: unknown) => {
  const v = e as {
    message?: string
    response?: { data?: { message?: string } }
  }
  return v.response?.data?.message || v.message || '暂时无法连接'
}
const previewReason = (v: string) =>
  ({
    account_not_created: '该日账户尚未注册',
    policy_unknown: '缺少历史政策',
    date_invalid: '日期不在可补签范围',
    already_signed: '该日已有记录',
  })[v] || v
async function run(fn: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = err(e)
    app.showError(error.value)
  } finally {
    busy.value = false
  }
}
async function load() {
  await run(async () => {
    config.value = await getAchievementConfig()
    allowlist.value = (config.value.cash_allowlist || []).join(', ')
  })
}
async function save() {
  if (!config.value) return
  await run(async () => {
    const ids = allowlist.value.trim()
      ? allowlist.value
          .trim()
          .split(/[,，\s]+/)
          .map(Number)
      : []
    if (ids.some((id) => !Number.isSafeInteger(id) || id <= 0))
      throw new Error('账户 ID 必须为正整数')
    await saveAchievementConfig({ ...config.value!, cash_allowlist: ids })
    app.showSuccess('即时发奖规则已保存')
  })
}
async function loadUser() {
  if (!validUser.value) return
  const id = userId.value
  await run(async () => {
    state.value = null
    preview.value = null
    const result = await getAdminAchievements(id)
    if (userId.value === id) {
      state.value = result
      loadedId.value = id
    }
  })
}
async function previewBackfill() {
  if (!userMatches.value) return
  const id = loadedId.value,
    date = backfillDate.value
  await run(async () => {
    const result = await previewAchievementBackfill(id, date)
    if (
      userMatches.value &&
      loadedId.value === id &&
      backfillDate.value === date
    )
      preview.value = result
  })
}
function queue(
  action: Pending['action'],
  title: string,
  message: string,
  body: Omit<AchievementAdminBody, 'request_key'>,
) {
  if (!userMatches.value || !reason.value.trim()) return
  const identity = JSON.stringify({ id: loadedId.value, action, ...body })
  let key = requestKeys.get(identity)
  if (!key) {
    key = crypto.randomUUID()
    requestKeys.set(identity, key)
  }
  confirmation.value = {
    id: loadedId.value,
    action,
    title,
    message,
    body: { ...body, request_key: key },
  }
}
function requestBackfill() {
  if (!preview.value || preview.value.existing || !preview.value.policy_known)
    return
  queue(
    'backfill',
    '确认补签与即时补发',
    '用户 ID ' +
      loadedId.value +
      '，补签 ' +
      preview.value.day +
      '，按历史 VIP ' +
      preview.value.tier +
      ' 发放 ' +
      money(preview.value.gross) +
      '。已有记录不重复发奖，可能抵扣待追回金额。',
    {
      date: preview.value.day,
      reason: reason.value.trim(),
      expected_gross: preview.value.gross,
      expected_tier: preview.value.tier,
    },
  )
}
function requestMedal(action: 'grant' | 'revoke' | 'restore', m: Medal) {
  const reward = action === 'grant' && grantMode.value === 'with_reward',
    reclaim = action === 'revoke' && revokeMode.value === 'reclaim'
  queue(
    action,
    actionNames[action],
    '用户 ID ' +
      loadedId.value +
      '，' +
      m.name +
      '（' +
      m.key +
      '）。' +
      (reward
        ? '同时立即发放 ' +
          (m.card_reward ? m.card_reward + ' 张补签卡' : money(m.reward)) +
          '；已发过则不重复发放。'
        : reclaim
          ? m.card_reward
            ? '追回该枚未使用补签卡及已领取的对应系列金额；已使用卡片及补签记录保留。'
            : '同时追回该枚已发奖励。'
          : '变更徽章状态，保留已有领取记录。'),
    {
      key: m.key,
      reason: reason.value.trim(),
      grant_reward: reward,
      reclaim_reward: reclaim,
    },
  )
}
async function confirmAction() {
  const p = confirmation.value
  confirmation.value = null
  if (!p) return
  if (!userMatches.value || p.id !== loadedId.value) {
    app.showError('用户已更改，请重新查询并确认')
    return
  }
  await run(async () => {
    const result = await adminAchievementAction(p.id, p.action, p.body)
    app.showSuccess(
      result.replayed
        ? '该请求已处理，返回原回执'
        : result.existing
          ? '该日已签到，没有重复补发'
          : p.action === 'revoke' && p.body.reclaim_reward
            ? '取消已完成，扣回 ' +
              money((result.recovered || 0) + (result.series_recovered || 0)) +
              '，新增待追回 ' +
              money(
                (result.debt_added || 0) + (result.series_debt_added || 0),
              ) +
              (result.cards_reclaimed
                ? '，回收 ' + result.cards_reclaimed + ' 张补签卡'
                : '')
            : actionNames[p.action] +
              '已完成' +
              (result.cards_awarded
                ? '，发放 ' + result.cards_awarded + ' 张补签卡'
                : result.gross
                  ? '，原额 ' +
                    money(result.gross) +
                    '，净到账 ' +
                    money(result.net)
                  : ''),
    )
    state.value = await getAdminAchievements(p.id)
    preview.value = null
    requestKeys.forEach((v, k) => {
      if (v === p.body.request_key) requestKeys.delete(k)
    })
  })
}
async function chooseTab(id: string) {
  tab.value = id
  if (id === 'audit')
    await run(async () => {
      audit.value = await getAchievementAudit()
    })
}
async function refresh() {
  if (tab.value === 'rules') await load()
  else if (tab.value === 'users') await loadUser()
  else await chooseTab('audit')
}
onMounted(load)
</script>
<style scoped>
.achievement-admin {
  min-width: 0;
}
.ach-admin-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.achievement-admin-tabs {
  display: flex;
  gap: 24px;
  border-bottom: 1px solid var(--mg-line-warm);
}
.achievement-admin-tabs button {
  padding: 12px 0;
  border-bottom: 2px solid transparent;
  font-size: 14px;
  white-space: nowrap;
  color: var(--mg-muted);
}
.achievement-admin-tabs button.active {
  border-color: var(--mg-gold-700);
  color: var(--mg-ink-900);
  font-weight: 600;
}
.achievement-admin-tabs button:focus-visible {
  outline: 2px solid var(--mg-gold-700);
  outline-offset: 4px;
}
.ach-admin-section {
  padding: 20px 0;
  border-top: 1px solid var(--mg-line-warm);
}
.ach-admin-section h2 {
  margin-bottom: 16px;
  font-size: 16px;
  font-weight: 600;
}
.ach-admin-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 16px;
}
.ach-admin-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  min-width: 0;
  max-width: 640px;
}
.ach-admin-field .input {
  width: 100%;
}
.ach-admin-note {
  margin-top: 10px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--mg-muted);
}
.ach-admin-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--mg-line-warm);
  border-radius: 6px;
  background: var(--mg-surface);
}
.ach-admin-table {
  width: 100%;
  text-align: left;
  font-size: 13px;
}
.ach-admin-table th {
  white-space: nowrap;
  font-weight: 500;
}
.ach-admin-table th,
.ach-admin-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--mg-line-warm);
}
.ach-admin-table tbody tr:last-child td {
  border-bottom: 0;
}
.ach-admin-medals {
  min-width: 950px;
}
.ach-admin-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 20px;
  padding: 18px 0;
  border-block: 1px solid var(--mg-line-warm);
}
.ach-admin-summary span {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--mg-muted);
}
.ach-admin-summary strong {
  font-size: 15px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.ach-admin-reason {
  min-width: 160px;
  max-width: 400px;
  overflow-wrap: anywhere;
}
@media (max-width: 640px) {
  .achievement-admin-tabs {
    gap: 20px;
  }
  .achievement-admin-tabs button {
    font-size: 13px;
  }
  .ach-admin-fields {
    grid-template-columns: minmax(0, 1fr);
  }
  .ach-admin-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .ach-admin-table th,
  .ach-admin-table td {
    padding: 10px 12px;
  }
}
</style>
