<template>
  <AppLayout
    ><main class="achievement-page">
      <header class="heading">
        <div>
          <p class="eyebrow">A RECORD OF EVERY MILESTONE</p>
          <h1>成就与签到</h1>
          <p class="muted">让每一次成长，留下值得收藏的印记。</p>
        </div>
        <div class="edition">
          <span>COLLECTION NO. 001</span><strong>成长印记 · 四个篇章</strong
          ><span>日序 · 星核 · 铭印 · 探索</span>
        </div>
      </header>
      <p v-if="error" class="notice" role="alert">
        {{ error }} <button :disabled="busy" @click="load">重试</button>
      </p>
      <p v-if="message" class="notice" role="status">{{ message }}</p>
      <p v-if="!state && !error" class="loading">正在读取你的成长印记…</p>
      <template v-if="state">
        <section class="hero">
          <div class="wear-panel">
            <MedalArt
              v-if="equipped"
              :medal="equipped.key"
              :name="equipped.name"
              class="hero-art"
            />
            <div v-else class="empty-medal">MG</div>
            <div>
              <p class="eyebrow">CURRENTLY EQUIPPED</p>
              <h2>{{ equipped?.name ?? '静待第一枚印记' }}</h2>
              <p class="muted">
                {{
                  equipped?.description ?? '达成成就后，即可将徽章佩戴于此。'
                }}
              </p>
              <button
                v-if="equipped"
                class="text-button"
                :disabled="busy"
                @click="act('equip', '')"
              >
                卸下徽章
              </button>
              <p v-else class="gold small">收藏从今天开始</p>
            </div>
          </div>
          <div class="overview">
            <div class="stats">
              <div>
                <strong>{{ unlocked }}<small> / 21</small></strong
                ><span>已解锁 / 全部收藏</span>
              </div>
              <div>
                <strong>${{ money(claimed) }}</strong
                ><span>成就已领奖励</span>
              </div>
              <div>
                <strong>{{ pending }}</strong
                ><span>可领取奖励</span>
              </div>
            </div>
            <p class="overview-note">
              达标解锁 · 每枚奖励仅领一次 · 换戴不重复发奖
            </p>
          </div>
        </section>
        <nav class="achievement-tabs" aria-label="成就分类">
          <button
            v-for="t in tabs"
            :key="t.key"
            :aria-current="tab === t.key ? 'page' : undefined"
            :class="{ active: tab === t.key }"
            @click="tab = t.key"
          >
            {{ t.name }}<small>{{ t.count }}</small>
          </button>
        </nav>
        <section v-if="tab === 'sign'" class="sign-section">
          <div class="sign-title">
            <div>
              <p class="eyebrow">THE DAILY CHAPTER</p>
              <h2>今日留痕</h2>
              <p class="muted">每日一次，为坚持留下印记。</p>
            </div>
            <span class="date-label">{{ state.date }} · 北京时间</span>
          </div>
          <div class="sign-layout">
            <div class="sign-calendar">
              <header>
                <div class="calendar-month-nav">
                  <button
                    type="button"
                    aria-label="上个月"
                    :disabled="busy || viewMonth <= minimumMonth"
                    @click="changeMonth(-1)"
                  >
                    ‹
                  </button>
                  <h3>{{ monthTitle }}</h3>
                  <button
                    type="button"
                    aria-label="下个月"
                    :disabled="busy || viewMonth >= state.date.slice(0, 7)"
                    @click="changeMonth(1)"
                  >
                    ›
                  </button>
                </div>
                <span class="gold">点击漏签日期补签</span>
              </header>
              <div class="week">
                <span
                  v-for="w in ['一', '二', '三', '四', '五', '六', '日']"
                  :key="w"
                  >{{ w }}</span
                >
              </div>
              <div class="days">
                <span v-for="n in monthOffset" :key="'blank' + n" /><button
                  v-for="n in daysInMonth"
                  :key="n"
                  type="button"
                  :data-testid="'calendar-day-' + dayString(n)"
                  :aria-label="
                    dayString(n) +
                    (state.calendar.includes(dayString(n))
                      ? '，已签到'
                      : canBackfillDate(dayString(n))
                        ? '，可补签'
                        : '，不可补签')
                  "
                  :disabled="busy || !canBackfillDate(dayString(n))"
                  @click="openCardDialog(dayString(n))"
                  :class="{
                    today: dayString(n) === state.date,
                    checked: state.calendar.includes(dayString(n)),
                    backfillable: canBackfillDate(dayString(n)),
                  }"
                >
                  {{ n
                  }}<small v-if="state.calendar.includes(dayString(n))">✓</small
                  ><small v-else-if="canBackfillDate(dayString(n))">补</small>
                </button>
              </div>
              <p class="calendar-hint">
                带「补」的日期可使用补签卡；切换月份可查看跨月漏签。
              </p>
            </div>
            <div class="sign-benefits">
              <p class="eyebrow">
                {{
                  state.tier
                    ? 'VIP ' + state.tier + ' DAILY PRIVILEGE'
                    : 'DAILY PRIVILEGE'
                }}
              </p>
              <h3>每天相见，每天成长</h3>
              <div class="daily-value">
                ${{ money(state.daily_amount) }}<small>当日签到权益</small>
              </div>
              <p class="muted" data-testid="current-cash-status">
                {{
                  state.today && state.cash_reason === 'eligible'
                    ? '当前签到金额奖励已开放。'
                    : cashReason(state.cash_reason)
                }}
              </p>
              <button
                data-testid="checkin"
                class="gold-button"
                :disabled="busy || !!state.today"
                @click="act('checkin')"
              >
                {{
                  busy
                    ? '正在处理…'
                    : state.today
                      ? '今日已签到'
                      : '留下今日印记 · 签到'
                }}
              </button>
              <p v-if="state.today" class="receipt">
                今日奖励 ${{ money(state.today.gross) }} · 实际到账 ${{
                  money(state.today.net)
                }}<span v-if="state.today.offset_amount">
                  · 抵扣 ${{ money(state.today.offset_amount) }}</span
                >
              </p>
              <p
                v-if="state.today"
                class="small muted"
                data-testid="checkin-receipt-reason"
              >
                {{ recordedCashReason(state.today.reason) }}
              </p>
              <div class="sign-stats">
                <span
                  ><strong>{{ state.streak }}</strong
                  >连续签到</span
                ><span
                  ><strong>{{ state.longest }}</strong
                  >最长连续</span
                ><span
                  ><strong>{{ state.total_days }}</strong
                  >累计签到</span
                >
              </div>
              <p class="small muted">
                奖励按签到时的实际成长等级确定，即时到账；同日升级不会重复发奖。签到不增加充值成长。
              </p>
            </div>
          </div>
          <details class="vip-plan">
            <summary>查看普通会员与 VIP 签到权益</summary>
            <div>
              <span v-for="(amount, index) in state.daily_rewards" :key="index"
                >{{ index ? 'VIP ' + index : '普通会员'
                }}<strong>${{ money(amount) }} / 日</strong></span
              >
            </div>
            <p class="small muted">
              VIP
              规则关闭时按普通会员权益计算。签到金额奖励须由管理员开启，并处于开放范围内，无需近期充值或消费；启用金额上限且额度不足时仍记录签到。
            </p>
          </details>
          <section class="card-panel" aria-label="补签卡">
            <div>
              <p class="eyebrow">CHECK-IN CARDS</p>
              <h3>补上遗漏，继续前行</h3>
              <p class="muted">
                可补最近 30 天的漏签，可跨月；按漏签日历史 VIP
                权益补发金额，并重算连续签到。
              </p>
            </div>
            <div class="card-panel-actions">
              <strong data-testid="card-balance"
                >{{ state.card_balance ?? 0 }} <small>张补签卡</small></strong
              >
              <button
                class="gold-button"
                :disabled="busy || !(state.card_balance ?? 0)"
                @click="focusBackfillCalendar"
              >
                在日历选择日期
              </button>
              <button class="text-button" @click="tab = 'activity'">
                前往活动领取
              </button>
            </div>
          </section>
          <details v-if="state.card_history?.length" class="history">
            <summary>补签卡获取与使用记录</summary>
            <div v-for="h in state.card_history" :key="h.id">
              <span>{{
                h.kind === 'use'
                  ? '补签 ' + h.day
                  : h.kind === 'reclaim'
                    ? '管理员追回'
                    : '领取 ' +
                      (state.medals.find((m) => m.key === h.key)?.name ?? h.key)
              }}</span>
              <span>{{ h.delta > 0 ? '+' : '' }}{{ h.delta }} 张</span>
            </div>
          </details>
        </section>
        <ActivityExplorer
          v-if="tab === 'activity'"
          :passes="state.passes"
          @updated="load"
        />
        <section
          v-if="tab === 'activity'"
          class="series-rewards"
          aria-label="系列收集奖励"
        >
          <article v-for="s in state.series ?? []" :key="s.key">
            <p class="eyebrow">COMPLETE THE COLLECTION</p>
            <h3>{{ s.name }}</h3>
            <p class="muted">
              收集全部
              {{ s.total }} 枚成就，领取系列金额奖励。单枚勋章奖励补签卡。
            </p>
            <div class="progress-label">
              <span>{{ s.collected }} / {{ s.total }} 已收集</span
              ><strong>${{ money(s.reward) }}</strong>
            </div>
            <p
              v-if="s.claim?.prior_amount || s.prior_amount"
              class="small muted"
            >
              已领取的旧活动金额 ${{
                money(s.claim?.prior_amount ?? s.prior_amount ?? 0)
              }}
              已计入系列总奖励。
              <span v-if="!s.claim"
                >本次可领 ${{ money(s.claimable_amount ?? s.reward) }}。</span
              >
            </p>
            <button
              class="text-button"
              :disabled="busy || !canClaimSeries(s)"
              @click="act('claim_series', s.key)"
            >
              {{
                s.claim?.revoked_at
                  ? '已追回'
                  : s.claim
                    ? '已领取'
                    : s.preview
                      ? '后续篇章待开放'
                      : !s.unlocked
                        ? '集齐后领取'
                        : canClaimSeries(s)
                          ? '领取系列奖励'
                          : '金额奖励待开放'
              }}
            </button>
          </article>
        </section>
        <div class="section-caption">
          <span>{{ captions[tab] }}</span
          ><span
            >{{ visible.filter((m) => m.unlocked).length }} /
            {{ visible.length }} 已解锁</span
          >
        </div>
        <section class="medal-grid" aria-label="成就册">
          <article
            v-for="m in visible"
            :key="m.key"
            class="medal-card"
            :class="{ locked: !m.unlocked }"
          >
            <div class="card-top">
              <span>{{ m.key }}</span
              ><span>{{
                m.manual === 'revoked'
                  ? '管理员已取消'
                  : m.manual === 'granted'
                    ? '管理员授予'
                    : m.preview
                      ? '篇章预告'
                      : m.claim?.revoked_at
                        ? '已追回'
                        : m.unlocked
                          ? '✓ 已解锁'
                          : '待点亮'
              }}</span>
            </div>
            <button
              class="art-button"
              :aria-label="'查看' + m.name"
              @click="openDetail(m)"
            >
              <MedalArt :medal="m.key" :name="m.name" />
            </button>
            <h3>{{ m.name }}</h3>
            <p class="condition">{{ condition(m) }}</p>
            <p class="medal-description">{{ m.description }}</p>
            <div class="progress-label">
              <span
                >{{ formatProgress(m.progress) }} /
                {{ formatProgress(m.target) }}</span
              ><span
                >{{
                  Math.min(100, Math.floor((m.progress / m.target) * 100))
                }}%</span
              >
            </div>
            <progress
              :value="Math.min(m.progress, m.target)"
              :max="m.target"
              :aria-label="m.name + '收集进度'"
            />
            <footer>
              <span class="reward"
                ><template v-if="m.card_reward"
                  >{{ m.card_reward }} 张补签卡<small
                    >活动勋章奖励</small
                  ></template
                ><template v-else
                  >${{ money(m.reward) }}<small>成就奖励</small></template
                ></span
              >
              <div>
                <button
                  class="text-button"
                  :data-testid="'claim-' + m.key"
                  :disabled="busy || !canClaim(m)"
                  @click="act('claim', m.key)"
                >
                  {{
                    m.card_reward
                      ? m.card_claim
                        ? m.card_claim.reclaimed
                          ? '已领取 · 有卡片追回'
                          : '补签卡已领取'
                        : m.preview
                          ? '篇章待开放'
                          : !m.unlocked
                            ? '尚未达成'
                            : '领取补签卡'
                      : m.claim?.revoked_at
                        ? '已追回'
                        : m.claim
                          ? '已领取'
                          : !m.unlocked
                            ? '尚未达成'
                            : canClaim(m)
                              ? '领取奖励'
                              : '奖励待开放'
                  }}</button
                ><button
                  class="equip-button"
                  :disabled="busy || !m.unlocked"
                  @click="act('equip', state.equipment === m.key ? '' : m.key)"
                >
                  {{ state.equipment === m.key ? '已佩戴' : '佩戴' }}
                </button>
              </div>
            </footer>
            <p v-if="m.card_claim" class="small muted">
              已使用 {{ m.card_claim.used }} 张 · 已追回
              {{ m.card_claim.reclaimed }} 张 · 可用
              {{
                m.card_claim.amount - m.card_claim.used - m.card_claim.reclaimed
              }}
              张
            </p>
          </article>
        </section>
        <p v-if="tab === 'token'" class="fine-print">
          成长进度从本功能启用后开始累计，以成功提交的付费文本计费为准，缓存
          Token 按互斥桶计数。图片、视频及未计费调用不计入。
        </p>
        <p v-if="tab === 'recharge'" class="fine-print">
          以累计有效充值本金为准，加赠和奖励不计入；退款可能使荣誉锁定，并追回对应已领奖励。
        </p>
        <details v-if="tab === 'sign' && state.history.length" class="history">
          <summary>最近的签到记录</summary>
          <div v-for="h in state.history" :key="h.day">
            <span
              >{{ h.day }} ·
              {{
                h.source === 'admin'
                  ? '管理员补签'
                  : h.source === 'card'
                    ? '补签卡补签'
                    : '用户签到'
              }}
              · 连续 {{ h.streak }} 天</span
            ><span>奖励 ${{ money(h.gross) }} · 到账 ${{ money(h.net) }}</span>
          </div>
        </details>
      </template>
      <dialog
        ref="cardDialog"
        class="medal-dialog card-dialog"
        @close="clearCardPreview"
        @cancel="busy && $event.preventDefault()"
      >
        <button
          class="close"
          :disabled="busy"
          aria-label="关闭补签"
          @click="cardDialog?.close()"
        >
          ×
        </button>
        <p class="eyebrow">RESTORE YOUR DAILY CHAPTER</p>
        <h2>{{ cardDay }} · 补签</h2>
        <p class="muted">
          消耗 1 张补签卡 · 当前剩余 {{ state?.card_balance ?? 0 }} 张
        </p>
        <p class="muted">
          每次消耗 1
          张。确认前核验历史会员权益，服务端校验未通过不扣卡。网络中断时请按原请求重试或刷新查询回执。
        </p>
        <p v-if="busy && !cardPreview" class="muted" role="status">
          正在核验该日历史权益…
        </p>
        <button
          v-if="cardError && !cardCommand"
          class="text-button"
          :disabled="busy || !cardDay"
          @click="prepareCard"
        >
          重新核验
        </button>
        <template v-if="cardPreview">
          <p v-if="cardPreview.available">
            {{ cardDay }} ·
            {{ cardPreview.tier ? 'VIP ' + cardPreview.tier : '普通会员' }} ·
            补发 ${{ money(cardPreview.gross) }}
          </p>
          <p v-else class="muted">
            {{
              cardPreview.existing
                ? '该日已经签到，无需补签。'
                : !cardPreview.policy_known
                  ? '历史权益无法完整核验，不能补签。'
                  : !cardPreview.card_balance
                    ? '补签卡不足。'
                    : cardUnavailableReason(cardPreview.cash_reason)
            }}
          </p>
        </template>
        <p v-if="cardError" class="notice" role="alert">{{ cardError }}</p>
        <button
          class="gold-button"
          :disabled="busy || !cardPreview?.available || !cardCommand"
          @click="spendCard"
        >
          {{
            busy ? '正在处理…' : cardError && cardCommand ? '重试补签' : '补签'
          }}
        </button>
      </dialog>
      <dialog ref="detailDialog" class="medal-dialog">
        <template v-if="selected"
          ><button
            class="close"
            aria-label="关闭详情"
            @click="detailDialog?.close()"
          >
            ×
          </button>
          <p class="eyebrow">{{ selected.key }} / COLLECTION</p>
          <MedalArt
            :medal="selected.key"
            :name="selected.name"
            class="detail-art"
          />
          <h2>{{ selected.name }}</h2>
          <p>{{ selected.description }}</p>
          <p class="muted">{{ condition(selected) }}</p>
          <p>
            {{
              selected.card_reward
                ? selected.card_reward + ' 张补签卡'
                : '成就奖励 $' + money(selected.reward)
            }}
            ·
            {{ selected.unlocked ? '已解锁' : '尚未达成' }}
          </p>
          <p class="small muted">
            佩戴和金额奖励分别记录；更换徽章不会重复发奖。
          </p></template
        >
      </dialog>
    </main></AppLayout
  >
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import MedalArt from '@/components/achievements/MedalArt.vue'
import ActivityExplorer from '@/components/achievements/ActivityExplorer.vue'
import {
  getAchievements,
  changeAchievement,
  type AchievementState,
  type Medal,
  type AchievementSeries,
  type CardPreview,
  type CardCommand,
  previewAchievementCard,
  useAchievementCard,
} from '@/api/achievements'
import { useAuthStore } from '@/stores/auth'
const auth = useAuthStore(),
  state = ref<AchievementState | null>(null),
  error = ref(''),
  message = ref(''),
  busy = ref(false),
  tab = ref('sign'),
  selected = ref<Medal | null>(null),
  detailDialog = ref<HTMLDialogElement>()
