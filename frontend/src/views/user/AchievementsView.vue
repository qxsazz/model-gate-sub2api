<template>
  <AppLayout
    ><main class="achievement-page">
      <p v-if="error" class="notice" role="alert">
        {{ error }} <button :disabled="busy" @click="load">重试</button>
      </p>
      <p v-if="message" class="achievement-toast" role="status">
        {{ message }}
      </p>
      <p v-if="!state && !error" class="loading">正在读取你的成就记录…</p>
      <template v-if="state">
        <section class="hero account-summary">
          <div class="wear-panel equipped">
            <button
              v-if="equipped"
              class="medal-button equipped-art"
              :aria-label="'查看' + equipped.name + '介绍'"
              @click="openDetail(equipped)"
            >
              <MedalArt :medal="equipped.key" :name="equipped.name" />
            </button>
            <div v-else class="empty-medal">MG</div>
            <div>
              <span class="eyebrow">当前佩戴</span>
              <h2>{{ equipped?.name ?? '静待第一枚印记' }}</h2>
              <p>
                {{
                  equipped
                    ? condition(equipped)
                    : '达成成就后，即可将徽章佩戴于此。'
                }}
              </p>
              <button
                v-if="equipped"
                class="text-link"
                @click="openDetail(equipped)"
              >
                查看徽章 <AchievementIcon name="arrow" /></button
              ><span v-else class="text-link">收藏从今天开始</span>
            </div>
          </div>
          <div class="overview">
            <div class="stats summary-metrics">
              <div class="metric">
                <strong
                  >{{ unlocked
                  }}<small> / {{ state.medals.length || 21 }}</small></strong
                ><span>已解锁徽章</span>
              </div>
              <div class="metric">
                <strong>{{ pending }}</strong
                ><span>待领取奖励</span
                ><button :disabled="!pending" @click="openPending">
                  查看可领取
                </button>
              </div>
              <div class="metric">
                <strong data-testid="card-balance">{{
                  state.card_balance ?? 0
                }}</strong
                ><span>可用补签卡</span
                ><button @click="goBackfill">去补签</button>
              </div>
            </div>
          </div>
        </section>
        <nav class="achievement-tabs main-tabs" aria-label="成就分类">
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
          <div class="sign-title section-head">
            <div>
              <h2>每日签到</h2>
              <p>每日一次，收下今天的奖励，也收下一点向前的能量。</p>
            </div>
            <span class="muted mono">北京时间 · {{ state.date }}</span>
          </div>
          <div class="sign-layout checkin-layout">
            <div class="calendar-stack">
              <section class="sign-calendar calendar-panel">
                <div class="calendar-month-nav calendar-head">
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
                    :disabled="busy || viewMonth >= maximumMonth"
                    @click="changeMonth(1)"
                  >
                    ›
                  </button>
                </div>
                <div class="week calendar-week">
                  <span
                    v-for="w in ['一', '二', '三', '四', '五', '六', '日']"
                    :key="w"
                    >{{ w }}</span
                  >
                </div>
                <div class="days calendar-grid">
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
                          : '，查看小笺')
                    "
                    :disabled="busy"
                    @click="selectCalendarDate(dayString(n))"
                    :class="{
                      today: dayString(n) === state.date,
                      'selected-day': dayString(n) === companionDate,
                      signed: state.calendar.includes(dayString(n)),
                      makeup: canBackfillDate(dayString(n)),
                    }"
                  >
                    {{ n
                    }}<small v-if="state.calendar.includes(dayString(n))"
                      >✓</small
                    ><small v-else-if="canBackfillDate(dayString(n))">补</small>
                  </button>
                </div>
                <div class="calendar-legend">
                  <span>✓ 已签到</span><span>补 可补签</span
                  ><span
                    >选中 {{ companionDate.slice(5).replace('-', ' / ') }}</span
                  >
                </div>
                <p class="quiet-note calendar-hint">
                  点击日期查看小笺；带「补」的漏签日期可使用补签卡。
                </p>
                <section class="token-inspiration" aria-label="Token 使用灵感">
                  <div>
                    <span class="eyebrow">TOKEN · 今日使用灵感</span>
                    <h4>{{ tokenTheme.name }}</h4>
                    <p>{{ tokenTheme.text }}</p>
                  </div>
                  <button
                    class="text-link"
                    aria-label="复制模型提问示例"
                    @click="copyPrompt"
                  >
                    复制提问 <AchievementIcon name="copy" />
                  </button>
                </section>
              </section>
            </div>
            <section class="sign-benefits daily-panel daily-companion">
              <div class="daily-head daily-action-top">
                <div>
                  <p class="eyebrow">
                    {{
                      state.tier
                        ? 'VIP ' + state.tier + ' 每日权益'
                        : '普通会员每日权益'
                    }}
                  </p>
                  <h2>
                    {{ state.today ? '今天，已如约而至' : '收下今天的小确幸' }}
                  </h2>
                </div>
                <div class="daily-value daily-amount">
                  ${{ money(state.daily_amount) }}<small>今日签到奖励</small>
                </div>
              </div>
              <p class="policy-status" data-testid="current-cash-status">
                {{
                  state.today && state.cash_reason === 'eligible'
                    ? '当前签到金额奖励已开放。'
                    : cashReason(state.cash_reason)
                }}
              </p>
              <button
                data-testid="checkin"
                class="gold-button primary"
                :disabled="busy || !!state.today"
                @click="act('checkin')"
              >
                {{
                  busy
                    ? '正在处理…'
                    : state.today
                      ? '今日已签到 · 明天再见'
                      : '立即签到'
                }}
              </button>
              <template v-if="state.today"
                ><p class="receipt">
                  今日奖励 ${{ money(state.today.gross) }} · 实际到账 ${{
                    money(state.today.net)
                  }}<span v-if="state.today.offset_amount">
                    · 抵扣 ${{ money(state.today.offset_amount) }}</span
                  >
                </p>
                <p class="policy-status" data-testid="checkin-receipt-reason">
                  {{ recordedCashReason(state.today.reason) }}
                </p></template
              >
              <div class="sign-stats day-stats">
                <div>
                  <strong>{{ state.streak }}</strong
                  ><span>连续签到</span>
                </div>
                <div>
                  <strong>{{ state.longest }}</strong
                  ><span>最长连续</span>
                </div>
                <div>
                  <strong>{{ state.card_balance ?? 0 }}</strong
                  ><span>可用补签卡</span>
                </div>
              </div>
              <div class="companion-time">
                <span>我们相伴的第</span
                ><strong
                  >{{ state.companionship_days ?? '—'
                  }}<small>天</small></strong
                >
                <p>
                  从 {{ joinedDateLabel }} 开始，一次次相逢，慢慢变成了日常。
                </p>
              </div>
              <div v-if="nextSignMedal" class="next-sign-medal daily-milestone">
                <MedalArt
                  :medal="nextSignMedal.key"
                  :name="nextSignMedal.name"
                  class="milestone-art"
                />
                <div>
                  <p class="eyebrow">下一枚签到徽章</p>
                  <h3>{{ nextSignMedal.name }}</h3>
                  <p>
                    目标连续 {{ nextSignMedal.target }} 天 · 历史最长
                    {{ state.longest }} 天
                  </p>
                  <progress
                    class="badge-progress"
                    :value="Math.min(state.longest, nextSignMedal.target)"
                    :max="nextSignMedal.target"
                  />
                </div>
                <button class="text-link" @click="openDetail(nextSignMedal)">
                  查看 <AchievementIcon name="arrow" />
                </button>
              </div>
              <div v-else class="token-gentle">
                <span class="eyebrow">签到系列已集齐</span>
                <p>四季里的坚持，都已收入你的收藏。</p>
              </div>
              <div class="daily-encouragement token-gentle">
                <span class="eyebrow">把好奇，交给一次认真提问</span>
                <p>从一个真实的问题开始，让 Token 为你的想法添一点光。</p>
              </div>
              <p class="companion-whisper">
                日子有快有慢，每一次回来，都值得被好好记住。
              </p>
            </section>
          </div>
          <AchievementCompanion
            :state="state"
            :date="companionDate"
            @updated="load"
          />
          <details class="vip-plan">
            <summary>查看普通会员与 VIP 签到权益</summary>
            <div>
              <span v-for="(amount, index) in state.daily_rewards" :key="index"
                >{{ index ? 'VIP ' + index : '普通会员'
                }}<strong>${{ money(amount) }} / 日</strong></span
              >
            </div>
            <p>
              奖励按签到时的实际成长等级确定，即时到账；同日升级不会重复发奖。VIP
              规则关闭时按普通会员权益计算，签到不增加充值成长。
            </p>
          </details>
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
              }}</span
              ><span>{{ h.delta > 0 ? '+' : '' }}{{ h.delta }} 张</span>
            </div>
          </details>
        </section>
        <template v-if="tab === 'activity'">
          <nav class="activity-groups group-tabs" aria-label="活动类型">
            <button
              v-for="g in activityGroups"
              :key="g.key"
              :class="{ active: activityGroup === g.key }"
              :aria-pressed="activityGroup === g.key"
              @click="chooseActivityGroup(g.key)"
            >
              {{ g.name }}<small>{{ groupProgress(g.key) }}</small>
            </button>
          </nav>
          <div class="section-head group-heading">
            <div>
              <h2>{{ activityName }}</h2>
              <p>{{ activityDescription }}</p>
            </div>
            <div
              v-for="s in visibleSeries"
              :key="s.key"
              class="series-summary series-rewards"
              id="activity-series"
            >
              <AchievementIcon name="gift" />
              <div>
                <span>集齐本系列 {{ s.total }} 枚</span
                ><small>{{
                  s.preview ? '后续篇章开放后可领取' : '额外获得系列金额奖励'
                }}</small
                ><button
                  class="text-link"
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
              </div>
              <strong>${{ money(s.reward) }}</strong>
            </div>
          </div>
          <p
            v-for="s in visibleSeries.filter(
              (s) => s.prior_amount || s.claim?.prior_amount,
            )"
            :key="s.key"
            class="quiet-note"
          >
            已领取的旧活动金额 ${{
              money(s.claim?.prior_amount ?? s.prior_amount ?? 0)
            }}
            已计入系列总奖励。<span v-if="!s.claim"
              >本次可领 ${{ money(s.claimable_amount ?? s.reward) }}。</span
            >
          </p>
        </template>
        <div v-else class="section-head collection-head">
          <div>
            <h2>{{ captions[tab] }}</h2>
            <p>{{ collectionDescription }}</p>
          </div>
          <span v-if="tab !== 'sign'" class="muted">{{
            collectionCounter
          }}</span>
        </div>
        <section
          class="medal-grid badge-grid"
          :class="{
            'collection-grid': tab !== 'activity',
            'sign-badges': tab === 'sign',
          }"
          aria-label="成就册"
        >
          <article
            v-for="m in visible"
            :key="m.key"
            class="medal-card badge-card"
            :class="{ locked: !m.unlocked }"
          >
            <div class="card-top badge-state">
              <span class="mono">阶段 {{ phase(m) }}</span
              ><span class="status" :class="{ unlocked: m.unlocked }"
                ><AchievementIcon v-if="m.unlocked" name="check" />{{
                  medalStatus(m)
                }}</span
              >
            </div>
            <button
              class="art-button medal-button"
              :aria-label="'查看' + m.name"
              @click="openDetail(m)"
            >
              <MedalArt :medal="m.key" :name="m.name" class="badge-art" />
            </button>
            <h3>{{ m.name }}</h3>
            <p class="condition badge-requirement">{{ condition(m) }}</p>
            <progress
              class="badge-progress"
              :aria-label="m.name + '收集进度'"
              :value="Math.min(m.progress, m.target)"
              :max="m.target"
            />
            <div class="progress-label badge-progress-label">
              <span
                >{{ formatProgress(Math.min(m.progress, m.target)) }} /
                {{ formatProgress(m.target) }}</span
              ><span>{{
                m.unlocked
                  ? '已达成'
                  : '还差 ' + formatProgress(Math.max(0, m.target - m.progress))
              }}</span>
            </div>
            <footer class="badge-footer">
              <span class="reward badge-reward"
                ><AchievementIcon :name="m.card_reward ? 'ticket' : 'coin'" />{{
                  m.card_reward
                    ? m.card_reward + ' 张补签卡'
                    : '$' + money(m.reward)
                }}</span
              ><button
                class="text-button small-button"
                :data-testid="'claim-' + m.key"
                :disabled="busy || m.preview"
                @click="medalAction(m)"
              >
                {{ medalActionLabel(m) }}
              </button>
            </footer>
            <p v-if="m.card_claim" class="claim-record">
              已使用 {{ m.card_claim.used }} 张 · 已追回
              {{ m.card_claim.reclaimed }} 张 · 可用
              {{
                m.card_claim.amount - m.card_claim.used - m.card_claim.reclaimed
              }}
              张
            </p>
          </article>
        </section>
        <p v-if="tab === 'activity'" class="badge-swipe-hint">
          左右滑动，查看本系列的 3 枚勋章
        </p>
        <ActivityExplorer
          v-if="tab === 'activity'"
          id="activity-tasks"
          :group="activityGroup"
          :goal="nextActivityMedal"
          :series-reward="visibleSeries[0]?.reward"
          :passes="state.passes"
          @updated="load"
          @show-series="documentScrollSeries"
        />
        <p v-if="tab === 'token'" class="fine-print">
          成长进度从本功能启用后开始累计，以成功提交的付费文本计费为准，缓存
          Token
          按互斥桶计数。图片、视频及未计费调用不计入；每枚奖励终身领取一次。
        </p>
        <p v-if="tab === 'recharge'" class="fine-print">
          以累计有效充值本金为准，加赠和奖励不计入；退款可能使荣誉锁定，并追回对应已领奖励，每枚奖励终身领取一次。
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
        <div class="dialog-top">
          <strong>这一天的签到</strong
          ><button
            class="close"
            :disabled="busy"
            aria-label="关闭日期详情"
            @click="cardDialog?.close()"
          >
            <AchievementIcon name="close" />
          </button>
        </div>
        <div v-if="state && cardDay" class="calendar-day-body">
          <p class="eyebrow">{{ lunarDate(cardDay) }}</p>
          <h2>{{ cardDay }}</h2>
          <p class="day-record-status">
            <AchievementIcon
              :name="state.calendar.includes(cardDay) ? 'check' : 'book'"
            />{{ dayStatus }}
          </p>
          <div class="day-inline-tip">
            <AchievementIcon name="sun" />
            <p>{{ dayTip }}</p>
          </div>
          <p v-if="actionError" class="quiz-error" role="alert">
            {{ actionError }}
          </p>
          <div v-if="dayRecord" class="financial-details">
            <div>
              <span>当日金额奖励</span
              ><strong>${{ money(dayRecord.gross) }}</strong>
            </div>
            <div>
              <span>实际到账</span><strong>${{ money(dayRecord.net) }}</strong>
            </div>
            <p>{{ recordedCashReason(dayRecord.reason) }}</p>
          </div>
          <template v-if="cardEligible"
            ><div class="financial-details">
              <div>
                <span>消耗补签卡</span
                ><strong
                  >1 张 · 当前剩余 {{ state.card_balance ?? 0 }} 张</strong
                >
              </div>
              <p v-if="busy && !cardPreview" role="status">
                正在核验该日历史权益…
              </p>
              <template v-if="cardPreview"
                ><div>
                  <span>历史会员权益</span
                  ><strong
                    >{{
                      cardPreview.policy_known
                        ? cardPreview.tier
                          ? 'VIP ' + cardPreview.tier
                          : '普通会员'
                        : '历史权益待核验'
                    }}
                    <template v-if="cardPreview.policy_known">
                      · ${{ money(cardPreview.gross) }}</template
                    ></strong
                  >
                </div>
                <p v-if="!cardPreview.available">
                  {{
                    cardPreview.existing
                      ? '该日已经签到，无需补签。'
                      : !cardPreview.policy_known
                        ? '历史权益无法完整核验，不能补签。'
                        : !cardPreview.card_balance
                          ? '补签卡不足。'
                          : cardUnavailableReason(cardPreview.cash_reason)
                  }}
                </p></template
              >
              <p>
                确认前核验历史权益，服务端校验未通过不扣卡。网络中断请按原请求重试或刷新查询回执。
              </p>
            </div>
            <p v-if="cardError" class="notice" role="alert">{{ cardError }}</p>
            <button
              v-if="cardError && !cardCommand"
              class="text-button"
              :disabled="busy"
              @click="prepareCard"
            >
              重新核验
            </button>
            <div class="day-makeup-action">
              <div>
                <strong>补上这一天的签到</strong>
                <p>
                  使用 1 张补签卡 · 剩余 {{ state.card_balance ?? 0 }} 张 ·
                  按核验的历史权益即时补发
                </p>
              </div>
              <button
                class="gold-button primary"
                :disabled="busy || !cardPreview?.available || !cardCommand"
                @click="spendCard"
              >
                {{
                  busy
                    ? '正在处理…'
                    : cardError && cardCommand
                      ? '重试补签'
                      : '补签'
                }}
              </button>
            </div>
          </template>
          <div
            v-else-if="cardDay === state.date && !state.today"
            class="day-makeup-action"
          >
            <span>今日签到权益 ${{ money(state.daily_amount) }}</span
            ><button class="primary" :disabled="busy" @click="checkinFromDate">
              {{ busy ? '正在处理…' : '立即签到' }}
            </button>
          </div>
        </div>
      </dialog>
      <dialog ref="detailDialog" class="medal-dialog detail-dialog">
        <div class="dialog-top">
          <strong>徽章详情</strong
          ><button
            class="close"
            aria-label="关闭详情"
            :disabled="busy"
            @click="detailDialog?.close()"
          >
            <AchievementIcon name="close" />
          </button>
        </div>
        <div v-if="selected" class="detail-body">
          <MedalArt
            :medal="selected.key"
            :name="selected.name"
            class="detail-art"
          />
          <h2>{{ selected.name }}</h2>
          <p>{{ selected.description }}</p>
          <div class="detail-info">
            <div>
              <span>达成条件</span><strong>{{ condition(selected) }}</strong>
            </div>
            <div>
              <span>当前进度</span
              ><strong
                >{{
                  formatProgress(Math.min(selected.progress, selected.target))
                }}
                / {{ formatProgress(selected.target) }}</strong
              >
            </div>
            <div>
              <span>勋章奖励</span
              ><strong>{{
                selected.card_reward
                  ? selected.card_reward + ' 张补签卡'
                  : '$' + money(selected.reward)
              }}</strong>
            </div>
            <div>
              <span>领取状态</span
              ><strong>{{
                selected.preview
                  ? '后续开放'
                  : selected.claim?.revoked_at
                    ? '已追回'
                    : claimed(selected)
                      ? '已领取'
                      : selected.unlocked
                        ? canClaim(selected)
                          ? '可领取'
                          : '暂不可领取'
                        : '尚未达成'
              }}</strong>
            </div>
          </div>
          <p v-if="selected.category === 'activity'" class="quiet-note">
            每个主题 10 题，答对至少 8
            题通关。单枚发补签卡，集齐系列后另领金额。
          </p>
          <p
            v-if="
              selected.unlocked && !claimed(selected) && !canClaim(selected)
            "
            class="quiet-note"
          >
            {{ milestoneUnavailableReason() }}
          </p>
          <p v-if="actionError" class="quiz-error" role="alert">
            {{ actionError }}
          </p>
          <div
            v-if="selected.claim && !selected.card_reward"
            class="financial-details"
          >
            <div>
              <span>已领奖励原额</span
              ><strong>${{ money(selected.claim.gross) }}</strong>
            </div>
            <div>
              <span>实际到账</span
              ><strong>${{ money(selected.claim.net) }}</strong>
            </div>
            <div v-if="selected.claim.offset_amount">
              <span>抵扣待追回金额</span
              ><strong>${{ money(selected.claim.offset_amount) }}</strong>
            </div>
            <p>历史回执保留，当前奖励配置不会重复发放已领取的奖励。</p>
          </div>
          <div class="detail-actions">
            <button
              class="primary"
              :disabled="
                busy ||
                selected.preview ||
                (selected.unlocked && !claimed(selected) && !canClaim(selected))
              "
              @click="
                selected.unlocked && !claimed(selected)
                  ? act('claim', selected.key)
                  : showMedalTask(selected)
              "
            >
              {{
                selected.preview
                  ? '敬请期待'
                  : selected.unlocked && !claimed(selected)
                    ? actionError
                      ? '重试领取'
                      : '领取奖励'
                    : '查看对应任务'
              }}</button
            ><button
              v-if="selected.unlocked"
              class="small-button"
              :disabled="busy"
              @click="
                act(
                  'equip',
                  state?.equipment === selected.key ? '' : selected.key,
                )
              "
            >
              {{
                state?.equipment === selected.key ? '卸下徽章' : '佩戴此徽章'
              }}
            </button>
          </div>
        </div>
      </dialog>
    </main></AppLayout
  >
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { formatMoneyFixed as money } from '@/utils/format'
import AchievementIcon from '@/components/achievements/AchievementIcon.vue'
import '@/components/achievements/achievement-ui.css'
import AppLayout from '@/components/layout/AppLayout.vue'
import MedalArt from '@/components/achievements/MedalArt.vue'
import ActivityExplorer from '@/components/achievements/ActivityExplorer.vue'
import AchievementCompanion from '@/components/achievements/AchievementCompanion.vue'
import {
  dayIndex,
  tokenThemes,
  dayWhispers,
  lunarDate,
} from '@/components/achievements/companion'
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
  actionError = ref(''),
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
const shiftMonth = (month: string, delta: number) => {
  const date = new Date(month + '-01T00:00:00Z')
  date.setUTCMonth(date.getUTCMonth() + delta)
  return date.toISOString().slice(0, 7)
}
const minimumMonth = computed(() => {
  const month = state.value?.date.slice(0, 7) || viewMonth.value
  return [
    state.value?.card_min_date?.slice(0, 7) || month,
    shiftMonth(month, -1),
  ].sort()[0]
})
const maximumMonth = computed(() =>
  shiftMonth(state.value?.date.slice(0, 7) || viewMonth.value, 1),
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
  sign: '坚持的奖励',
  token: 'Token 成长',
  recharge: '充值荣誉',
  activity: '本系列勋章',
}
const selectedCalendarDate = ref('')
const companionDate = computed(
  () => selectedCalendarDate.value || state.value?.date || '',
)
function selectCalendarDate(date: string) {
  if (busy.value) return
  selectedCalendarDate.value = date
  actionError.value = ''
  cardDay.value = date
  clearCardPreview()
  if (canBackfillDate(date)) void openCardDialog(date)
  else cardDialog.value?.showModal()
}
const activityGroup = ref<'knowledge' | 'practice' | 'chapter'>('knowledge')
const focusActivityMedal = ref('')
function chooseActivityGroup(group: 'knowledge' | 'practice' | 'chapter') {
  activityGroup.value = group
  focusActivityMedal.value = ''
}
const activityGroups = [
  { key: 'knowledge' as const, name: '知识挑战' },
  { key: 'practice' as const, name: '实践演练' },
  { key: 'chapter' as const, name: '篇章收集' },
]
const groupPrefix = (group: string) =>
  group === 'knowledge' ? 'A-K' : group === 'practice' ? 'A-X' : 'A-C'
