<template>
  <AppLayout>
    <div class="vip-admin space-y-6" :aria-busy="busy">
      <div class="vip-toolbar">
        <div role="tablist" aria-label="VIP 权益设置" class="vip-tabs">
          <button
            v-for="tab in tabs"
            :id="`vip-tab-${tab.id}`"
            :key="tab.id"
            type="button"
            role="tab"
            :aria-selected="activeTab === tab.id"
            :aria-controls="`vip-panel-${tab.id}`"
            :tabindex="activeTab === tab.id ? 0 : -1"
            :data-tab="tab.id"
            :class="{ active: activeTab === tab.id }"
            @click="activeTab = tab.id"
            @keydown="navigateTabs($event, tab.id)"
          >
            {{ tab.label }}
          </button>
        </div>
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="btn btn-secondary"
            title="刷新数据"
            aria-label="刷新数据"
            :disabled="busy"
            @click="requestRefresh"
          >
            <Icon name="refresh" size="md" />
          </button>
          <button
            v-if="activeTab !== 'users'"
            type="button"
            data-save-rules
            class="btn btn-primary"
            :disabled="busy || !rules"
            @click="save"
          >
            <Icon name="checkCircle" size="sm" />保存规则
          </button>
          <router-link v-else to="/admin/users" class="btn btn-secondary"
            >用户管理</router-link
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
      <p
        v-if="!rules"
        role="status"
        class="py-12 text-center text-sm text-gray-500"
      >
        {{ busy ? '正在加载权益配置…' : '权益配置加载失败，请重试' }}
      </p>
      <template v-else>
        <div
          id="vip-panel-rules"
          v-show="activeTab === 'rules'"
          role="tabpanel"
          aria-labelledby="vip-tab-rules"
          class="space-y-6"
        >
          <section class="vip-section">
            <h2>功能与计量</h2>
            <div class="vip-fields">
              <label class="vip-field"
                >自动权益<span class="flex h-10 items-center gap-2"
                  ><input
                    v-model="rules.enabled"
                    type="checkbox"
                    :disabled="busy"
                  />启用自动权益</span
                ></label
              >
              <label class="vip-field"
                >充值加赠<span class="flex h-10 items-center gap-2"
                  ><input
                    v-model="rules.recharge_bonus_enabled"
                    type="checkbox"
                    :disabled="busy"
                  />按有效 VIP 等级加赠</span
                ></label
              >
              <label class="vip-field"
                >专属组门槛<input
                  v-model.number="rules.access_threshold"
                  type="number"
                  min="100"
                  class="input"
                  :disabled="busy"
              /></label>
              <label class="vip-field"
                >计量单位<input :value="rules.currency" readonly class="input"
              /></label>
            </div>
            <div class="vip-fields mt-4">
              <label
                v-for="(_, currency) in rules.exchange_rates"
                :key="currency"
                class="vip-field"
                >{{ currency }} → USD<input
                  v-model.number="rules.exchange_rates[currency]"
                  type="number"
                  min="0.000001"
                  step="0.000001"
                  class="input"
                  :disabled="busy"
              /></label>
              <div class="vip-field">
                <label for="vip-new-currency">新增币种</label>
                <div class="flex gap-2">
                  <input
                    id="vip-new-currency"
                    v-model="newCurrency"
                    maxlength="3"
                    class="input min-w-0"
                    :disabled="busy"
                  /><button
                    type="button"
                    class="btn btn-secondary"
                    :disabled="busy"
                    @click="addCurrency"
                  >
                    添加
                  </button>
                </div>
              </div>
            </div>
            <p class="vip-note">
              换算仅用于累计充值成长，不包含余额加赠。新订单保存下单时的换算值；未配置币种在启用
              VIP 后无法创建余额充值订单，历史订单不自动回填。
            </p>
          </section>
          <section class="vip-section">
            <h2>等级阶梯</h2>
            <div
              v-if="rules.daily_rewards?.length === 6"
              class="vip-fields mb-4"
            >
              <label class="vip-field"
                >普通会员每日签到金额（USD）<input
                  v-model.number="rules.daily_rewards[0]"
                  data-daily-reward="0"
                  type="number"
                  min="0"
                  max="1000"
                  step="0.01"
                  class="input"
                  :disabled="busy"
              /></label>
              <p class="vip-note">
                与成就与签到共用一份金额配置；保存后两处同步，已有签到回执不重算。
              </p>
            </div>
            <div class="vip-table-wrap">
              <table class="vip-table">
                <thead>
                  <tr>
                    <th>等级</th>
                    <th>累计充值</th>
                    <th>并发目标</th>
                    <th>RPM 目标</th>
                    <th>邀请返利 %</th>
                    <th>充值加赠 %</th>
                    <th>每日签到 USD</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="tier in rules.tiers" :key="tier.level">
                    <td class="whitespace-nowrap font-medium">
                      VIP {{ tier.level }}
                    </td>
                    <td>
                      <input
                        v-model.number="tier.threshold"
                        :data-threshold="tier.level"
                        :aria-label="`VIP ${tier.level} 累计充值门槛`"
                        type="number"
                        :readonly="tier.level === 1"
                        min="100"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                    <td>
                      <input
                        v-model.number="tier.concurrency"
                        :aria-label="`VIP ${tier.level} 并发目标`"
                        type="number"
                        min="1"
                        max="1000"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                    <td>
                      <input
                        v-model.number="tier.rpm"
                        :aria-label="`VIP ${tier.level} RPM 目标`"
                        type="number"
                        min="0"
                        max="1000"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                    <td>
                      <input
                        v-model.number="tier.rebate_percent"
                        :aria-label="`VIP ${tier.level} 邀请返利百分比`"
                        type="number"
                        min="0"
                        max="10"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                    <td>
                      <input
                        v-model.number="tier.recharge_bonus_percent"
                        :aria-label="`VIP ${tier.level} 充值加赠百分比`"
                        type="number"
                        min="0"
                        max="5"
                        step="0.1"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                    <td v-if="rules.daily_rewards?.length === 6">
                      <input
                        v-model.number="rules.daily_rewards[tier.level]"
                        :data-daily-reward="tier.level"
                        :aria-label="`VIP ${tier.level} 每日签到金额`"
                        type="number"
                        min="0"
                        max="1000"
                        step="0.01"
                        class="input"
                        :disabled="busy"
                      />
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="vip-note">
              并发不会降低原基础额度。RPM 为 0
              时沿用原有限流；当前不限流基础不被自动改为有限额度。
            </p>
            <p class="vip-note">
              充值加赠最高 5%，普通会员为 0%；按下单时的有效 VIP
              等级计算，管理员指定等级同样适用，旧的单独身份标识不改变权益。关闭此选项或
              VIP 时，充值沿用原支付设置，旧订单不重算。
            </p>
          </section>
        </div>
        <div
          id="vip-panel-groups"
          v-show="activeTab === 'groups'"
          role="tabpanel"
          aria-labelledby="vip-tab-groups"
        >
          <div class="vip-table-wrap vip-group-table">
            <table class="vip-table">
              <thead>
                <tr>
                  <th>分组</th>
                  <th>属性 / 基础倍率</th>
                  <th>VIP 准入</th>
                  <th>倍率下限</th>
                  <th v-for="n in 5" :key="n">VIP {{ n }} 减免</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="group in groups" :key="group.id">
                  <td class="font-medium">{{ group.name }}</td>
                  <td class="whitespace-nowrap text-gray-500">
                    {{
                      excluded(group)
                        ? '私人 / 订阅 / 测试'
                        : group.is_exclusive
                          ? '专属'
                          : '公开'
                    }}
                    · {{ group.rate_multiplier }}
                  </td>
                  <td>
                    <input
                      v-model="rule(group.id).access"
                      type="checkbox"
                      :aria-label="`${group.name} VIP 准入`"
                      :disabled="busy || !group.is_exclusive || excluded(group)"
                    />
                  </td>
                  <td>
                    <input
                      v-model.number="rule(group.id).floor"
                      :aria-label="`${group.name} 倍率下限`"
                      type="number"
                      min="0"
                      :max="group.rate_multiplier"
                      step="0.001"
                      :disabled="busy || group.is_exclusive || excluded(group)"
                      class="input"
                    />
                  </td>
                  <td v-for="n in 5" :key="n">
                    <input
                      v-model.number="rule(group.id).discounts[n - 1]"
                      :data-discount="`${group.id}-${n}`"
                      :aria-label="`${group.name} VIP ${n} 减免`"
                      type="number"
                      min="0"
                      max="0.075"
                      step="0.001"
                      :disabled="busy || group.is_exclusive || excluded(group)"
                      class="input"
                    />
                  </td>
                </tr>
                <tr v-if="!groups.length">
                  <td colspan="9" class="text-center text-gray-500">
                    暂无分组
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
        <div
          id="vip-panel-users"
          v-show="activeTab === 'users'"
          role="tabpanel"
          aria-labelledby="vip-tab-users"
          class="space-y-6"
        >
          <form
            class="flex flex-wrap items-end gap-3"
            @submit.prevent="loadUser"
          >
            <label class="vip-field vip-user-picker"
              >选择用户<Select
                :model-value="userId || null"
                :options="userOptions"
                :searchable="true"
                :remote="true"
                :loading="searching"
                search-placeholder="搜索用户名或邮箱"
                placeholder="选择用户查看 VIP 权益"
                :disabled="busy"
                @search="searchUsers"
                @change="chooseUser"
            /></label>
            <label class="vip-field"
              >或输入用户 ID<input
                v-model.number="userId"
                data-user-id
                type="number"
                min="1"
                class="input"
                :disabled="busy"
            /></label>
            <button
              type="submit"
              data-load-user
              class="btn btn-secondary"
              :disabled="busy || !validUserId"
            >
              <Icon name="search" size="sm" />查看权益
            </button>
          </form>
          <p v-if="!userState" class="py-12 text-center text-sm text-gray-500">
            请选择用户查看有效权益
          </p>
          <template v-else>
            <div data-user-summary class="vip-user-summary">
              <div>
                <span>当前用户</span
                ><strong>{{
                  userState.user?.username ||
                  userState.user?.email ||
                  '用户 ' + loadedUserId
                }}</strong
                ><small>ID {{ loadedUserId }}</small>
              </div>
              <div>
                <span>当前有效等级</span
                ><strong>{{ gradeName(userState.tier.level) }}</strong>
              </div>
              <div>
                <span>权益来源</span
                ><strong>{{
                  userState.tier_source === 'manual'
                    ? '管理员指定'
                    : userState.enabled === false
                      ? 'VIP 权益已关闭'
                      : '充值成长自动计算'
                }}</strong>
              </div>
              <div>
                <span>充值成长等级</span
                ><strong>{{
                  gradeName(
                    userState.growth_tier?.level ?? userState.tier.level,
                  )
                }}</strong
                ><small>累计有效充值 ${{ money(userState.total) }}</small>
              </div>
              <div>
                <span>指定等级有效期</span
                ><strong>{{
                  userState.level_override?.expires_at
                    ? new Date(
                        userState.level_override.expires_at,
                      ).toLocaleString()
                    : userState.level_override
                      ? '持续有效，直到恢复自动'
                      : '自动随充值成长变化'
                }}</strong>
              </div>
            </div>
            <section class="vip-section">
              <div class="vip-section-heading">
                <div>
                  <h2>指定 VIP 等级</h2>
                  <p class="vip-note">
                    选择一个等级，即统一应用该等级的签到、并发、返利、充值加赠与分组权益。
                  </p>
                </div>
                <button
                  type="button"
                  data-restore-level
                  class="btn btn-secondary"
                  :disabled="busy || !userMatches"
                  @click="requestRestoreLevel"
                >
                  恢复自动等级
                </button>
              </div>
              <p
                v-if="userState.overrides?.some((o) => o.benefit !== 'tier')"
                class="vip-legacy-note"
              >
                此账户存在旧版单项覆盖。保存指定等级或恢复自动时，会统一移除这些旧覆盖，并保留审计记录。
              </p>
              <p v-if="!rules.enabled" class="vip-legacy-note">
                VIP 权益总开关当前关闭；指定等级可保存，权益在开关开启后生效。
              </p>
              <form class="vip-level-form" @submit.prevent="requestLevelChange">
                <div class="vip-field">
                  <label for="vip-level">指定等级</label
                  ><Select
                    id="vip-level"
                    v-model="selectedLevel"
                    data-vip-level
                    :options="levelOptions"
                    :disabled="busy"
                    aria-label="指定VIP等级"
                  />
                </div>
                <label class="vip-field"
                  >到期时间（本地时间，可选）<input
                    v-model="expiry"
                    type="datetime-local"
                    class="input"
                    :disabled="busy"
                /></label>
                <label class="vip-field"
                  >操作原因<input
                    v-model="levelReason"
                    data-level-reason
                    class="input"
                    placeholder="至少三个字，便于审计"
                    :disabled="busy"
                /></label>
                <button
                  type="submit"
                  data-save-level
                  class="btn btn-primary"
                  :disabled="busy || !userMatches"
                >
                  保存等级
                </button>
              </form>
              <div class="vip-benefit-preview" aria-label="对应等级权益预览">
                <div>
                  <span>并发权益目标</span
                  ><strong>{{
                    selectedTier?.concurrency || '基础账号额度'
                  }}</strong>
                </div>
                <div>
                  <span>RPM 权益目标</span
                  ><strong>{{ selectedTier?.rpm || '沿用基础限流' }}</strong>
                </div>
                <div>
                  <span>邀请返利</span
                  ><strong>{{ selectedTier?.rebate_percent || 0 }}%</strong>
                </div>
                <div>
                  <span>充值加赠</span
                  ><strong>{{
                    userState.rules?.recharge_bonus_enabled === false
                      ? '未启用'
                      : (selectedTier?.recharge_bonus_percent || 0) + '%'
                  }}</strong>
                </div>
                <div>
                  <span>每日签到权益</span
                  ><strong
                    >${{
                      money(
                        userState.rules?.daily_rewards?.[selectedLevel] ??
                        rules.daily_rewards?.[selectedLevel] ??
                        0
                      )
                    }}</strong
                  >
                </div>
              </div>
              <p class="vip-note">
                指定等级优先于自动等级；到期或恢复自动后按真实充值成长重新计算。基础账号限额更宽松时保留原额度；私人组、订阅组和已有定制价仍按原权限处理。改级不修改充值本金，也不重复发放已有签到或充值达标奖励。
              </p>
            </section>
            <section class="vip-section">
              <h2>当前有效权益</h2>
              <div class="vip-effective-summary">
                <span
                  >并发 <strong>{{ userState.concurrency }}</strong></span
                ><span
                  >RPM <strong>{{ userState.rpm || '不限' }}</strong></span
                ><span
                  >邀请返利
                  <strong>{{ userState.rebate_percent }}%</strong></span
                ><span
                  >每日签到
                  <strong
                    >${{
                      money(
                        userState.rules?.daily_rewards?.[
                          userState.tier.level
                        ] ??
                        rules.daily_rewards?.[userState.tier.level] ??
                        0
                      )
                    }}</strong
                  ></span
                >
              </div>
              <div v-if="userState.groups?.length" class="vip-table-wrap mt-4">
                <table class="vip-table">
                  <thead>
                    <tr>
                      <th>分组</th>
                      <th>当前准入</th>
                      <th>当前有效倍率</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="g in userState.groups" :key="g.id">
                      <td>{{ g.name }}</td>
                      <td>{{ g.granted ? '可使用' : '未获准入' }}</td>
                      <td>{{ g.rate }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
            <section class="vip-section">
              <details>
                <summary class="cursor-pointer text-sm font-medium">
                  历史充值初始确认
                </summary>
                <form
                  class="mt-4 flex flex-wrap items-end gap-3"
                  @submit.prevent="requestOpening"
                >
                  <label class="vip-field"
                    >确认金额<input
                      v-model.number="openingAmount"
                      data-opening-amount
                      type="number"
                      min="0"
                      class="input"
                      :disabled="busy"
                  /></label>
                  <label class="vip-field"
                    >核对原因<input
                      v-model="openingReason"
                      data-opening-reason
                      class="input"
                      :disabled="busy"
                  /></label>
                  <button
                    type="submit"
                    data-opening
                    class="btn btn-secondary"
                    :disabled="busy || !userMatches"
                  >
                    记录一次性初始额
                  </button>
                </form>
                <p class="vip-note">
                  每个用户只能创建一次初始记录，不改变余额。须先核对已有成长流水，避免把已计入的充值重复回填。
                </p>
              </details>
            </section>
          </template>
        </div>
      </template>
      <ConfirmDialog
        :show="!!confirmation"
        :title="confirmation?.title || ''"
        :message="confirmation?.message || ''"
        @confirm="confirmAction"
        @cancel="confirmation = null"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, watch } from 'vue'