const requestKey = ref(crypto.randomUUID())
const cardDialog = ref<HTMLDialogElement>()
const cardDay = ref(''),
  cardError = ref('')
const cardPreview = ref<CardPreview | null>(null)
const cardCommand = ref<CardCommand | null>(null)
const calendarMonth = ref('')
const viewMonth = computed(
  () => calendarMonth.value || state.value?.date.slice(0, 7) || '2026-01',
)
const minimumMonth = computed(
  () =>
    state.value?.card_min_date?.slice(0, 7) ||
    state.value?.date.slice(0, 7) ||
    viewMonth.value,
)
const missingCardDays = computed(() => {
  if (!state.value?.card_min_date || !state.value.card_max_date) return []
  const dates: string[] = []
  const date = new Date(state.value.card_max_date + 'T00:00:00Z')
  while (
    date.toISOString().slice(0, 10) >= state.value.card_min_date &&
    dates.length < 30
  ) {
    const day = date.toISOString().slice(0, 10)
    if (!state.value.calendar.includes(day)) dates.push(day)
    date.setUTCDate(date.getUTCDate() - 1)
  }
  return dates
})
const tabs = [
  { key: 'sign', name: '签到', count: 3 },
  { key: 'token', name: 'Token 成长', count: 6 },
  { key: 'recharge', name: '充值荣誉', count: 3 },
  { key: 'activity', name: '活动探索', count: 9 },
]
const captions: Record<string, string> = {
  sign: '日序系列 · 收藏坚持的时光',
  token: '星核系列 · 六个成长阶段',
  recharge: '铭印系列 · 珍藏每一份支持',
  activity: '探索系列 · 知识、实践与篇章',
}
const visible = computed(
  () => state.value?.medals.filter((m) => m.category === tab.value) ?? [],
)
const equipped = computed(() =>
  state.value?.medals.find((m) => m.key === state.value?.equipment),
)
const unlocked = computed(
  () => state.value?.medals.filter((m) => m.unlocked).length ?? 0,
)
const claimed = computed(
  () =>
    (state.value?.medals.reduce(
      (v, m) => v + (m.claim && !m.claim.revoked_at ? m.claim.gross : 0),
      0,
    ) ?? 0) +
    (state.value?.series?.reduce(
      (v, s) => v + (s.claim && !s.claim.revoked_at ? s.claim.gross : 0),
      0,
    ) ?? 0),
)
const pending = computed(
  () =>
    (state.value?.medals.filter(canClaim).length ?? 0) +
    (state.value?.series?.filter(canClaimSeries).length ?? 0),
)
const monthTitle = computed(() => viewMonth.value.replace('-', ' 年 ') + ' 月')
const monthOffset = computed(() => {
  const d = new Date(viewMonth.value + '-01T00:00:00Z')
  return (d.getUTCDay() + 6) % 7
})
const daysInMonth = computed(() => {
  const [y, m] = viewMonth.value.split('-').map(Number)
  return new Date(Date.UTC(y!, m!, 0)).getUTCDate()
})
const dayString = (n: number) =>
  viewMonth.value + '-' + String(n).padStart(2, '0')