const visible = computed(
  () =>
    state.value?.medals.filter(
      (m) =>
        m.category === tab.value &&
        (tab.value !== 'activity' ||
          m.key.startsWith(groupPrefix(activityGroup.value))),
    ) ?? [],
)
const visibleSeries = computed(
  () =>
    state.value?.series?.filter(
      (s) => s.key === groupPrefix(activityGroup.value),
    ) ?? [],
)
const groupProgress = (group: string) =>
  group === 'chapter'
    ? `${state.value?.medals.find((m) => m.key === 'A-C01')?.progress ?? 0} / 4`
    : `${state.value?.passes.filter((p) => p.kind === group).length ?? 0} / 6`
const nextSignMedal = computed(() =>
  state.value?.medals.find((m) => m.category === 'sign' && !m.unlocked),
)
const nextActivityMedal = computed(
  () =>
    visible.value.find(
      (m) => m.key === focusActivityMedal.value && !m.unlocked,
    ) ??
    visible.value.find((m) => !m.unlocked) ??
    visible.value.at(-1),
)
const tokenTheme = computed(
  () =>
    tokenThemes[
      dayIndex(companionDate.value || '2026-01-01') % tokenThemes.length
    ],
)
async function copyPrompt() {
  try {
    await navigator.clipboard.writeText(tokenTheme.value.prompt)
    message.value = '提问示例已复制，把括号里的内容换成你的问题即可。'
  } catch {
    message.value = '复制暂不可用，请稍后重试。'
  }
}
function goBackfill() {
  tab.value = 'sign'
  setTimeout(focusBackfillCalendar, 0)
}

