<template>
  <AppLayout>
    <div class="vip-page">
      <header class="vip-heading">
        <div><p class="eyebrow">MODEL-GATE MEMBERSHIP</p><h1>VIP 中心</h1></div>
        <button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />刷新</button>
      </header>
      <div v-if="loading && !state" class="empty">正在读取会员权益…</div>
      <div v-else-if="error" class="status-note" role="alert">{{ error }}<button class="btn btn-secondary" @click="load">重试</button></div>
      <template v-else-if="state">
        <p v-if="!state.enabled" class="status-note">会员规则尚未启用，当前按原有价格与权限使用。</p>
        <section class="membership">
          <div class="member-card" :class="'level-' + state.badge_level">
            <div class="member-top"><span class="eyebrow">MODEL-GATE PRIVATE MEMBER</span><span class="eyebrow">MEMBER SERIES</span></div>
            <div class="member-body"><div class="diamond"><Icon name="badge" size="xl" /></div><div><p class="eyebrow">CURRENT PRIVILEGE</p><h2>{{ levelName(state.badge_level) }}</h2><span class="vip-tag">VIP {{ state.badge_level }}</span></div></div>
            <div class="card-benefits"><div><span>邀请返利</span><strong>{{ state.rebate_percent }}%</strong></div><div><span>同时请求上限</span><strong>{{ state.concurrency }}</strong></div></div>
            <footer><span>MODEL-GATE</span><span>MEMBERSHIP / {{ String(state.badge_level).padStart(2, '0') }}</span></footer>
          </div>
          <div class="member-overview">
            <div class="recharge-heading"><div><p class="muted">累计有效充值</p><strong class="total">${{ money(state.total) }}</strong></div><router-link to="/purchase" class="btn btn-primary"><Icon name="creditCard" size="sm" />去充值</router-link></div>
            <p v-if="manualPrivilege" class="status-note">包含管理员授予的权益，成长进度仍按累计有效充值计算。</p>
            <div class="progress-label"><span>{{ state.next ? '距 VIP ' + state.next.level + ' 还需 $' + money(Math.max(0, state.next.threshold - state.total)) : '已达到最高充值等级' }}</span><span>{{ Math.round(progress) }}%</span></div>
            <progress :value="progress" max="100" aria-label="会员成长进度" />
            <div class="overview-stats">
              <div><span>邀请返利</span><strong>{{ state.rebate_percent }}%</strong></div>
              <div><span>每分钟请求上限</span><strong>{{ state.rpm ? state.rpm + ' RPM' : '沿用原有限流规则' }}</strong></div>
              <div><span>专属分组</span><strong>{{ exclusiveAccess ? '已开放' : '待解锁' }}</strong></div>
              <div><span>累计已领奖励</span><strong>{{ membership ? '$' + money(membership.claimed) : '—' }}</strong></div>
              <div><span>可领取奖励</span><strong>{{ membership ? availableRewards : '—' }}</strong></div>
              <div><span>成长门槛</span><strong>${{ money(state.rules.access_threshold) }} 起</strong></div>
            </div>
            <router-link v-if="exclusiveAccess" to="/keys" class="text-link">管理专属分组 API Key <Icon name="arrowRight" size="sm" /></router-link>
          </div>
        </section>
        <section class="honors">
          <div class="section-title"><div><p class="eyebrow">PRIVATE CIRCLE</p><h2>VIP 荣誉席位</h2><p class="muted">按当前有效等级排列 · 会员名称已脱敏</p></div><span class="eyebrow">TOP 10</span></div>
          <p v-if="membershipError" class="status-note" role="alert">{{ membershipError }}<button class="text-link" @click="loadMembership">重试</button></p>
          <template v-else-if="membership">
            <div v-if="membership.seats.length" class="podium"><article v-for="(seat, index) in membership.seats.slice(0, 3)" :key="index"><p class="rank">{{ String(index + 1).padStart(2, '0') }} <span>{{ index === 0 ? '首席' : '荣誉席' }}</span></p><h3>{{ seat.name }}</h3><span class="vip-tag" :class="'level-' + seat.level">VIP {{ seat.level }} · {{ levelName(seat.level) }}</span></article></div>
            <ol v-if="membership.seats.length > 3" class="seat-list" start="4"><li v-for="(seat, index) in membership.seats.slice(3)" :key="index"><span class="rank">{{ String(index + 4).padStart(2, '0') }}</span><strong>{{ seat.name }}</strong><span class="vip-tag" :class="'level-' + seat.level">VIP {{ seat.level }} · {{ levelName(seat.level) }}</span></li></ol>
            <p v-if="!membership.seats.length" class="empty">荣誉席位静待首位会员</p>
          </template>
        </section>
        <nav class="vip-tabs" role="tablist" aria-label="会员权益">
          <button v-for="item in tabs" :id="'tab-' + item.id" :key="item.id" role="tab" :data-tab="item.id" :aria-selected="tab === item.id" :aria-controls="'panel-' + item.id" :class="{ active: tab === item.id }" @click="tab = item.id">{{ item.label }}</button>
        </nav>
        <section v-if="tab === 'benefits'" id="panel-benefits" role="tabpanel" aria-labelledby="tab-benefits">
          <div class="section-title"><div><p class="eyebrow">MEMBERSHIP</p><h2>等级权益</h2><p class="muted">累计有效充值，逐级开启更多权益</p></div></div>
          <div class="tier-grid">
            <article class="tier-card level-0" :class="{ current: state.tier.level === 0 }"><div class="tier-top"><div class="diamond"><Icon name="badge" size="lg" /></div><span v-if="state.tier.level === 0" class="current-label">当前等级</span></div><span class="vip-tag">VIP 0</span><h3>普通会员</h3><strong class="threshold">注册即享</strong><ul><li>累计充值成长记录</li><li>原有分组与基础权益</li><li>邀请返利 0%</li></ul></article>
            <article v-for="tier in state.rules.tiers" :key="tier.level" class="tier-card" :class="['level-' + tier.level, { current: state.tier.level === tier.level }]">
              <div class="tier-top"><div class="diamond"><Icon name="badge" size="lg" /></div><span v-if="state.tier.level === tier.level" class="current-label">当前等级</span></div><span class="vip-tag">VIP {{ tier.level }}</span><h3>{{ levelName(tier.level) }}</h3><strong class="threshold">${{ money(tier.threshold) }} 起</strong>
              <ul><li>{{ discountText(tier.level) }}</li><li>{{ tier.threshold >= state.rules.access_threshold ? '开放 VIP 专属分组' : '专属分组按独立门槛开放' }}</li><li>同时请求 {{ tier.concurrency }} · 邀请返利 {{ tier.rebate_percent }}%</li><li>累充里程碑奖励 ${{ money(tier.threshold * 0.01) }}</li></ul>
            </article>
          </div>
          <p class="fine-print">倍率为绝对值减免，部分分组维持原价。专属分组独立定价；人工授权与定价优先。请求仍受渠道自身容量与限额约束。</p>
        </section>
        <section v-else-if="tab === 'rewards'" id="panel-rewards" role="tabpanel" aria-labelledby="tab-rewards">
          <div class="section-title"><div><p class="eyebrow">MILESTONES</p><h2>累充奖励</h2><p class="muted">每个里程碑仅可领取一次 · 奖励为门槛金额的 1%</p></div></div>
          <p v-if="membership?.debt" class="status-note">待追回奖励 ${{ money(membership.debt) }}，后续余额入账将优先抵扣。</p>
          <p v-if="claimMessage" class="status-note" role="status">{{ claimMessage }}</p>
          <p v-if="membershipError" class="status-note" role="alert">{{ membershipError }}</p>
          <div v-for="reward in membership?.rewards || []" :key="reward.level" class="reward-row">
            <div class="reward-description"><div class="diamond"><Icon :name="reward.status === 'available' ? 'gift' : reward.status === 'claimed' ? 'check' : 'lock'" size="lg" /></div><div><h3>VIP {{ reward.level }} {{ levelName(reward.level) }}</h3><p class="muted">累计有效充值达到 ${{ money(reward.threshold) }}</p></div></div>
            <div class="reward-amount"><strong>${{ money(reward.amount) }}</strong><span class="muted">余额奖励</span></div>
            <button v-if="reward.status === 'available'" class="btn btn-primary" :data-claim="reward.level" :disabled="claiming !== null" @click="claim(reward.level)"><Icon name="gift" size="sm" />{{ claiming === reward.level ? '领取中…' : '领取奖励' }}</button><span v-else class="reward-status">{{ statusLabel(reward.status) }}</span>
          </div>
          <p class="fine-print">奖励不计入累计充值，不产生邀请返利。退款或后台退费跌破门槛时追回对应奖励，领取记录保留，再次达标不重复发放。</p>
        </section>
        <section v-else id="panel-records" role="tabpanel" aria-labelledby="tab-records">
          <div class="section-title"><h2>充值成长记录</h2><router-link to="/affiliate" class="text-link">邀请返利明细 <Icon name="arrowRight" size="sm" /></router-link></div>
          <div v-if="state.ledger.length" class="table-overflow"><table><thead><tr><th>时间</th><th>来源</th><th>计入金额</th></tr></thead><tbody><tr v-for="entry in state.ledger" :key="entry.id"><td>{{ new Date(entry.created_at).toLocaleString() }}</td><td>{{ sourceLabel(entry.source) }}</td><td>{{ entry.amount >= 0 ? '+' : '−' }}${{ money(Math.abs(entry.amount)) }}</td></tr></tbody></table></div><p v-else class="empty">暂无已确认的充值成长记录</p>
        </section>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { getVIP, getVIPMembership, claimVIPReward, type VIPSnapshot, type VIPMembership, type VIPReward } from '@/api/vip'