function canBackfillDate(day: string) {
  return (
    !!state.value?.card_min_date &&
    !!state.value.card_max_date &&
    day >= state.value.card_min_date &&
    day <= state.value.card_max_date &&
    !state.value.calendar.includes(day)
  )
}
function changeMonth(offset: number) {
  const [year, month] = viewMonth.value.split('-').map(Number)
  const next = new Date(Date.UTC(year!, month! - 1 + offset, 1))
    .toISOString()
    .slice(0, 7)
  if (
    next >= minimumMonth.value &&
    next <= (state.value?.date.slice(0, 7) ?? viewMonth.value)
  )
    calendarMonth.value = next
}
const money = (v: number) => Number(v || 0).toFixed(2)
const formatProgress = (v: number) =>
  v >= 1e8
    ? (v / 1e8).toLocaleString('zh-CN') + ' 亿'
    : v >= 1e4
      ? (v / 1e4).toLocaleString('zh-CN') + ' 万'
      : v.toLocaleString('zh-CN')
const condition = (m: Medal) =>
  m.category === 'token'
    ? '累计 ' + formatProgress(m.target) + ' Token'
    : m.category === 'sign'
      ? '连续签到 ' + m.target + ' 天'
      : m.category === 'recharge'
        ? '累计有效充值 $' + formatProgress(m.target)
        : m.key.startsWith('A-K')
          ? '通过 ' + m.target + ' 个知识主题'
          : m.key.startsWith('A-X')
            ? '通过 ' + m.target + ' 个实践关卡'
            : '收藏 ' + m.target + ' 个活动篇章'