const equipped = computed(() =>
  state.value?.medals.find((m) => m.key === state.value?.equipment),
)
const unlocked = computed(
  () => state.value?.medals.filter((m) => m.unlocked).length ?? 0,
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
  if (next >= minimumMonth.value && next <= maximumMonth.value)
    calendarMonth.value = next
}
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
        ? '累计有效充值 $' + m.target.toLocaleString('zh-CN')
        : m.key.startsWith('A-K')
          ? '通过 ' + m.target + ' 个不同知识主题'
          : m.key.startsWith('A-X')
            ? '通过 ' + m.target + ' 个不同实践关卡'
            : m.key === 'A-C01'
              ? '完成初航 4 项指定任务'
              : '收藏 ' + m.target + ' 个不同篇章'
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
function milestoneUnavailableReason() {
  const reason = state.value?.milestone_cash_reason ?? state.value?.cash_reason
  return (
    (
      {
        cash_disabled: '成就金额奖励尚未开启，徽章解锁与补签卡领取分别记录。',
        not_in_cash_pilot: '当前账户不在成就金额奖励开放范围。',
        recent_activity_required:
          '成就金额奖励需近 30 日有有效充值或余额计费使用；签到金额不受此条件限制。',
        budget_exhausted: '本期金额奖励额度不足，请稍后核对。',
        account_unavailable: '当前账户不可领取成就金额奖励。',
      } as Record<string, string>
    )[reason || ''] || '暂不可领取，请刷新核对奖励状态。'
  )
}
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
    if (selected.value)
      selected.value =
        state.value.medals.find((m) => m.key === selected.value?.key) ?? null
    if (
      calendarMonth.value &&
      (calendarMonth.value < minimumMonth.value ||
        calendarMonth.value > maximumMonth.value)
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
  actionError.value = ''
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
    if (action !== 'checkin') detailDialog.value?.close()
  } catch (e) {
    error.value = errorMessage(e)
    actionError.value = error.value
  } finally {
    busy.value = false
  }
}
function openDetail(m: Medal) {
  actionError.value = ''
  selected.value = m
  detailDialog.value?.showModal()
}
onMounted(load)

