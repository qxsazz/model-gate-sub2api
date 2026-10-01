<template>
 <AppLayout>
  <div class="vip-page">
   <div class="vip-heading"><div><p class="eyebrow">MODEL-GATE MEMBERSHIP</p><h1>VIP 中心</h1></div><button class="btn btn-secondary" :disabled="loading" @click="load"><Icon name="refresh" size="sm" />刷新</button></div>
   <div v-if="loading && !state" class="card p-8 text-center">正在读取会员权益…</div>
   <div v-else-if="error" class="card p-6 text-red-600" role="alert">{{ error }}<button class="btn btn-secondary ml-4" @click="load">重试</button></div>
   <template v-else-if="state">
    <p v-if="!state.enabled" class="status-note">会员规则尚未启用，当前按原有价格与权限使用。</p>
    <section class="membership">
     <div class="member-top"><span class="brand">MODEL-GATE</span><VIPBadge :key="state.badge_level" /></div>
     <div class="member-body"><div class="member-emblem"><Icon name="badge" size="xl" /><span>{{ state.badge_level || 'MG' }}</span></div><div><p class="eyebrow">MEMBER STATUS</p><h2>{{ state.tier.level ? 'VIP ' + state.tier.level : '普通会员' }}</h2><p>累计有效充值决定会员成长</p></div></div>
     <div class="member-stats"><div><span>累计有效充值</span><strong>${{ money(state.total) }}</strong></div><div><span>同时请求上限</span><strong>{{ state.concurrency }}</strong></div><div><span>邀请返利</span><strong>{{ state.rebate_percent }}%</strong></div></div>
     <div class="member-progress"><div><span>{{ state.next ? '距 VIP ' + state.next.level + ' 还需充值 $' + money(Math.max(0, state.next.threshold - state.total)) : '已达到最高充值等级' }}</span><router-link to="/purchase">前往充值 <Icon name="arrowRight" size="sm" /></router-link></div><progress :value="progress" max="100" aria-label="会员成长进度"></progress></div>
    </section>
    <section class="tiers"><h2>等级权益</h2><div class="table-overflow"><table><thead><tr><th>等级</th><th>累计充值</th><th>同时请求</th><th>邀请返利</th><th>专属分组</th></tr></thead><tbody><tr v-for="tier in state.rules.tiers" :key="tier.level" :class="{ current: tier.level === state.tier.level }"><td>VIP {{ tier.level }} <span v-if="tier.level === state.tier.level" class="current-label">当前</span></td><td>${{ money(tier.threshold) }}</td><td>{{ tier.concurrency }}</td><td>{{ tier.rebate_percent }}%</td><td>{{ tier.threshold >= state.rules.access_threshold ? '达标开放' : '独立门槛' }}</td></tr></tbody></table></div></section>
    <section><div class="section-title"><h2>普通分组优惠</h2><span>有效价格以各组规则为准</span></div><div class="table-overflow"><table><thead><tr><th>分组</th><th>基础倍率</th><th>当前倍率</th><th>实际减免</th></tr></thead><tbody><tr v-for="g in ordinary" :key="g.id"><td>{{ g.name }}</td><td>×{{ rate(g.base_rate) }}</td><td class="gold">×{{ rate(g.rate) }}</td><td>{{ g.participating ? '−' + rate(Math.max(0,g.base_rate-g.rate)) : '原有定价' }}</td></tr></tbody></table></div></section>
    <section><div class="section-title"><h2>VIP 专属服务</h2><span>专属服务独立定价</span></div><div v-if="exclusive.length" class="exclusive-list"><article v-for="g in exclusive" :key="g.id"><div><h3>{{ g.name }}</h3><span>{{ g.platform }} · 基础倍率 ×{{ rate(g.base_rate) }}</span></div><router-link v-if="g.granted" to="/keys" class="btn btn-secondary">管理 API Key <Icon name="arrowRight" size="sm" /></router-link><span v-else class="locked">累计充值满 ${{ money(state.rules.access_threshold) }} 开放</span></article></div><p v-else class="empty">暂无可展示的专属服务</p></section>
    <section class="limits"><h2>请求额度</h2><div><span>每分钟请求上限</span><strong>{{ state.rpm === 0 ? '沿用原有限流规则' : state.rpm + ' RPM' }}</strong></div><p>同一用户的 API Key 共用用户并发上限，具体渠道还受自身容量与限额约束。</p></section>
    <section><div class="section-title"><h2>充值成长记录</h2><router-link to="/affiliate">邀请返利明细 <Icon name="arrowRight" size="sm" /></router-link></div><div v-if="state.ledger.length" class="table-overflow"><table><thead><tr><th>时间</th><th>来源</th><th>计入金额</th></tr></thead><tbody><tr v-for="entry in state.ledger" :key="entry.id"><td>{{ new Date(entry.created_at).toLocaleString() }}</td><td>{{ sourceLabel(entry.source) }}</td><td :class="entry.amount >= 0 ? 'gold' : 'text-red-600'">{{ entry.amount >= 0 ? '+' : '−' }}${{ money(Math.abs(entry.amount)) }}</td></tr></tbody></table></div><p v-else class="empty">暂无已确认的充值成长记录</p></section>
   </template>
  </div>
 </AppLayout>