const state = ref<VIPSnapshot | null>(null)
const authStore = useAuthStore()
const membership = ref<VIPMembership | null>(null)
const loading = ref(false)
const error = ref('')
const membershipError = ref('')
const claiming = ref<number | null>(null)
const claimMessage = ref('')
const tab = ref('benefits')
const tabs = [{ id: 'benefits', label: '等级权益' }, { id: 'rewards', label: '累充奖励' }, { id: 'records', label: '成长记录' }]
const levelName = (level: number) => ['普通会员', '青铜', '白银', '黄金', '铂金', '黑钻'][level] || '会员'
const money = (value: number) => value.toFixed(2)
const rate = (value: number) => value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
const exclusiveAccess = computed(() => state.value?.groups.some(g => g.exclusive && g.granted) || false)
const manualPrivilege = computed(() => state.value?.overrides?.length || 0)
const availableRewards = computed(() => membership.value?.rewards.filter(r => r.status === 'available').length || 0)
const progress = computed(() => { const s = state.value; if (!s) return 0; if (!s.next) return 100; const previous = s.tier.threshold || 0; return Math.max(0, Math.min(100, (s.total - previous) / (s.next.threshold - previous) * 100)) })
function discountText(level: number) {
  const cuts = membership.value?.discount_summaries[level] || []
  if (!cuts.length) return membership.value ? '普通分组按现行规则计价' : '普通分组成长优惠'
  return `普通分组减免 ${rate(cuts[0])}${cuts.length > 1 ? ' · 部分减免 ' + rate(cuts[cuts.length - 1]) : ''}`
}
const statusLabel = (status: VIPReward['status']) => ({ locked: '未达标', available: '可领取', claimed: '已领取', revoked: '已追回' }[status])
const sourceLabel = (source: string) => ({ payment: '在线充值', payment_refund: '在线退款', admin_balance: '后台调整', opening: '初始确认记录' }[source] || source)
async function loadMembership() {
  membershipError.value = ''
  try { membership.value = await getVIPMembership() } catch { membership.value = null; membershipError.value = '荣誉席位与奖励读取失败，请重试。' }
}
async function load() {
  loading.value = true; error.value = ''
  try { state.value = await getVIP(); await loadMembership() } catch { error.value = '会员权益读取失败，请重试。' } finally { loading.value = false }
}
async function claim(level: number) {
  if (claiming.value !== null) return
  claiming.value = level; claimMessage.value = ''
  try { const result = await claimVIPReward(level); claimMessage.value = result.amount > 0 ? `奖励 $${money(result.amount)} 已入账。` : '该档奖励已领取，请勿重复领取。'; await load(); await authStore.refreshUser().catch(() => undefined) } catch { claimMessage.value = '领取失败，请刷新确认资格后重试。' } finally { claiming.value = null }
}
onMounted(load)
</script>
<style scoped>
.vip-page{max-width:1440px;margin:auto;color:var(--mg-ink-900,#24252a)}
h1,h2,h3{font-family:'Noto Serif SC',SimSun,serif;letter-spacing:0;font-weight:600}h1{font-size:26px}h2{font-size:22px}h3{font-size:18px}
.vip-heading,.section-title{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:24px}.eyebrow{font:500 11px 'DM Mono',Consolas,monospace;color:var(--mg-gold-700,#927640);letter-spacing:0}.muted,.fine-print{font-size:13px;color:var(--mg-muted,#746f65);line-height:1.8}.section-title .muted{margin-top:8px}
.membership{display:grid;grid-template-columns:minmax(0,5fr) minmax(0,6fr);border-block:1px solid var(--mg-line-warm,#e7e1d4);margin-bottom:40px}
.member-card{background:#101114;color:#e0cf9e;padding:28px;min-height:360px;display:flex;flex-direction:column}.member-card .eyebrow{color:#b6a475}.member-top{display:flex;justify-content:space-between;gap:12px}.member-body{display:flex;align-items:center;gap:28px;flex:1;padding:38px 0}.member-body h2{font-size:36px;margin:8px 0 12px}.diamond{width:54px;height:54px;flex-shrink:0;border:1px solid currentColor;transform:rotate(45deg);display:flex;align-items:center;justify-content:center;margin:12px}.diamond :deep(svg){transform:rotate(-45deg)}.member-body .diamond{width:80px;height:80px}
.card-benefits{display:flex;gap:28px;padding:0 0 24px 12px}.card-benefits>div{padding-right:28px;border-right:1px solid #555044}.card-benefits span{font-size:12px;display:block}.card-benefits strong{display:block;font:500 24px 'DM Mono',monospace;margin-top:8px}.member-card footer{border-top:1px solid #555044;padding-top:18px;display:flex;justify-content:space-between;gap:12px;font:500 10px 'DM Mono',monospace}
.member-overview{padding:28px 32px;background:var(--mg-surface,#fff)}.recharge-heading{display:flex;align-items:center;justify-content:space-between;gap:16px}.total{font:500 42px 'DM Mono',Consolas,monospace;display:block;margin-top:8px}.progress-label{display:flex;justify-content:space-between;gap:16px;margin-top:28px;font-size:13px}.overview-stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));margin:24px 0 16px}.overview-stats>div{padding:16px 12px;border-top:1px solid var(--mg-line-warm,#e7e1d4)}.overview-stats span{display:block;font-size:12px;color:var(--mg-muted,#746f65)}.overview-stats strong{display:block;margin-top:8px;font:500 15px 'DM Mono',monospace;overflow-wrap:anywhere}
progress{width:100%;height:5px;display:block;margin-top:14px;appearance:none;border:none;background:#e7e1d4}progress::-webkit-progress-bar{background:#e7e1d4}progress::-webkit-progress-value{background:#aa8c47}progress::-moz-progress-bar{background:#aa8c47}.text-link{display:inline-flex;gap:8px;align-items:center;color:var(--mg-gold-700,#765f2c);font-size:13px}
.honors{margin-bottom:40px}.podium{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));border-block:1px solid var(--mg-line-warm,#e7e1d4);background:var(--mg-gold-50,#fcfaf4)}.podium article{padding:24px;border-right:1px solid var(--mg-line-warm,#e7e1d4);min-height:180px}.podium article:last-child{border:0}.rank{font:500 26px 'DM Mono',monospace;color:var(--mg-gold-700,#927640)}.rank span{font-size:11px}.podium h3{margin:24px 0 12px;overflow-wrap:anywhere}.seat-list{list-style:none;padding:0}.seat-list li{display:flex;align-items:center;gap:24px;padding:18px 8px;border-bottom:1px solid var(--mg-line-warm,#e7e1d4)}.seat-list .rank{font-size:18px}.seat-list strong{font-size:14px;overflow-wrap:anywhere}
.vip-tabs{display:flex;gap:28px;border-bottom:1px solid var(--mg-line-warm,#e7e1d4);margin-bottom:28px}.vip-tabs button{font-size:15px;padding:14px 0;border-bottom:2px solid transparent;color:var(--mg-muted,#746f65);white-space:nowrap}.vip-tabs button.active{color:var(--mg-ink-900,#24252a);border-color:#aa8c47}.tier-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.tier-card{border:1px solid var(--tier-color,#afbbc0);border-radius:6px;padding:24px;min-height:330px;color:var(--tier-color,#66747b);background:var(--mg-surface,#fff)}.tier-top{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:18px}.tier-card h3{font-size:25px;margin:16px 0}.threshold{font:500 20px 'DM Mono',monospace}.tier-card ul{list-style:none;padding:0;margin-top:22px;font-size:13px;line-height:1.9;color:var(--mg-muted,#746f65)}.tier-card li{margin-top:7px}.tier-card li::before{content:'−';margin-right:8px}.tier-card.current{outline:1px solid #aa8c47;outline-offset:2px}.current-label{font-size:11px;font-weight:600}.vip-tag{display:inline-block;font:500 11px 'DM Mono',monospace;border:1px solid currentColor;border-radius:3px;padding:5px 8px;color:var(--tier-color,inherit)}
.level-0{--tier-color:#7d8586}.level-1{--tier-color:#ab7955}.level-2{--tier-color:#7a8d98}.level-3{--tier-color:#aa8b3e}.level-4{--tier-color:#538286}.level-5{--tier-color:#ceb573}.tier-card.level-5{background:#101114}.tier-card.level-5 ul{color:#d0c6b0}.fine-print{font-size:12px;margin-top:20px}.reward-row{display:grid;grid-template-columns:minmax(0,5fr) minmax(0,2fr) auto;align-items:center;gap:24px;border:1px solid var(--mg-line-warm,#e7e1d4);border-radius:6px;padding:20px 24px;margin-bottom:12px;background:var(--mg-surface,#fff)}.reward-description{display:flex;align-items:center;gap:24px}.reward-description .diamond{width:40px;height:40px;color:#a38b58}.reward-description p{margin-top:6px}.reward-amount strong{display:block;font:500 25px 'DM Mono',monospace}.reward-amount span{display:block;margin-top:4px}.reward-status{font-size:12px;color:var(--mg-muted,#746f65);border:1px solid var(--mg-line-warm,#e7e1d4);padding:8px 12px;border-radius:4px}.status-note{padding:12px 16px;margin:16px 0;border-left:2px solid #aa8c47;background:var(--mg-gold-50,#fcfaf4);font-size:13px;line-height:1.8}.status-note button{margin-left:12px}.empty{padding:32px 0;color:var(--mg-muted,#746f65);font-size:13px}.table-overflow{overflow:auto}table{width:100%;border-collapse:collapse;font-size:13px}td,th{padding:14px 12px;border-bottom:1px solid var(--mg-line-warm,#e7e1d4);text-align:left}td:last-child{font-family:'DM Mono',monospace}
:global(.dark) .member-overview,:global(.dark) .tier-card:not(.level-5),:global(.dark) .reward-row{background:#191a1d}:global(.dark) .podium,:global(.dark) .status-note{background:#20201e}
@media(min-width:1600px){.tier-grid{grid-template-columns:repeat(4,minmax(0,1fr))}}@media(max-width:1100px){.membership{grid-template-columns:1fr}.member-card{min-height:300px}.tier-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media(max-width:640px){.vip-heading{align-items:flex-start}h1{font-size:23px}.member-card,.member-overview{padding:22px}.member-body{gap:20px}.member-body h2{font-size:30px}.member-body .diamond{width:60px;height:60px}.total{font-size:30px}.overview-stats{grid-template-columns:repeat(2,minmax(0,1fr))}.podium{grid-template-columns:1fr}.podium article{min-height:130px;border-right:0;border-bottom:1px solid var(--mg-line-warm,#e7e1d4)}.podium h3{margin-top:14px}.seat-list li{gap:14px;flex-wrap:wrap}.tier-grid{grid-template-columns:1fr}.tier-card{min-height:310px}.reward-row{grid-template-columns:minmax(0,1fr) auto;gap:16px;padding:16px}.reward-description{grid-column:1/-1;gap:16px}.reward-description h3{font-size:16px}.reward-amount strong{font-size:22px}.vip-tabs{gap:24px}.member-top,.member-card footer{flex-wrap:wrap}.section-title{align-items:flex-start}}
</style>