const cashReason = (reason: string) =>
  ({
    eligible: '今日现金奖励已开放，签到后即时入账。',
    cash_disabled: '当前现金奖励未开放，签到仍计入成长。',
    not_in_cash_pilot: '当前账户不在奖励开放名单，签到仍计入成长。',
    recent_activity_required:
      '近 30 日需有有效充值或余额计费使用，签到仍计入成长。',
    budget_exhausted: '本期奖励预算已用完，签到仍计入成长。',
    account_unavailable: '当前账户不可领取签到金额奖励。',
    admin_backfill: '管理员已补签，奖励按核验的历史权益即时补发。',
    card_backfill: '已使用补签卡，奖励按漏签日历史权益即时补发。',
  })[reason] ?? reason
const recordedCashReason = (reason: string) =>
  ({
    eligible: '本次签到奖励已处理，到账金额以回执为准。',
    cash_disabled: '本次签到时金额奖励尚未开启，因此未发放金额奖励。',
    not_in_cash_pilot: '本次签到时账户不在奖励开放名单，因此未发放金额奖励。',
    recent_activity_required:
      '本次签到时未满足近 30 日有效充值或余额计费使用条件，因此未发放金额奖励。',
    budget_exhausted: '本次签到时奖励额度不足，因此未发放金额奖励。',
    admin_backfill: '管理员已补签，奖励按核验的历史权益即时补发。',
    card_backfill: '已使用补签卡，奖励按漏签日历史权益即时补发。',
  })[reason] ?? reason