import { formatMoneyFixed as money } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useAppStore } from '@/stores/app'
import { apiClient } from '@/api/client'
import {
  getVIP,
  getVIPRules,
  saveVIPRules,
  setVIPLevel,
  restoreVIPLevel,
  createVIPOpening,
  type VIPRules,
  type VIPSnapshot,
} from '@/api/vip'
import type { Group } from '@/types'

const appStore = useAppStore()
const tabs = [
  { id: 'rules', label: '等级与规则' },
  { id: 'groups', label: '分组权益' },
  { id: 'users', label: '用户权益' },
]
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
const selectedLevel = ref(0),
  levelReason = ref('')
const levelOptions = Array.from({ length: 6 }, (_, level) => ({
  value: level,
  label: level ? 'VIP ' + level : '普通会员',
}))
const gradeName = (level: number) => (level ? 'VIP ' + level : '普通会员')
const userOptions = ref<{ value: number; label: string }[]>([]),
  searching = ref(false)
let searchGeneration = 0
const selectedTier = computed(() =>
  (userState.value?.rules ?? rules.value)?.tiers.find(
    (t) => t.level === selectedLevel.value,
  ),
)
async function searchUsers(query: string) {
  const generation = ++searchGeneration
  searching.value = true
  try {
    const result = await apiClient.get<{
      items: { id: number; email: string; username: string }[]
    }>('/admin/users', { params: { page: 1, page_size: 20, search: query } })
    if (generation === searchGeneration)
      userOptions.value = (result.data.items || []).map((u) => ({
        value: u.id,
        label: `${u.username || u.email} · ${u.email} · ID ${u.id}`,
      }))
  } catch {
    if (generation === searchGeneration)
      appStore.showError('用户列表读取失败，可输入ID查询')
  } finally {
    if (generation === searchGeneration) searching.value = false
  }
}
function chooseUser(value: string | number | boolean | null) {
  if (typeof value === 'number' && value > 0) {
    userId.value = value
    void loadUser()
  }
}
watch(activeTab, (tab) => {
  if (tab === 'users') void searchUsers('')
})
const validUserId = computed(
  () => Number.isInteger(userId.value) && userId.value > 0,
)
const userMatches = computed(
  () => validUserId.value && userId.value === loadedUserId.value,
)
const savedRules = ref('')
const confirmation = ref<{
  title: string
  message: string
  execute: () => Promise<void>
} | null>(null)
const excluded = (group: Group) =>
  ['zth-plus', 'zth-pro', 'ceshi', 'ceshi-gemini'].includes(group.name) ||
  group.subscription_type === 'subscription'