const activityName = computed(
  () => activityGroups.find((g) => g.key === activityGroup.value)?.name || '',
)
const collectionDescription = computed(() =>
  tab.value === 'sign'
    ? '连续签到达到里程碑，解锁对应勋章与金额奖励。'
    : tab.value === 'token'
      ? '持续使用付费文本模型，积累 Token，逐步解锁成长勋章。'
      : '按累计有效充值本金解锁，奖励与充值加赠分别计算。',
)
const collectionCounter = computed(() =>
  tab.value === 'token'
    ? '当前累计 ' +
      formatProgress(
        state.value?.medals.find((m) => m.category === 'token')?.progress ?? 0,
      ) +
      ' Token'
    : '当前累计 $' +
      money(
        state.value?.medals.find((m) => m.category === 'recharge')?.progress ??
          0,
      ),
)
const activityDescription = computed(() =>
  activityGroup.value === 'knowledge'
    ? '通过 1、3、6 个不同知识主题，分别解锁对应勋章。'
    : activityGroup.value === 'practice'
      ? '通过 2、4、6 个不同实践关卡，逐步解锁三枚勋章。'
      : '按篇章完成指定任务，收藏对应勋章。',
)
const claimed = (m: Medal) => !!(m.card_reward ? m.card_claim : m.claim)
const cardEligible = computed(
  () => !!state.value && canBackfillDate(cardDay.value),
)
const dayTip = computed(
  () =>
    dayWhispers[
      dayIndex(cardDay.value || state.value?.date || '2026-01-01') %
        dayWhispers.length
    ][0],
)
const dayRecord = computed(() =>
  state.value?.history.find((h) => h.day === cardDay.value),
)
const dayStatus = computed(() => {
  const s = state.value
  if (!s) return ''
  if (s.calendar.includes(cardDay.value)) return '这一天已签到，记录已保留。'
  if (cardDay.value === s.date) return '今天还未签到，收下今天的奖励吧。'
  if (cardDay.value > s.date) return '还未到来的日子，也值得期待。'
  if (cardEligible.value) return '这一天留了个空位，可使用补签卡补上。'
  if (s.joined_date && cardDay.value < s.joined_date)
    return '这一天还未与你相遇，可以看看这一天的小笺。'
  return '该日期已超过补签范围，可以看看这一天的小笺。'
})
const joinedDateLabel = computed(() => {
  if (!state.value?.joined_date) return '初次相遇'
  const [year, month, day] = state.value.joined_date.split('-').map(Number)
  return `${year} 年 ${month} 月 ${day} 日`
})
function phase(m: Medal) {
  return String(
    (state.value?.medals
      .filter(
        (x) =>
          x.category === m.category &&
          (m.category !== 'activity' ||
            x.key.slice(0, 3) === m.key.slice(0, 3)),
      )
      .findIndex((x) => x.key === m.key) ?? 0) + 1,
  ).padStart(2, '0')
}
function medalStatus(m: Medal) {
  return m.manual === 'revoked'
    ? '管理员已取消'
    : m.manual === 'granted'
      ? '管理员授予'
      : m.preview
        ? '后续开放'
        : m.claim?.revoked_at
          ? '已追回'
          : m.unlocked
            ? '已解锁'
            : '未解锁'
}
function medalActionLabel(m: Medal) {
  return m.preview
    ? '敬请期待'
    : !m.unlocked
      ? '查看任务'
      : claimed(m)
        ? '查看详情'
        : canClaim(m)
          ? m.card_reward
            ? '领取补签卡'
            : '领取金额'
          : '查看领取条件'
}
function showMedalTask(m: Medal) {
  if (busy.value) return
  detailDialog.value?.close()
  tab.value = m.category
  if (m.category === 'activity')
    activityGroup.value = m.key.startsWith('A-K')
      ? 'knowledge'
      : m.key.startsWith('A-X')
        ? 'practice'
        : 'chapter'
  focusActivityMedal.value = m.key
  selected.value = m
  setTimeout(
    () =>
      document
        .querySelector(
          m.category === 'activity'
            ? '#activity-tasks'
            : m.category === 'sign'
              ? '.sign-calendar'
              : '.collection-head',
        )
        ?.scrollIntoView({ behavior: 'smooth', block: 'start' }),
    0,
  )
}
function medalAction(m: Medal) {
  if (busy.value) return
  if (!m.unlocked) showMedalTask(m)
  else if (!claimed(m) && canClaim(m)) void act('claim', m.key)
  else openDetail(m)
}
function openPending() {
  const medal = state.value?.medals.find(canClaim)
  if (medal) openDetail(medal)
  else {
    const series = state.value?.series?.find(canClaimSeries)
    if (series) {
      tab.value = 'activity'
      activityGroup.value =
        series.key === 'A-K'
          ? 'knowledge'
          : series.key === 'A-X'
            ? 'practice'
            : 'chapter'
    }
  }
}
async function checkinFromDate() {
  await act('checkin')
  if (state.value?.today) cardDialog.value?.close()
}
let toastTimer: ReturnType<typeof setTimeout> | undefined
watch(message, (value) => {
  clearTimeout(toastTimer)
  if (value)
    toastTimer = setTimeout(() => {
      message.value = ''
    }, 6500)
})
onUnmounted(() => clearTimeout(toastTimer))

function documentScrollSeries() {
  document
    .getElementById('activity-series')
    ?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
</script>
<style scoped>
.achievement-page {
  --paper: #fffefa;
  --surface: #fffefa;
  --ink: #282925;
  --muted: #78766d;
  --line: #e6e0d3;
  --gold: #93763c;
  --gold-light: #c9b477;
  --wash: #f5f2e9;
  --companion-surface: #f7f4ec;
  --companion-emphasis: #eee4ce;
  --companion-soft: #f1eadb;
  --companion-border: #e3d9c4;
  --companion-ink: #806536;
}
:global(.dark .achievement-page) {
  --paper: #181a1a;
  --surface: #181a1a;
  --ink: #f0ece2;
  --muted: #a09d91;
  --line: #37382e;
  --gold: #d0ba80;
  --gold-light: #9b8550;
  --wash: #20221e;
  --companion-surface: #23231f;
  --companion-emphasis: #3b3323;
  --companion-soft: #2d2b23;
  --companion-border: #45402f;
  --companion-ink: #d8c28e;
}
</style>
