<template>
  <AppLayout>
    <div class="vip-page">
      <header class="vip-heading">
        <div>
          <p class="eyebrow">MODEL-GATE MEMBERSHIP</p>
          <h1>VIP 中心</h1>
        </div>
        <button class="btn btn-secondary" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" />刷新
        </button>
      </header>
      <div v-if="loading && !state" class="empty">正在读取会员权益…</div>
      <div v-else-if="error" class="status-note" role="alert">
        {{ error }}<button class="btn btn-secondary" @click="load">重试</button>
      </div>
      <template v-else-if="state">
        <p v-if="!state.enabled" class="status-note">
          会员规则尚未启用，当前按原有价格与权限使用。
        </p>
        <section class="membership">
          <VIPMembershipCard
            class="member-card"
            :level="state.badge_level"
            :name="levelName(state.badge_level)"
            :threshold-label="ownerThreshold"
            :benefits="ownerBenefits"
            :owner-name="accountName"
            :owner-id="authStore.user?.id"
          />
          <div class="member-overview">
            <div class="recharge-heading">
              <div>
                <p class="muted">累计有效充值</p>
                <strong class="total">${{ money(state.total) }}</strong>
              </div>
              <router-link to="/purchase" class="btn btn-primary"
                ><Icon name="creditCard" size="sm" />去充值</router-link
              >
            </div>
            <div class="member-progress">
              <div class="progress-label">
                <span>{{
                  state.next
                    ? '距 VIP ' +
                      state.next.level +
                      ' 还需 $' +
                      money(Math.max(0, state.next.threshold - state.total))
                    : '已达到最高充值等级'
                }}</span
                ><span>{{ Math.round(progress) }}%</span>
              </div>
              <progress :value="progress" max="100" aria-label="会员成长进度" />
            </div>
            <div class="overview-stats">
              <div>
                <span>邀请返利</span
                ><strong>{{ state.rebate_percent }}%</strong>
              </div>
              <div>
                <span>每分钟请求上限</span
                ><strong>{{
                  state.rpm ? state.rpm + ' RPM' : '沿用原有限流规则'
                }}</strong>
              </div>
              <div>
                <span>专属分组</span
                ><strong>{{ exclusiveAccess ? '已开放' : '待解锁' }}</strong>
              </div>
              <div>
                <span>累计已领奖励</span
                ><strong>{{
                  membership ? '$' + money(membership.claimed) : '—'
                }}</strong>
              </div>
              <div>
                <span>可领取奖励</span
                ><strong>{{ membership ? availableRewards : '—' }}</strong>
              </div>
              <div>
                <span>成长门槛</span
                ><strong>${{ money(state.rules.access_threshold) }} 起</strong>
              </div>
            </div>
          </div>
          <div
            v-if="manualPrivilege || exclusiveAccess"
            class="membership-followup"
          >
            <p v-if="manualPrivilege" class="status-note">
              包含管理员授予的权益，成长进度仍按累计有效充值计算。
            </p>
            <router-link v-if="exclusiveAccess" to="/keys" class="text-link"
              >管理专属分组 API Key <Icon name="arrowRight" size="sm"
            /></router-link>
          </div>
        </section>
        <section class="honors">
          <div class="section-title">
            <div>
              <p class="eyebrow">PRIVATE CIRCLE</p>
              <h2>VIP 荣誉席位</h2>
              <p class="muted">按当前有效等级排列 · 会员名称已脱敏</p>
            </div>
            <span class="eyebrow">TOP 10</span>
          </div>
          <p v-if="membershipError" class="status-note" role="alert">
            {{ membershipError
            }}<button class="text-link" @click="loadMembership">重试</button>
          </p>
          <template v-else-if="membership">
            <div v-if="membership.seats.length" class="podium">
              <article
                v-for="(seat, index) in membership.seats.slice(0, 3)"
                :key="index"
              >
                <p class="rank">
                  {{ String(index + 1).padStart(2, '0') }}
                  <span>{{ index === 0 ? '首席' : '荣誉席' }}</span>
                </p>
                <h3>{{ seat.name }}</h3>
                <span class="vip-tag" :class="'level-' + seat.level"
                  >VIP {{ seat.level }} · {{ levelName(seat.level) }}</span
                >
              </article>
            </div>
            <ol v-if="membership.seats.length > 3" class="seat-list" start="4">
              <li
                v-for="(seat, index) in membership.seats.slice(3)"
                :key="index"
              >
                <span class="rank">{{
                  String(index + 4).padStart(2, '0')
                }}</span
                ><strong>{{ seat.name }}</strong
                ><span class="vip-tag" :class="'level-' + seat.level"
                  >VIP {{ seat.level }} · {{ levelName(seat.level) }}</span
                >
              </li>
            </ol>
            <p v-if="!membership.seats.length" class="empty">
              荣誉席位静待首位会员
            </p>
          </template>
        </section>
        <nav class="vip-tabs" role="tablist" aria-label="会员权益">
          <button
            v-for="item in tabs"
            :id="'tab-' + item.id"
            :key="item.id"
            role="tab"
            :data-tab="item.id"
            :aria-selected="tab === item.id"
            :aria-controls="'panel-' + item.id"
            :class="{ active: tab === item.id }"
            @click="tab = item.id"
          >
            {{ item.label }}
          </button>
        </nav>
        <section
          v-if="tab === 'benefits'"
          id="panel-benefits"
          role="tabpanel"
          aria-labelledby="tab-benefits"
        >
          <div class="section-title">
            <div>
              <p class="eyebrow">MEMBERSHIP</p>
              <h2>等级权益</h2>
              <p class="muted">累计有效充值，逐级开启更多权益</p>
            </div>
          </div>
          <div class="tier-journey-mobile" aria-label="会员成长摘要">
            <div v-for="item in journeys" :key="item.title"><h3>{{ item.title }}</h3><p>{{ item.mobile }}</p></div>
          </div>
          <div class="tier-grid">
            <VIPMembershipCard
              class="tier-card"
              :class="{ current: state.tier.level === 0 }"
              :level="0"
              name="普通会员"
              threshold-label="注册即享 · 无充值门槛"
              :benefits="[
                '普通分组维持原价' + (bonusActive ? ' · 加赠 0%' : ''),
                '原有并发额度 · 邀请返利 0%',
                '累计充值成长记录',
                `签到 $${money(signInPlan[0] ?? 0)} / 日`,
              ]"
              :current="state.tier.level === 0"
            />
            <VIPMembershipCard
              v-for="tier in state.rules.tiers"
              :key="tier.level"
              class="tier-card"
              :class="{ current: state.tier.level === tier.level }"
              :level="tier.level"
              :name="levelName(tier.level)"
              :threshold-label="
                '累计有效充值 $' + money(tier.threshold) + ' 起'
              "
              :benefits="tierBenefits(tier)"
              :current="state.tier.level === tier.level"
            />
            <aside v-for="(item, index) in journeys" :key="item.title" class="tier-journey" :style="{ gridRow: index + 1 }">
              <div class="journey-content"><span class="journey-node">{{ String(index + 1).padStart(2, '0') }}</span><h3>{{ item.title }}</h3><strong class="vip-number">{{ item.value }}</strong><small>{{ item.caption }}</small><p v-for="line in item.lines" :key="line">{{ line }}</p></div>
            </aside>
          </div>
          <p class="fine-print">
            倍率为绝对值减免，部分分组维持原价。专属分组独立定价；人工授权与定价优先。请求仍受渠道自身容量与限额约束。
          </p>
          <section
            class="growth-benefits"
            aria-labelledby="growth-benefits-title"
          >
            <div class="section-title">
              <div>
                <p class="eyebrow">GROWTH PRIVILEGES</p>
                <h2 id="growth-benefits-title">成长回馈</h2>
              </div>
              <router-link to="/achievements" class="text-link"
                >去签到 →</router-link
              >
            </div>
            <div class="table-overflow">
              <table>
                <thead>
                  <tr>
                    <th>等级</th>
                    <th>充值加赠</th>
                    <th>每日签到方案</th>
                    <th>示例节省 · 0.4 倍率</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>普通会员</td>
                    <td><span :class="{ 'vip-number': bonusActive }">{{ bonusActive ? '0%' : '按充值页现行规则' }}</span></td>
                    <td><span class="vip-number">${{ money(signInPlan[0] ?? 0) }}</span> · 以签到页为准</td>
                    <td>对照基准</td>
                  </tr>
                  <tr v-for="tier in state.rules.tiers" :key="tier.level">
                    <td><span class="vip-number">VIP {{ tier.level }}</span> · {{ levelName(tier.level) }}</td>
                    <td><span :class="{ 'vip-number': bonusActive }">{{ bonusActive ? bonus(tier) + '%' : '未启用' }}</span></td>
                    <td>
                      <span class="vip-number">${{ money(signInPlan[tier.level] ?? 0) }}</span> · 以签到页为准
                    </td>
                    <td class="vip-number example-saving">{{ referenceSaving(tier) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p class="fine-print">
              文档示例测算：基础倍率 0.4，普通会员无加赠；VIP 1–5 分别减免 0.015 / 0.03 / 0.045 / 0.06 / 0.075，加赠 1%–5%。计算方式：1 −（会员倍率 ÷ 0.4）÷（1 + 加赠比例）。不含人工价、高峰因子、邀请收入、一次性奖励或签到，不代表所有分组实际折扣；实际价格与签到资格以对应页面为准。
            </p>
            <router-link to="/docs?cat=membership&amp;page=vip" class="text-link">查看会员权益文档</router-link>
          </section>
        </section>
        <section
          v-else-if="tab === 'rewards'"
          id="panel-rewards"
          role="tabpanel"
          aria-labelledby="tab-rewards"
        >
          <div class="section-title">
            <div>
              <p class="eyebrow">MILESTONES</p>
              <h2>累充奖励</h2>
              <p class="muted">每个里程碑仅可领取一次 · 奖励为门槛金额的 2%</p>
            </div>
          </div>
          <p v-if="membership?.debt" class="status-note">
            待追回奖励 ${{ money(membership.debt) }}，后续余额入账将优先抵扣。
          </p>
          <p v-if="claimMessage" class="status-note" role="status">
            {{ claimMessage }}
          </p>
          <p v-if="membershipError" class="status-note" role="alert">
            {{ membershipError }}
          </p>
          <div
            v-for="reward in membership?.rewards || []"
            :key="reward.level"
            class="reward-row"
          >
            <div class="reward-description">
              <div class="diamond">
                <Icon
                  :name="
                    reward.status === 'available'
                      ? 'gift'
                      : reward.status === 'claimed'
                        ? 'check'
                        : 'lock'
                  "
                  size="sm"
                />
              </div>
              <div>
                <h3>VIP {{ reward.level }} {{ levelName(reward.level) }}</h3>
                <p class="muted">累计充值满 ${{ money(reward.threshold) }}</p>
              </div>
            </div>
            <div class="reward-actions">
              <div class="reward-amount">
                <strong>${{ money(reward.amount) }}</strong
                ><span class="muted">余额奖励</span>
              </div>
              <button
                v-if="reward.status === 'available'"
                class="btn btn-primary"
                :data-claim="reward.level"
                :disabled="claiming !== null"
                @click="claim(reward.level)"
              >
                <Icon name="gift" size="sm" />{{
                  claiming === reward.level ? '领取中…' : '领取奖励'
                }}</button
              ><span v-else class="reward-status">{{
                statusLabel(reward.status)
              }}</span>
            </div>
          </div>
          <p class="fine-print">
            奖励不计入累计充值，不产生邀请返利。退款或后台退费跌破门槛时追回对应奖励，领取记录保留，再次达标不重复发放。
          </p>
        </section>
        <section
          v-else
          id="panel-records"
          role="tabpanel"
          aria-labelledby="tab-records"
        >
          <div class="section-title">
            <h2>充值成长记录</h2>
            <router-link to="/affiliate" class="text-link"
              >邀请返利明细 <Icon name="arrowRight" size="sm"
            /></router-link>
          </div>
          <div v-if="state.ledger.length" class="table-overflow">
            <table>
              <thead>
                <tr>
                  <th>时间</th>
                  <th>来源</th>
                  <th>计入金额</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="entry in state.ledger" :key="entry.id">
                  <td>{{ new Date(entry.created_at).toLocaleString() }}</td>
                  <td>{{ sourceLabel(entry.source) }}</td>
                  <td>
                    {{ entry.amount >= 0 ? '+' : '−' }}${{
                      money(Math.abs(entry.amount))
                    }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="empty">暂无已确认的充值成长记录</p>
        </section>
        <section
          class="membership-rules"
          aria-labelledby="membership-rules-title"
        >
          <div>
            <p class="eyebrow">PROGRAM RULES</p>
            <h2 id="membership-rules-title">会员规则</h2>
          </div>
          <ul>
            <li>
              累计有效充值包含已完成的在线余额充值及后台手动增加余额，统一按配置汇率折算为美元。
            </li>
            <li>
              兑换码、赠送余额、订阅订单及活动奖励不计入累计充值；消费不影响充值成长额。
            </li>
            <li>
              在线退款与后台退费冲减累计额，自动更新等级和未领奖励资格；人工授予的权益按授权执行。
            </li>
            <li>
              普通分组按规则减免倍率绝对值，最多减免
              0.075；小倍率分组维持原有较小减免或原价，人工定价优先。
            </li>
            <li>
              充值加赠按下单时的有效 VIP 等级计算，普通会员 0%、VIP 阶梯最高
              5%；加赠不计成长额，升级后的比例从下一笔订单生效。
            </li>
            <li>
              每日签到按当前有效 VIP
              等级与后台配置金额计算；现金开放范围及当前可领金额以成就与签到页为准，已签到金额不重复发放。
            </li>
            <li>
              VIP 专属分组独立定价，现有 API Key 不自动切换分组；并发和 RPM
              仍受渠道容量限制。
            </li>
            <li>
              每档累充奖励为门槛金额的
              2%，达标后手动领取，每档仅一次；奖励不计入充值、不产生邀请返利。
            </li>
            <li>
              退款跌破历史领奖门槛时追回对应奖励，再次达标不重复发放；余额不足的差额由后续入账抵扣。
            </li>
          </ul>
        </section>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { formatMoneyFixed as money } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import VIPMembershipCard from '@/components/user/VIPMembershipCard.vue'
import { useAuthStore } from '@/stores/auth'
import {
  getVIP,
  getVIPMembership,
  claimVIPReward,
  type VIPSnapshot,
  type VIPMembership,
  type VIPReward,
  type VIPTier,
} from '@/api/vip'
const state = ref<VIPSnapshot | null>(null)
const authStore = useAuthStore()
const accountName = computed(
  () => authStore.user?.username?.trim() || authStore.user?.email || '当前会员',
)
const membership = ref<VIPMembership | null>(null)
const loading = ref(false)
const error = ref('')
const membershipError = ref('')
const claiming = ref<number | null>(null)
const claimMessage = ref('')
const tab = ref('benefits')
const tabs = [
  { id: 'benefits', label: '等级权益' },
  { id: 'rewards', label: '累充奖励' },
  { id: 'records', label: '成长记录' },
]
const levelName = (level: number) =>
  ['普通会员', '青铜', '白银', '黄金', '铂金', '黑钻'][level] || '会员'
const rate = (value: number) =>
  value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
const exclusiveAccess = computed(
  () => state.value?.groups.some((g) => g.exclusive && g.granted) || false,
)
const manualPrivilege = computed(() => state.value?.overrides?.length || 0)
const availableRewards = computed(
  () =>
    membership.value?.rewards.filter((r) => r.status === 'available').length ||
    0,
)
const progress = computed(() => {
  const s = state.value
  if (!s) return 0
  if (!s.next) return 100
  const previous = (s.growth_tier ?? s.tier).threshold || 0
  return Math.max(
    0,
    Math.min(100, ((s.total - previous) / (s.next.threshold - previous)) * 100),
  )
})
const signInPlan = computed(
  () => state.value?.rules.daily_rewards ?? [0.01, 0.05, 0.1, 0.25, 0.5, 1],
)
const bonusActive = computed(
  () => !!state.value?.enabled && !!state.value?.rules.recharge_bonus_enabled,
)
const bonus = (tier: VIPTier) => tier.recharge_bonus_percent ?? 0
function referenceSaving(tier: VIPTier) {
  // Documentation illustration, not the user's effective or manually overridden price.
  const discount = [0, 0.015, 0.03, 0.045, 0.06, 0.075][tier.level]
  if (discount === undefined) return '—'
  return ((1 - (0.4 - discount) / 0.4 / (1 + tier.level / 100)) * 100).toFixed(2) + '%'
}
const journeys = computed(() => {
  const top = state.value?.rules.tiers.at(-1)
  const threshold = state.value?.rules.access_threshold ?? 100
  return [
    { title: '入会礼遇', value: '$' + money(threshold), caption: '累计有效充值起', lines: ['开启 VIP 专属分组资格', '每笔充值积累成长'], mobile: '$' + money(threshold) + ' 起 · 专属资格' },
    { title: '成长回馈', value: '13.83%', caption: 'VIP 3 · 文档示例节省', lines: ['倍率减免与充值加赠', '共同提升使用价值'], mobile: '倍率减免 · 充值加赠' },
    { title: '尊享权益', value: (top?.concurrency ?? 30) + ' 并发', caption: '最高档 · 并发目标', lines: ['邀请返利最高 ' + (top?.rebate_percent ?? 10) + '%', '签到余额按等级提升'], mobile: '并发 · 邀请 · 签到' },
  ]
})
function discountText(level: number) {
  const cuts = membership.value?.discount_summaries[level] || []
  if (!cuts.length)
    return membership.value ? '普通分组按现行规则计价' : '普通分组成长优惠'
  return `普通分组最高减免 ${rate(cuts[0])}`
}
function tierBenefits(tier: VIPTier) {
  const reward = membership.value?.rewards.find(
    (item) => item.level === tier.level,
  )
  const rewardAmount =
    reward?.status === 'claimed' || reward?.status === 'revoked'
      ? tier.threshold * 0.02
      : (reward?.amount ?? tier.threshold * 0.02)
  return [
    discountText(tier.level).replace('普通分组最高', '最高') +
      (bonusActive.value ? ` · 加赠 ${bonus(tier)}%` : ''),
    `${tier.concurrency} 并发 · 邀请返利 ${tier.rebate_percent}%`,
    `本档奖励 $${money(rewardAmount)} · 签到 $${money(signInPlan.value[tier.level] ?? 0)} / 日`,
    'VIP 专属分组资格 · 以开放规则为准',
  ]
}
const ownerThreshold = computed(() => {
  const s = state.value
  if (!s) return ''
  if (s.tier_source === 'manual')
    return `管理员指定 VIP ${s.tier.level} · 充值成长 VIP ${s.growth_tier?.level ?? 0}`
  if (s.badge_level !== s.tier.level) return `充值成长等级 VIP ${s.tier.level}`
  return s.tier.level
    ? `累计有效充值 $${money(s.tier.threshold)} 起`
    : '注册即享 · 无充值门槛'
})
const ownerBenefits = computed(() => {
  const s = state.value
  if (!s) return []
  const access = exclusiveAccess.value ? '专属分组' : '分组待解锁'
  const rewardText = `${access} · 签到 $${money(signInPlan.value[s.tier.level] ?? 0)} / 日`
  const reward = membership.value?.rewards.find(item => item.level === s.tier.level)
  return [
    discountText(s.tier.level).replace('普通分组最高', '最高') +
      (bonusActive.value ? ` · 加赠 ${bonus(s.tier)}%` : ''),
    `${s.concurrency} 并发 · 邀请返利 ${s.rebate_percent}%`,
    rewardText,
    reward ? `本档奖励 $${money(reward.amount)}` : '累计充值成长记录',
  ]
})
const statusLabel = (status: VIPReward['status']) =>
  ({
    locked: '未达标',
    available: '可领取',
    claimed: '已领取',
    revoked: '已追回',
  })[status]
const sourceLabel = (source: string) =>
  ({
    payment: '在线充值',
    payment_refund: '在线退款',
    admin_balance: '后台调整',
    opening: '初始确认记录',
  })[source] || source
async function loadMembership() {
  membershipError.value = ''
  try {
    membership.value = await getVIPMembership()
  } catch {
    membership.value = null
    membershipError.value = '荣誉席位与奖励读取失败，请重试。'
  }
}
async function load() {
  loading.value = true
  error.value = ''
  try {
    state.value = await getVIP()
    await loadMembership()
  } catch {
    error.value = '会员权益读取失败，请重试。'
  } finally {
    loading.value = false
  }
}
async function claim(level: number) {
  if (claiming.value !== null) return
  claiming.value = level
  claimMessage.value = ''
  try {
    const result = await claimVIPReward(level)
    claimMessage.value =
      result.amount > 0
        ? `奖励 $${money(result.amount)} 已入账。`
        : '该档奖励已领取，请勿重复领取。'
    await load()
    await authStore.refreshUser().catch(() => undefined)
  } catch (failure) {
    const detail = failure as { reason?: string; message?: string }
    const known =
      typeof detail?.reason === 'string' &&
      detail.reason.startsWith('VIP_REWARD_')
    claimMessage.value =
      known && typeof detail.message === 'string'
        ? detail.message
        : '领取失败，请稍后重试。'
    if (known) await load()
  } finally {
    claiming.value = null
  }
}
onMounted(load)
</script>
<style scoped>
.growth-benefits {
  margin-top: 28px;
  border-top: 1px solid var(--mg-line, #e8dfd0);
  padding-top: 24px;
}
.growth-benefits td {
  white-space: nowrap;
}
.vip-page {
  max-width: 1440px;
  margin: auto;
  color: var(--mg-ink-900, #24252a);
  letter-spacing: 0;
}
h1,
h2,
h3 {
  font-family: 'Noto Serif SC', SimSun, serif;
  font-weight: 600;
  letter-spacing: 0;
}
h1 {
  font-size: 26px;
}
h2 {
  font-size: 22px;
}
h3 {
  font-size: 18px;
}
.vip-heading,
.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
}
.eyebrow {
  font:
    500 11px 'DM Mono',
    Consolas,
    monospace;
  color: var(--mg-gold-700, #927640);
}
.muted,
.fine-print {
  font-size: 13px;
  color: var(--mg-muted, #746f65);
  line-height: 1.8;
}
.section-title .muted {
  margin-top: 8px;
}
/* The summary shares the card's column width and aspect-ratio geometry. */
.membership {
  display: grid;
  grid-template-columns: minmax(0, 4fr) minmax(0, 6fr);
  align-items: start;
  gap: 24px;
  margin-bottom: 28px;
  container-type: inline-size;
  --vip-aspect-ratio: 1.72;
  --member-card-height: calc((100cqi - 24px) * 0.4/var (--vip-aspect-ratio));
}
.member-overview {
  height: var(--member-card-height);
  padding: 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}
.membership-followup {
  grid-column: 1/-1;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 16px;
  flex-wrap: wrap;
  margin-top: -12px;
}
.membership-followup .status-note {
  flex: 1;
  margin: 0;
}
.recharge-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.recharge-heading .muted {
  line-height: 20px;
}
.total {
  font:
    500 34px/1.2 'DM Mono',
    Consolas,
    monospace;
  display: block;
  margin-top: 4px;
}
.progress-label {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  font-size: 12px;
  line-height: 18px;
}
.overview-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
}
.overview-stats > div {
  padding: 8px 10px;
  border-top: 1px solid var(--mg-line-warm, #e7e1d4);
}
.overview-stats > div:nth-child(n + 4) {
  padding-bottom: 0;
}
.overview-stats span {
  display: block;
  font-size: 11px;
  line-height: 16px;
  color: var(--mg-muted, #746f65);
}
.overview-stats strong {
  display: block;
  margin-top: 5px;
  font:
    500 13px/18px 'DM Mono',
    monospace;
  overflow-wrap: anywhere;
}
progress {
  width: 100%;
  height: 5px;
  display: block;
  margin-top: 8px;
  appearance: none;
  border: none;
  background: #e7e1d4;
}
progress::-webkit-progress-bar {
  background: #e7e1d4;
}
progress::-webkit-progress-value {
  background: #aa8c47;
}
progress::-moz-progress-bar {
  background: #aa8c47;
}
.text-link {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  color: var(--mg-gold-700, #765f2c);
  font-size: 13px;
}
.honors {
  margin-bottom: 28px;
}
.podium {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  border-block: 1px solid var(--mg-line-warm, #e7e1d4);
  background: var(--mg-gold-50, #fcfaf4);
}
.podium article {
  padding: 18px 22px;
  border-right: 1px solid var(--mg-line-warm, #e7e1d4);
  min-height: 140px;
}
.podium article:last-child {
  border: 0;
}
.rank {
  font:
    500 26px 'DM Mono',
    monospace;
  color: var(--mg-gold-700, #927640);
}
.rank span {
  font-size: 11px;
}
.podium h3 {
  margin: 14px 0 10px;
  overflow-wrap: anywhere;
}
.seat-list {
  list-style: none;
  padding: 0;
}
.seat-list li {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 13px 8px;
  border-bottom: 1px solid var(--mg-line-warm, #e7e1d4);
}
.seat-list .rank {
  font-size: 18px;
}
.seat-list strong {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.vip-tag {
  display: inline-block;
  font:
    500 11px 'DM Mono',
    monospace;
  border: 1px solid currentColor;
  border-radius: 3px;
  padding: 5px 8px;
  color: var(--tier-color, inherit);
}
.level-0 {
  --tier-color: #7a8d98;
}
.level-1 {
  --tier-color: #ab7955;
}
.level-2 {
  --tier-color: #7d8586;
}
.level-3 {
  --tier-color: #aa8b3e;
}
.level-4 {
  --tier-color: #538286;
}
.level-5 {
  --tier-color: #ceb573;
}
.vip-tabs {
  display: flex;
  gap: 28px;
  border-bottom: 1px solid var(--mg-line-warm, #e7e1d4);
  margin-bottom: 28px;
}
.vip-tabs button {
  font-size: 15px;
  padding: 14px 0;
  border-bottom: 2px solid transparent;
  color: var(--mg-muted, #746f65);
  white-space: nowrap;
}
.vip-tabs button.active {
  color: var(--mg-ink-900, #24252a);
  border-color: #aa8c47;
}
.tier-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 180px minmax(0, 1fr);
  column-gap: 28px;
  row-gap: 32px;
  justify-content: space-between;
}
.tier-card.palette-0{grid-row:1;grid-column:1}.tier-card.palette-1{grid-row:1;grid-column:3}.tier-card.palette-2{grid-row:2;grid-column:1}.tier-card.palette-3{grid-row:2;grid-column:3}.tier-card.palette-4{grid-row:3;grid-column:1}.tier-card.palette-5{grid-row:3;grid-column:3}
.vip-number{font-family:'DM Mono','SFMono-Regular',Consolas,monospace;font-variant-numeric:lining-nums tabular-nums;font-feature-settings:'lnum' 1,'tnum' 1}
.tier-journey{grid-column:2;position:relative;display:flex;align-items:center;justify-content:center;text-align:center;min-width:0}
.tier-journey:before{content:'';position:absolute;top:0;bottom:0;left:50%;border-left:1px solid var(--mg-line-warm,#d7c9a8)}
.journey-content{position:relative;background:var(--mg-pearl-50,#faf9f6);padding:18px 4px;width:100%}
.journey-node{display:flex;align-items:center;justify-content:center;width:32px;height:32px;margin:0 auto 14px;border:1px solid var(--mg-gold-700,#aa8c47);border-radius:50%;font:12px 'DM Mono',Consolas,monospace;color:var(--mg-gold-700,#aa8c47)}
.journey-content h3{font-size:16px;margin-bottom:12px}.journey-content strong{display:block;font-size:22px;font-weight:500;color:var(--mg-gold-700,#aa8c47);margin:12px 0}.journey-content small{font-size:11px;color:var(--mg-muted,#777b73)}.journey-content p{font-size:12px;line-height:1.7;margin-top:8px}.tier-journey-mobile{display:none}.example-saving{color:var(--mg-gold-700,#aa8c47)}
.tier-card.current {
  outline: 1px solid #aa8c47;
  outline-offset: 3px;
}
.fine-print {
  font-size: 12px;
  margin-top: 20px;
}
.reward-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 16px;
  width: calc((100% - 24px) * 0.8 + 24px);
  max-width: 100%;
  border: 1px solid var(--mg-line-warm, #e7e1d4);
  border-radius: 6px;
  padding: 12px 16px;
  margin-bottom: 8px;
  background: var(--mg-surface, #fff);
}
.reward-description {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.reward-description h3 {
  font-size: 14px;
  line-height: 20px;
}
.diamond {
  flex-shrink: 0;
  border: 1px solid currentColor;
  transform: rotate(45deg);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 12px;
}
.diamond :deep(svg) {
  transform: rotate(-45deg);
}
.reward-description .diamond {
  width: 24px;
  height: 24px;
  margin: 5px;
  color: #a38b58;
}
.reward-description p {
  margin-top: 2px;
  font-size: 11px;
  line-height: 16px;
}
.reward-actions {
  display: flex;
  align-items: center;
  justify-self: end;
  gap: 10px;
}
.reward-amount {
  min-width: 72px;
  text-align: right;
}
.reward-amount strong {
  display: block;
  font:
    500 20px/24px 'DM Mono',
    monospace;
}
.reward-amount span {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  line-height: 16px;
}
.reward-status {
  font-size: 11px;
  color: var(--mg-muted, #746f65);
  border: 1px solid var(--mg-line-warm, #e7e1d4);
  padding: 6px 10px;
  border-radius: 4px;
  width: 94px;
  text-align: center;
}
.reward-row .btn {
  justify-self: end;
  min-height: 36px;
  width: 94px;
  padding: 6px 10px;
  font-size: 12px;
  gap: 6px;
  white-space: nowrap;
}
.status-note {
  padding: 12px 16px;
  margin: 16px 0;
  border-left: 2px solid #aa8c47;
  background: var(--mg-gold-50, #fcfaf4);
  font-size: 13px;
  line-height: 1.8;
}
.status-note button {
  margin-left: 12px;
}
.empty {
  padding: 32px 0;
  color: var(--mg-muted, #746f65);
  font-size: 13px;
}
.table-overflow {
  overflow: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
td,
th {
  padding: 14px 12px;
  border-bottom: 1px solid var(--mg-line-warm, #e7e1d4);
  text-align: left;
}
td:last-child {
  font-family: 'DM Mono', monospace;
}
.membership-rules {
  display: grid;
  grid-template-columns: 160px minmax(0, 1fr);
  gap: 28px;
  border-top: 1px solid var(--mg-line-warm, #e7e1d4);
  margin-top: 32px;
  padding: 24px 0;
}
.membership-rules h2 {
  font-size: 20px;
  margin-top: 6px;
}
.membership-rules ul {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 28px;
  padding-left: 16px;
  list-style: disc;
  color: var(--mg-muted, #746f65);
  font-size: 12px;
  line-height: 1.8;
}
.membership-rules li {
  padding-left: 3px;
  overflow-wrap: anywhere;
}
:global(.dark) .reward-row {
  background: #191a1d;
}
:global(.dark) .podium,
:global(.dark) .status-note {
  background: #20201e;
}
@media (max-width: 1240px) {
  .tier-grid {
    grid-template-columns: minmax(0, min(100%, 560px));
  }
  .tier-card{grid-column:auto!important;grid-row:auto!important}
  .tier-journey{display:none}
  .tier-journey-mobile{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:12px;padding:16px 0;margin-bottom:24px;border-block:1px solid var(--mg-line-warm,#e7e1d4)}
  .tier-journey-mobile h3{font-size:13px;margin-bottom:6px}.tier-journey-mobile p{font-size:11px;line-height:1.6;color:var(--mg-muted,#777b73)}
}
@media (max-width: 1240px) {
  .reward-row {
    width: 100%;
    max-width: 560px;
  }
}
@media (max-width: 1240px) {
  .membership {
    grid-template-columns: 1fr;
  }
  .member-card {
    max-width: 560px;
  }
  .member-overview {
    height: auto;
    padding: 0;
    gap: 18px;
  }
  .total {
    margin-top: 4px;
  }
}
@media (max-width: 640px) {
  .vip-heading {
    align-items: flex-start;
  }
  h1 {
    font-size: 23px;
  }
  .member-overview {
    padding: 0;
  }
  .total {
    font-size: 30px;
  }
  .overview-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .overview-stats > div:nth-child(n + 4) {
    padding-bottom: 8px;
  }
  .overview-stats > div:nth-child(n + 5) {
    padding-bottom: 0;
  }
  .podium {
    grid-template-columns: 1fr;
  }
  .podium article {
    min-height: 130px;
    border-right: 0;
    border-bottom: 1px solid var(--mg-line-warm, #e7e1d4);
  }
  .seat-list li {
    gap: 14px;
    flex-wrap: wrap;
  }
  .reward-row {
    gap: 8px;
    padding: 10px;
  }
  .reward-description {
    gap: 8px;
  }
  .reward-description h3 {
    font-size: 13px;
  }
  .reward-description .diamond {
    display: none;
  }
  .reward-amount {
    min-width: 62px;
  }
  .reward-amount strong {
    font-size: 17px;
    line-height: 22px;
  }
  .reward-actions {
    gap: 8px;
  }
  .reward-row .btn {
    width: 86px;
    padding: 6px 8px;
    font-size: 11px;
  }
  .reward-status {
    width: 86px;
  }
  .vip-tabs {
    gap: 24px;
  }
  .section-title {
    align-items: flex-start;
  }
  .membership-rules {
    grid-template-columns: 1fr;
    gap: 16px;
    margin-top: 24px;
  }
  .membership-rules ul {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