function rule(id: number) {
  const result = rules.value!.groups.find((group) => group.group_id === id)
  if (!result) throw new Error('分组规则缺失')
  return result
}
function navigateTabs(event: KeyboardEvent, id: string) {
  const index = tabs.findIndex((tab) => tab.id === id)
  const next =
    event.key === 'ArrowRight'
      ? (index + 1) % tabs.length
      : event.key === 'ArrowLeft'
        ? (index + tabs.length - 1) % tabs.length
        : event.key === 'Home'
          ? 0
          : event.key === 'End'
            ? tabs.length - 1
            : -1
  if (next < 0) return
  event.preventDefault()
  activeTab.value = tabs[next].id
  document.getElementById(`vip-tab-${activeTab.value}`)?.focus()
}
function addCurrency() {
  const currency = newCurrency.value.toUpperCase()
  if (!/^[A-Z]{3}$/.test(currency) || !rules.value) {
    appStore.showError('请输入三位币种代码')
    return
  }
  if (currency in rules.value.exchange_rates) {
    appStore.showError('该币种已存在')
    return
  }
  rules.value.exchange_rates[currency] = 1
  newCurrency.value = ''
}
async function action(work: () => Promise<void>, message?: string) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await work()
    if (message) appStore.showSuccess(message)
  } catch (cause) {
    const apiError = cause as { message?: unknown }
    error.value =
      typeof apiError?.message === 'string'
        ? apiError.message
        : '操作失败，请重试'
    appStore.showError(error.value)
  } finally {
    busy.value = false
  }
}
async function loadRules() {
  await action(async () => {
    const [config, all] = await Promise.all([
      getVIPRules(),
      apiClient.get<Group[]>('/admin/groups/all'),
    ])
    config.exchange_rates ||= { USD: 1 }
    for (const group of all.data) {
      if (!config.groups.some((item) => item.group_id === group.id))
        config.groups.push({
          group_id: group.id,
          private: excluded(group),
          access: false,
          floor: group.rate_multiplier,
          discounts: [0, 0, 0, 0, 0],
        })
    }
    rules.value = config
    groups.value = all.data
    savedRules.value = JSON.stringify(config)
  })
}
function requestRefresh() {
  if (activeTab.value === 'users') {
    if (validUserId.value) void loadUser()
    return
  }
  if (rules.value && JSON.stringify(rules.value) !== savedRules.value) {
    confirmation.value = {
      title: '重新加载规则',
      message: '存在尚未保存的规则修改，刷新将放弃这些修改。是否继续？',
      execute: loadRules,
    }
  } else void loadRules()
}
async function save() {
  if (!rules.value) return
  await action(async () => {
    await saveVIPRules(rules.value!)
    savedRules.value = JSON.stringify(rules.value)
  }, 'VIP 规则已保存')
}
async function loadUser() {
  if (!validUserId.value) return
  const id = userId.value
  userState.value = null
  loadedUserId.value = 0
  await action(async () => {
    userState.value = await getVIP(id)
    loadedUserId.value = id
    syncLevelForm()
  })
}
function requestRestoreLevel() {
  if (!userMatches.value || busy.value) return
  const id = loadedUserId.value
  confirmation.value = {
    title: '恢复自动VIP等级',
    message: `将清除用户 ID ${id} 的指定等级与旧单项覆盖，按真实充值成长重新计算整套权益。是否继续？`,
    execute: async () => {
      if (userId.value !== id || loadedUserId.value !== id) {
        appStore.showError('所选用户已变化，请重新确认')
        return
      }
      await action(async () => {
        await restoreVIPLevel(id, '管理员恢复自动等级')
        userState.value = await getVIP(id)
        syncLevelForm()
      }, '已恢复自动等级')
    },
  }
}
function syncLevelForm() {
  selectedLevel.value =
    userState.value?.level_override?.value ?? userState.value?.tier.level ?? 0
  levelReason.value = ''
  originalExpiry = userState.value?.level_override?.expires_at ?? null
  if (originalExpiry) {
    const date = new Date(originalExpiry)
    expiry.value = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
      .toISOString()
      .slice(0, 16)
  } else expiry.value = ''
  originalExpiryInput = expiry.value
}
let originalExpiry: string | null = null,
  originalExpiryInput = ''