function canClaim(m: Medal) {
  if (m.card_reward) return m.unlocked && !m.preview && !m.card_claim
  return (
    m.unlocked &&
    !m.claim &&
    state.value?.milestone_cash_enabled &&
    (state.value.milestone_cash_reason ?? state.value.cash_reason) ===
      'eligible'
  )
}
function canClaimSeries(s: AchievementSeries) {
  return (
    s.unlocked &&
    !s.claim &&
    state.value?.milestone_cash_enabled &&
    (state.value.milestone_cash_reason ?? state.value.cash_reason) ===
      'eligible'
  )
}
function cardUnavailableReason(reason: string) {
  if (reason === 'cash_disabled')
    return '签到金额奖励尚未开启，暂不能使用补签卡，卡片保留。'
  if (reason === 'not_in_cash_pilot')
    return '账户不在签到奖励开放范围，暂不能使用补签卡，卡片保留。'
  return '当前无法使用补签卡，请刷新核对状态，卡片保留。'
}
function clearCardPreview() {
  cardPreview.value = null
  cardCommand.value = null
  cardError.value = ''
}
async function openCardDialog(day: string) {
  if (busy.value || !canBackfillDate(day)) return
  cardDay.value = day
  clearCardPreview()
  cardDialog.value?.showModal()
  await prepareCard()
}
function focusBackfillCalendar() {
  const day = missingCardDays.value[0]
  if (!day) {
    message.value = '最近 30 天内没有可补签的漏签日期。'
    return
  }
  calendarMonth.value = day.slice(0, 7)
  document
    .querySelector('.sign-calendar')
    ?.scrollIntoView({ block: 'center', behavior: 'smooth' })
}
async function prepareCard() {
  if (busy.value || !cardDay.value) return
  busy.value = true
  clearCardPreview()
  try {
    const p = await previewAchievementCard(cardDay.value)
    cardPreview.value = p
    if (p.available)
      cardCommand.value = {
        date: cardDay.value,
        expected_gross: p.gross,
        expected_tier: p.tier,
        request_key: crypto.randomUUID(),
      }
  } catch (e) {
    cardError.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
async function spendCard() {
  if (busy.value || !cardCommand.value || !cardPreview.value?.available) return
  busy.value = true
  cardError.value = ''
  try {
    const r = await useAchievementCard(cardCommand.value)
    message.value = r.existing
      ? '该日已签到，未扣除补签卡。'
      : `补签成功：使用 ${r.cards_spent} 张，补发 $${money(r.gross)}，实际到账 $${money(r.net)}。`
    cardDialog.value?.close()
    clearCardPreview()
    await auth.refreshUser()
    await load()
  } catch (e) {
    cardError.value = errorMessage(e)
    const failure = e as {
      code?: string
      response?: { data?: { code?: string } }
    }
    if (
      (failure.code ?? failure.response?.data?.code) ===
      'ACHIEVEMENT_HISTORY_CHANGED'
    ) {
      cardCommand.value = null
      cardPreview.value = null
    }
  } finally {
    busy.value = false
  }
}
const errorMessage = (e: unknown) => {
  const v = e as {
    message?: string
    response?: { data?: { message?: string } }
  }
  return v.response?.data?.message ?? v.message ?? '暂时无法连接，请稍后重试'
}
async function load() {
  try {
    state.value = await getAchievements()
    if (
      calendarMonth.value &&
      (calendarMonth.value <
        (state.value.card_min_date?.slice(0, 7) ??
          state.value.date.slice(0, 7)) ||
        calendarMonth.value > state.value.date.slice(0, 7))
    )
      calendarMonth.value = ''
    error.value = ''
  } catch (e) {
    error.value = errorMessage(e)
  }
}
async function act(
  action: 'checkin' | 'claim' | 'claim_series' | 'equip',
  key?: string,
) {
  if (busy.value || !state.value) return
  busy.value = true
  error.value = ''
  message.value = ''
  try {
    if (
      action === 'checkin' ||
      action === 'claim' ||
      action === 'claim_series'
    ) {
      const r = await changeAchievement(action, {
        key,
        date: state.value.date,
        request_key: requestKey.value,
      })
      message.value =
        r.cards_awarded !== undefined
          ? `已领取 ${r.cards_awarded} 张补签卡，剩余 ${r.card_balance} 张。`
          : `${action === 'checkin' ? '签到成功' : '奖励已领取'}：奖励 $${money(r.gross)}，实际到账 $${money(r.net)}${r.offset_amount ? '，抵扣待追回金额 $' + money(r.offset_amount) : ''}`
      requestKey.value = crypto.randomUUID()
      await auth.refreshUser()
    } else {
      await changeAchievement('equip', { key })
      message.value = key ? '徽章已佩戴' : '徽章已卸下'
    }
    await load()
  } catch (e) {
    error.value = errorMessage(e)
  } finally {
    busy.value = false
  }
}
function openDetail(m: Medal) {
  selected.value = m
  detailDialog.value?.showModal()
}
onMounted(load)
</script>
<style scoped>
.card-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 28px;
  margin-top: 26px;
  padding: 24px;
  border: 1px solid var(--line);
  background: var(--wash);
}
.card-panel h3,
.series-rewards h3 {
  margin: 8px 0 12px;
  font-size: 22px;
}
.card-panel-actions {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 10px;
  flex-shrink: 0;
}
.card-panel-actions strong {
  text-align: center;
  font-size: 30px;
  color: var(--gold);
}
.card-panel-actions strong small {
  font-size: 12px;
  font-weight: normal;
}
.series-rewards {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
  margin: 24px 0;
}
.series-rewards article {
  padding: 22px;
  border: 1px solid var(--line);
  background: var(--wash);
}
.series-rewards .text-button {
  margin-top: 14px;
}
.card-dialog {
  width: min(520px, 92vw);
}
.card-dialog .gold-button {
  width: 100%;
  margin-top: 18px;
}
@media (max-width: 760px) {
  .card-panel {
    flex-direction: column;
    align-items: stretch;
  }
  .series-rewards {
    grid-template-columns: 1fr;
  }
}
.achievement-page {
  --paper: #fffefa;
  --ink: #292c29;
  --muted: #73776e;
  --gold: #806635;
  --line: #e3dfd3;
  --wash: #f5f3ec;
  color: var(--ink);
  max-width: 1460px;
  margin: auto;
  padding: 34px 30px 56px;
}
.heading {
  display: flex;
  justify-content: space-between;
  align-items: end;
  gap: 24px;
  margin-bottom: 28px;
}
.eyebrow {
  font:
    10px Consolas,
    monospace;
  letter-spacing: 2px;
  color: var(--gold);
}
h1,
h2,
h3 {
  font-family: 'Noto Serif SC', 'SimSun', serif;
  font-weight: 500;
}
h1 {
  font-size: 30px;
  letter-spacing: 2px;
  margin: 8px 0;
}
h2 {
  font-size: 23px;
  margin: 7px 0;
}
h3 {
  font-size: 23px;
}
.muted {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.8;
}
.small {
  font-size: 11px;
  line-height: 1.8;
}
.gold {
  color: var(--gold);
}
.edition {
  text-align: right;
  display: grid;
  gap: 5px;
  font:
    10px Consolas,
    monospace;
  color: var(--muted);
}
.edition strong {
  font:
    20px 'SimSun',
    serif;
  color: var(--gold);
}
button {
  cursor: pointer;
}
button:disabled {
  cursor: default;
  opacity: 0.45;
}
button:focus-visible,
summary:focus-visible {
  outline: 2px solid var(--gold);
  outline-offset: 4px;
}
.hero {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  border: 1px solid var(--line);
  background: var(--paper);
  margin-bottom: 30px;
}
.wear-panel {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 25px 30px;
  background: radial-gradient(ellipse at 25% 50%, #e5d6b62a, transparent);
}
.hero-art {
  width: 126px;
  flex-shrink: 0;
}
.empty-medal {
  width: 126px;
  height: 126px;
  border: 1px solid var(--line);
  border-radius: 50%;
  display: grid;
  place-items: center;
  color: var(--gold);
  font:
    35px 'Times New Roman',
    serif;
  flex-shrink: 0;
}
.overview {
  padding: 30px;
  border-left: 1px solid var(--line);
  display: grid;
  align-content: center;
}
.stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
}
.stats > div {
  padding: 0 18px;
  border-right: 1px solid var(--line);
}
.stats > div:first-child {
  padding-left: 0;
}
.stats > div:last-child {
  border: 0;
}
.stats strong {
  font:
    27px Consolas,
    monospace;
}
.stats small {
  font-size: 12px;
  color: var(--muted);
}
.stats span {
  display: block;
  font-size: 11px;
  color: var(--muted);
  margin-top: 6px;
}
.overview-note {
  border-top: 1px solid var(--line);
  padding-top: 16px;
  margin-top: 22px;
  color: var(--muted);
  font-size: 10px;
}
.achievement-tabs {
  display: flex;
  border-bottom: 1px solid var(--line);
  gap: 28px;
}
.achievement-tabs button {
  padding: 15px 0;
  background: none;
  border-bottom: 2px solid transparent;
  color: var(--muted);
  white-space: nowrap;
}
.achievement-tabs .active {
  color: var(--gold);
  border-color: var(--gold);
}
.achievement-tabs small {
  font: 10px Consolas;
  margin-left: 9px;
  opacity: 0.65;
}
.sign-section {
  margin-top: 27px;
  background: var(--paper);
  border: 1px solid var(--line);
  padding: 26px;
}
.sign-title,
.sign-calendar header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
.date-label {
  font: 11px Consolas;
  color: var(--muted);
}
.sign-layout {
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  gap: 46px;
  margin-top: 24px;
}
.sign-calendar {
  border: 1px solid var(--line);
  padding: 20px;
}
.sign-calendar h3 {
  font:
    18px 'SimSun',
    serif;
}
.sign-calendar header span {
  font-size: 11px;
}
.calendar-month-nav {
  display: flex;
  align-items: center;
  gap: 8px;
}
.calendar-month-nav button {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  border: 1px solid var(--line);
  color: var(--gold);
  background: transparent;
  font-size: 22px;
}
.calendar-month-nav button:disabled {
  opacity: 0.35;
  cursor: default;
}
.calendar-hint {
  margin: 16px 0 0;
  color: var(--muted);
  font-size: 11px;
  line-height: 1.8;
}
@media (max-width: 720px) {
  .sign-calendar header {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
  .calendar-month-nav {
    width: 100%;
    justify-content: space-between;
  }
  .calendar-month-nav h3 {
    white-space: nowrap;
  }
}
.week,
.days {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 7px;
  text-align: center;
}
.week {
  font-size: 11px;
  color: var(--muted);
  margin: 18px 0 10px;
}
.days > span,
.days > button {
  min-height: 40px;
  display: grid;
  place-content: center;
  font: 13px Consolas;
  border: 1px solid transparent;
  position: relative;
}
.days > button {
  color: var(--ink);
  background: transparent;
  border-radius: 3px;
}
.days > button:disabled {
  cursor: default;
}
.days .backfillable {
  border-color: var(--line);
  color: var(--gold);
  background: var(--wash);
  cursor: pointer;
}
.days .backfillable:hover {
  border-color: var(--gold);
}
.days > button:focus-visible,
.calendar-month-nav button:focus-visible {
  outline: 2px solid var(--gold);
  outline-offset: 2px;
}
.days .today {
  border-color: var(--gold);
}
.days .checked {
  background: #b4925420;
  color: var(--gold);
}
.days small {
  position: absolute;
  right: 4px;
  bottom: 2px;
  font-size: 9px;
}
.sign-benefits h3 {
  margin: 9px 0;
}
.daily-value {
  font:
    38px 'Times New Roman',
    serif;
  color: var(--gold);
  margin: 16px 0;
}
.daily-value small {
  display: block;
  font:
    11px 'Microsoft YaHei',
    sans-serif;
  color: var(--muted);
  margin-top: 4px;
}
.gold-button {
  background: var(--gold);
  color: var(--paper);
  border: 0;
  padding: 13px 20px;
  margin: 15px 0 8px;
  width: 100%;
  font-size: 13px;
}
.receipt {
  font-size: 11px;
  line-height: 1.8;
  color: var(--gold);
}
.sign-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  margin: 17px 0;
  padding-top: 15px;
  border-top: 1px solid var(--line);
}
.sign-stats span {
  font-size: 11px;
  color: var(--muted);
}
.sign-stats strong {
  display: block;
  font: 23px Consolas;
  color: var(--ink);
  margin-bottom: 6px;
}
.vip-plan {
  margin-top: 24px;
  border-top: 1px solid var(--line);
  padding-top: 15px;
  font-size: 12px;
}
.vip-plan > div {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 15px;
  padding: 18px 0;
}
.vip-plan strong {
  display: block;
  color: var(--gold);
  font: 13px Consolas;
  margin-top: 8px;
}
summary {
  cursor: pointer;
}
.section-caption {
  display: flex;
  justify-content: space-between;
  gap: 15px;
  font-size: 11px;
  color: var(--muted);
  margin: 24px 0 16px;
}
.medal-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}
.medal-card {
  border: 1px solid var(--line);
  background: var(--paper);
  padding: 22px 24px;
  text-align: center;
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.card-top {
  display: flex;
  justify-content: space-between;
  font:
    11px Consolas,
    monospace;
  color: var(--muted);
}
.card-top span:last-child {
  color: var(--gold);
}
.art-button {
  width: 180px;
  height: 220px;
  display: flex;
  align-items: center;
  margin: 17px auto 13px;
  border: 0;
  background: transparent;
}
.locked .art-button {
  opacity: 0.7;
}
.medal-card h3 {
  font-size: 25px;
  letter-spacing: 2px;
  margin-bottom: 8px;
}
.condition {
  font-size: 11px;
  color: var(--muted);
  margin: 0 0 12px;
}
.medal-description {
  font:
    13px 'SimSun',
    serif;
  line-height: 1.8;
  color: var(--muted);
  min-height: 38px;
}
.progress-label {
  display: flex;
  justify-content: space-between;
  font: 10px Consolas;
  color: var(--muted);
  margin: 18px 0 7px;
}
progress {
  appearance: none;
  height: 3px;
  width: 100%;
  border: 0;
  background: var(--line);
}
progress::-webkit-progress-bar {
  background: var(--line);
}
progress::-webkit-progress-value {
  background: var(--gold);
}
progress::-moz-progress-bar {
  background: var(--gold);
}
.medal-card footer {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  border-top: 1px solid var(--line);
  margin-top: 21px;
  padding-top: 16px;
  text-align: left;
}
.reward {
  font:
    20px 'Times New Roman',
    serif;
  color: var(--gold);
}
.reward small {
  display: block;
  font:
    9px 'Microsoft YaHei',
    sans-serif;
  margin-top: 3px;
  color: var(--muted);
}
.text-button {
  border: 0;
  background: transparent;
  color: var(--gold);
  font-size: 11px;
  padding: 7px;
}
.equip-button {
  border: 1px solid var(--line);
  background: transparent;
  font-size: 11px;
  padding: 6px 11px;
  color: var(--ink);
}
.fine-print,
.history {
  font-size: 11px;
  color: var(--muted);
  line-height: 1.9;
  margin-top: 22px;
}
.history > div {
  display: flex;
  justify-content: space-between;
  border-bottom: 1px solid var(--line);
  padding: 12px 0;
}
.notice {
  padding: 14px 18px;
  border: 1px solid var(--line);
  background: var(--paper);
  color: var(--gold);
  font-size: 12px;
  margin: 16px 0;
}
.notice button {
  background: none;
  border: 0;
  text-decoration: underline;
  margin-left: 12px;
}
.loading {
  padding: 60px;
  text-align: center;
  color: var(--muted);
}
.medal-dialog {
  background: var(--paper);
  color: var(--ink);
  border: 1px solid var(--line);
  padding: 35px;
  text-align: center;
  width: min(430px, 90vw);
  max-height: 90vh;
  overflow: auto;
}
.medal-dialog::backdrop {
  background: #10181299;
}
.medal-dialog p {
  margin: 16px 0;
  line-height: 1.8;
  font-size: 13px;
}
.detail-art {
  width: 190px;
  margin: 20px auto;
}
.close {
  position: absolute;
  right: 13px;
  top: 9px;
  border: 0;
  background: none;
  color: var(--muted);
  font-size: 25px;
}
:global(.dark .achievement-page) {
  --paper: #1e2421;
  --ink: #e6e2d6;
  --muted: #a0a497;
  --gold: #d2ba86;
  --line: #3b4239;
  --wash: #181e1a;
}
@media (max-width: 1050px) {
  .hero {
    grid-template-columns: 1fr;
  }
  .overview {
    border-left: 0;
    border-top: 1px solid var(--line);
  }
  .sign-layout {
    gap: 24px;
  }
  .art-button {
    width: 150px;
    height: 210px;
  }
  .medal-card {
    padding: 20px 16px;
  }
  .medal-card footer {
    flex-wrap: wrap;
  }
  .vip-plan > div {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (max-width: 720px) {
  .achievement-page {
    padding: 23px 14px;
  }
  .heading {
    align-items: start;
  }
  .edition {
    display: none;
  }
  .wear-panel {
    padding: 20px;
    gap: 18px;
  }
  .hero-art,
  .empty-medal {
    width: 90px;
  }
  .empty-medal {
    height: 90px;
  }
  .stats strong {
    font-size: 23px;
  }
  .overview {
    padding: 22px;
  }
  .achievement-tabs {
    gap: 22px;
    overflow-x: auto;
  }
  .sign-layout {
    grid-template-columns: 1fr;
  }
  .sign-title {
    align-items: start;
    flex-direction: column;
  }
  .sign-section {
    padding: 20px 15px;
  }
  .medal-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 10px;
  }
  .medal-card {
    padding: 15px 10px;
  }
  .art-button {
    width: 110px;
    height: 165px;
  }
  .medal-card h3 {
    font-size: 20px;
    letter-spacing: 1px;
  }
  .medal-description {
    font-size: 12px;
    min-height: 45px;
  }
  .card-top {
    font-size: 9px;
  }
  .sign-calendar {
    padding: 15px;
  }
  .days > span,
  .days > button {
    min-height: 35px;
  }
  .history > div {
    gap: 12px;
  }
  .medal-card footer > div {
    display: flex;
    flex-wrap: wrap;
  }
  .reward {
    font-size: 18px;
  }
}
</style>