</template>
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import VIPBadge from '@/components/user/VIPBadge.vue'
import { getVIP, type VIPSnapshot } from '@/api/vip'
const state = ref<VIPSnapshot | null>(null)
const loading = ref(false)
const error = ref('')
const ordinary = computed(() => state.value?.groups.filter(g => !g.exclusive) || [])
const exclusive = computed(() => state.value?.groups.filter(g => g.exclusive) || [])
const progress = computed(() => { const s = state.value; if (!s) return 0; if (!s.next) return 100; const previous = s.tier.threshold || 0; return Math.max(0, Math.min(100, (s.total - previous) / (s.next.threshold - previous) * 100)) })
const money = (v: number) => v.toFixed(2)
const rate = (v: number) => v.toFixed(3).replace(/0+$/, '').replace(/\.$/, '')
const sourceLabel = (source: string) => ({ payment: '在线充值', payment_refund: '在线退款', admin_balance: '后台调整', opening: '初始确认记录' }[source] || source)
async function load() { loading.value = true; error.value = ''; try { state.value = await getVIP() } catch { error.value = '会员权益读取失败，请重试。' } finally { loading.value = false } }
onMounted(load)
</script>
<style scoped>
.vip-page{max-width:1200px;margin:auto;color:var(--mg-ink-900,#24252a)}
.vip-heading,.section-title{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:20px}
h1,h2,h3{font-family:'Noto Serif SC',SimSun,serif;letter-spacing:0}h1{font-size:24px;font-weight:600}h2{font-size:20px;font-weight:600}h3{font-size:15px;font-weight:600}
.eyebrow{font:500 11px 'DM Mono',Consolas,monospace;color:#9c8344;letter-spacing:0}
.membership{background:#101114;color:#f5f1e7;border:1px solid #665732;border-radius:8px;padding:28px;margin-bottom:32px}
.member-top{display:flex;justify-content:space-between;align-items:center}.brand{font:600 20px 'Bodoni Moda',Georgia,serif;color:#dfcf9f}
.member-body{display:flex;align-items:center;gap:20px;margin:28px 0}.member-body h2{font-size:30px;margin:6px 0}.member-body p:last-child{font-size:13px;color:#d6cebd}
.member-emblem{width:64px;height:64px;flex-shrink:0;border:1px solid #c9b477;transform:rotate(0);border-radius:8px;display:flex;flex-direction:column;align-items:center;justify-content:center;color:#dfcf9f}.member-emblem span{font:600 18px Georgia,serif}
.member-stats{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:18px}.member-stats span{display:block;font-size:12px;color:#d6cebd}.member-stats strong{display:block;font:500 25px 'DM Mono',Consolas,monospace;margin-top:8px}
.member-progress{margin-top:28px;border-top:1px solid #38352f;padding-top:18px}.member-progress>div{display:flex;justify-content:space-between;gap:12px;font-size:12px}.member-progress a,.section-title a{display:inline-flex;align-items:center;gap:6px}.member-progress a{color:#dfcf9f}
progress{width:100%;height:4px;display:block;margin-top:14px;appearance:none;border:none;background:#38352f}progress::-webkit-progress-bar{background:#38352f}progress::-webkit-progress-value{background:#c9b477}
section:not(.membership){margin-bottom:32px}.table-overflow{overflow-x:auto}table{width:100%;border-collapse:collapse;font-size:13px}th{text-align:left;font-weight:500;background:var(--mg-champagne-50,#f5f1e7);color:var(--mg-muted,#746f65)}td,th{padding:13px 12px;border-bottom:1px solid var(--mg-line-warm,#e7e1d4)}td:not(:first-child){font-family:'DM Mono',Consolas,monospace}
.gold{color:var(--mg-gold-700,#765f2c)}.current{background:var(--mg-gold-50,#fcfaf4)}.current-label{font:500 10px sans-serif;color:#765f2c;margin-left:6px}.tiers h2{margin-bottom:16px}
.section-title>span{font-size:12px;color:var(--mg-muted,#746f65)}.exclusive-list article{display:flex;justify-content:space-between;align-items:center;gap:16px;padding:18px 0;border-bottom:1px solid var(--mg-line-warm,#e7e1d4)}.exclusive-list article span{font-size:12px;color:var(--mg-muted,#746f65)}.empty{padding:24px 0;color:var(--mg-muted,#746f65);font-size:13px}
.limits>div{display:flex;justify-content:space-between;margin:16px 0;font-size:14px}.limits>p{font-size:12px;color:var(--mg-muted,#746f65)}
.status-note{padding:12px 16px;margin-bottom:18px;border:1px solid var(--mg-line-warm,#e7e1d4);font-size:13px}
@media(max-width:640px){.membership{padding:20px}.member-stats{gap:8px}.member-stats strong{font-size:18px}.vip-heading{align-items:flex-start}.section-title{flex-wrap:wrap}.exclusive-list article{align-items:flex-start;flex-wrap:wrap}.member-progress>div{flex-wrap:wrap}.member-body h2{font-size:26px}}
</style>