function requestLevelChange() {
  if (!userMatches.value || busy.value) return
  if (
    !Number.isInteger(selectedLevel.value) ||
    selectedLevel.value < 0 ||
    selectedLevel.value > 5 ||
    levelReason.value.trim().length < 3
  ) {
    appStore.showError('请选择有效等级并填写至少三个字的原因')
    return
  }
  const id = loadedUserId.value,
    body = {
      level: selectedLevel.value,
      reason: levelReason.value.trim(),
      expires_at: expiry.value
        ? expiry.value === originalExpiryInput && originalExpiry
          ? originalExpiry
          : new Date(expiry.value).toISOString()
        : null,
    }
  confirmation.value = {
    title: '确认修改VIP等级',
    message: `将用户 ID ${id} 指定为 ${gradeName(body.level)}，统一应用对应权益并替换旧单项覆盖。${body.expires_at ? '有效至 ' + new Date(body.expires_at).toLocaleString() + '，到期后恢复自动等级。' : '持续有效，直到管理员恢复自动等级。'}`,
    execute: async () => {
      if (userId.value !== id || loadedUserId.value !== id) {
        appStore.showError('所选用户已变化，请重新确认')
        return
      }
      await action(async () => {
        await setVIPLevel(id, body)
        userState.value = await getVIP(id)
        syncLevelForm()
      }, 'VIP等级及整套权益已更新')
    },
  }
}
function requestOpening() {
  if (!userMatches.value || busy.value) return
  const id = loadedUserId.value,
    amount = openingAmount.value,
    reason = openingReason.value
  confirmation.value = {
    title: '确认历史充值初始额',
    message: `为用户 ID ${id} 记录 $${money(Number(amount))} 的一次性初始额，不改变余额。请确认已核对历史流水、不会重复计入充值。`,
    execute: async () => {
      await action(async () => {
        await createVIPOpening(id, amount, reason)
        userState.value = await getVIP(id)
      }, '历史充值初始额已记录')
    },
  }
}
async function confirmAction() {
  const pending = confirmation.value
  confirmation.value = null
  if (pending && !busy.value) await pending.execute()
}
onMounted(loadRules)
</script>

<style scoped>
.vip-admin {
  min-width: 0;
}
.vip-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}
.vip-tabs {
  display: flex;
  gap: 24px;
  border-bottom: 1px solid var(--mg-line-warm);
}
.vip-tabs button {
  padding: 12px 0;
  border-bottom: 2px solid transparent;
  font-size: 14px;
  white-space: nowrap;
  color: var(--mg-muted);
}
.vip-tabs button.active {
  border-color: var(--mg-gold-700);
  color: var(--mg-ink-900);
  font-weight: 600;
}
.vip-tabs button:focus-visible {
  outline: 2px solid var(--mg-gold-700);
  outline-offset: 4px;
}
.vip-section {
  padding: 20px 0;
  border-top: 1px solid var(--mg-line-warm);
}
.vip-section h2 {
  margin-bottom: 16px;
  font-size: 16px;
  font-weight: 600;
}
.vip-fields {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 240px));
  gap: 16px;
}
.vip-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 13px;
  min-width: 0;
}
.vip-field .input {
  width: 100%;
}
.vip-field .btn {
  flex-shrink: 0;
  white-space: nowrap;
}
.vip-note {
  margin-top: 14px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--mg-muted);
}
.vip-table-wrap {
  overflow-x: auto;
  border: 1px solid var(--mg-line-warm);
  border-radius: 6px;
  background: var(--mg-surface);
}
.vip-table {
  width: 100%;
  text-align: left;
  font-size: 13px;
}
.vip-table th {
  white-space: nowrap;
  font-weight: 500;
}
.vip-table th,
.vip-table td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--mg-line-warm);
}
.vip-table tbody tr:last-child td {
  border-bottom: 0;
}
.vip-table .input {
  min-width: 88px;
  width: 100%;
  max-width: 200px;
}
.vip-table .btn {
  white-space: nowrap;
}
.vip-group-table .vip-table {
  min-width: 1180px;
}
.vip-group-table .input {
  width: 92px;
}
.vip-user-summary {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 20px;
  padding: 18px 0;
  border-block: 1px solid var(--mg-line-warm);
}
.vip-user-summary span {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--mg-muted);
}
.vip-user-summary strong {
  font-size: 15px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.vip-level-form {
  display: grid;
  grid-template-columns:
    minmax(140px, 1fr) 110px minmax(210px, 1fr) minmax(180px, 2fr)
    auto;
  align-items: end;
  gap: 16px;
}
.vip-reason {
  min-width: 160px;
  max-width: 400px;
  overflow-wrap: anywhere;
}
@media (max-width: 1100px) {
  .vip-level-form {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .vip-user-summary {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .vip-tabs {
    gap: 20px;
  }
  .vip-tabs button {
    font-size: 13px;
  }
  .vip-fields,
  .vip-level-form {
    grid-template-columns: minmax(0, 1fr);
  }
  .vip-user-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .vip-table th,
  .vip-table td {
    padding: 10px 12px;
  }
}
.vip-user-picker {
  width: min(420px, 100%);
}
.vip-section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.vip-section-heading h2 {
  margin-bottom: 0;
}
.vip-level-form {
  grid-template-columns:
    minmax(140px, 1fr) minmax(220px, 1fr) minmax(180px, 2fr)
    auto;
  margin-top: 22px;
}
.vip-benefit-preview {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
  margin-top: 24px;
  padding: 20px;
  border: 1px solid var(--mg-line-warm);
  border-radius: 8px;
  background: var(--mg-surface);
}
.vip-benefit-preview span,
.vip-user-summary small {
  display: block;
  font-size: 12px;
  color: var(--mg-muted);
}
.vip-benefit-preview strong {
  display: block;
  margin-top: 10px;
  color: var(--mg-gold-700);
  font-weight: 500;
}
.vip-effective-summary {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--mg-muted);
}
.vip-effective-summary strong {
  color: var(--mg-ink-900);
  margin-left: 8px;
}
.vip-legacy-note {
  padding: 12px 16px;
  border: 1px solid var(--mg-line-warm);
  border-radius: 6px;
  font-size: 12px;
  color: var(--mg-gold-700);
  margin-top: 18px;
}
.vip-user-summary small {
  margin-top: 6px;
}
@media (max-width: 1000px) {
  .vip-benefit-preview {
    grid-template-columns: repeat(3, 1fr);
  }
  .vip-level-form {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 640px) {
  .vip-section-heading {
    align-items: start;
    flex-direction: column;
  }
  .vip-level-form {
    grid-template-columns: 1fr;
  }
  .vip-benefit-preview {
    grid-template-columns: repeat(2, 1fr);
    padding: 16px;
  }
}
</style>
