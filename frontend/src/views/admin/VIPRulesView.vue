<template>
 <AppLayout>
  <div class="max-w-7xl mx-auto space-y-6">
   <div class="flex items-center justify-between"><h1 class="text-2xl font-semibold">VIP 规则与权益</h1><button class="btn btn-primary" :disabled="busy || !rules" @click="save"><Icon name="checkCircle" size="sm" />保存规则</button></div>
   <p v-if="error" class="text-red-600" role="alert">{{ error }}</p><p v-if="success" class="text-emerald-600" role="status">{{ success }}</p>
   <template v-if="rules">
    <section class="card p-6 space-y-4"><h2 class="text-lg font-semibold">功能与计量</h2><div class="flex flex-wrap gap-6 items-center"><label class="flex items-center gap-2"><input v-model="rules.enabled" type="checkbox" />启用自动权益</label><label>专属组门槛<input v-model.number="rules.access_threshold" type="number" min="100" class="input mt-2 w-40" /></label><label>计量单位<input :value="rules.currency" readonly class="input mt-2 w-24" /></label></div><div class="flex gap-4 flex-wrap"><label v-for="(_,currency) in rules.exchange_rates" :key="currency">{{ currency }} → USD<input v-model.number="rules.exchange_rates[currency]" type="number" min="0.000001" step="0.000001" class="input mt-2 w-40" /></label><div><label>新增币种<input v-model="newCurrency" maxlength="3" class="input mt-2 w-24" /></label><button class="btn btn-secondary mt-2" @click="addCurrency">添加</button></div></div><p class="text-xs text-gray-500">换算值须按实际支付口径确认。未配置的支付币种在启用 VIP 后不能创建充值订单；老订单不自动回填。</p></section>
    <section class="card p-6"><h2 class="text-lg font-semibold mb-4">等级阶梯</h2><div class="overflow-x-auto"><table class="w-full text-sm"><thead><tr><th>等级</th><th>累计充值</th><th>并发目标</th><th>RPM 目标</th><th>返利 %</th></tr></thead><tbody><tr v-for="t in rules.tiers" :key="t.level"><td class="p-2">VIP {{ t.level }}</td><td class="p-2"><input v-model.number="t.threshold" type="number" :readonly="t.level===1" min="100" class="input min-w-28" /></td><td class="p-2"><input v-model.number="t.concurrency" type="number" min="1" max="1000" class="input min-w-20" /></td><td class="p-2"><input v-model.number="t.rpm" type="number" min="0" max="1000" class="input min-w-20" /></td><td class="p-2"><input v-model.number="t.rebate_percent" type="number" min="0" max="10" class="input min-w-20" /></td></tr></tbody></table></div><p class="mt-3 text-xs text-gray-500">并发不会降低原基础额度。RPM 为 0 时沿用原有限流；当前不限流基础不被自动改为有限额度。</p></section>
    <section class="card p-6"><h2 class="text-lg font-semibold mb-4">分组权益</h2><div class="overflow-x-auto"><table class="w-full text-sm"><thead><tr><th>分组</th><th>属性 / 基础倍率</th><th>VIP 准入</th><th>下限</th><th v-for="n in 5" :key="n">VIP {{ n }} 减免</th></tr></thead><tbody><tr v-for="g in groups" :key="g.id" class="border-b"><td class="p-2 whitespace-nowrap">{{ g.name }}</td><td class="p-2 whitespace-nowrap">{{ excluded(g) ? '私人 / 订阅 / 测试' : g.is_exclusive ? '专属' : '公开' }} · {{ g.rate_multiplier }}</td><td class="p-2"><input v-model="rule(g.id).access" type="checkbox" :disabled="!g.is_exclusive || excluded(g)" /></td><td class="p-2"><input v-model.number="rule(g.id).floor" type="number" min="0" :max="g.rate_multiplier" step="0.001" :disabled="g.is_exclusive || excluded(g)" class="input min-w-24" /></td><td v-for="n in 5" :key="n" class="p-2"><input v-model.number="rule(g.id).discounts[n-1]" type="number" min="0" max="0.1" step="0.001" :disabled="g.is_exclusive || excluded(g)" class="input min-w-24" /></td></tr></tbody></table></div></section>
    <section class="card p-6 space-y-4"><h2 class="text-lg font-semibold">用户有效权益与人工覆盖</h2><div class="flex items-end gap-3"><label>用户 ID<input v-model.number="userId" type="number" min="1" class="input mt-2 w-36" /></label><button class="btn btn-secondary" :disabled="busy||!userId" @click="loadUser">查看</button><router-link to="/admin/users" class="btn btn-secondary">用户管理</router-link></div><template v-if="userState"><div class="flex gap-6 flex-wrap text-sm"><span>累计 ${{ userState.total.toFixed(2) }}</span><span>充值等级 VIP {{ userState.tier.level }}</span><span>有效并发 {{ userState.concurrency }}</span><span>返利 {{ userState.rebate_percent }}%</span></div><div class="overflow-x-auto"><table class="w-full text-sm"><thead><tr><th>权益</th><th>有效人工值</th><th>原因</th><th>操作</th></tr></thead><tbody><tr v-for="o in userState.overrides" :key="o.benefit"><td class="py-3">{{ benefits[o.benefit] }}</td><td>{{ o.value }}</td><td>{{ o.reason }}</td><td><button class="btn btn-secondary btn-sm" @click="restore(o.benefit)">恢复自动</button></td></tr></tbody></table></div><div class="flex flex-wrap gap-3 items-end"><label>权益<select v-model="override.benefit" class="input mt-2"><option v-for="(label,key) in benefits" :key="key" :value="key">{{ label }}</option></select></label><label>指定值<input v-model.number="override.value" type="number" step="0.001" min="0" class="input mt-2 w-28" /></label><label>到期时间（可选）<input v-model="expiry" type="datetime-local" class="input mt-2" /></label><label>原因<input v-model="override.reason" class="input mt-2" /></label><button class="btn btn-primary" :disabled="busy" @click="setOverride">保存覆盖</button></div><p class="text-xs text-gray-500">访问资格 1 为授予候选 VIP 组、0 为禁用自动资格；等级标识 0～5；减免 0～0.1；返利 0～10。已有人工组授权和定制价格仍由原入口管理。</p><details><summary class="cursor-pointer text-sm">历史充值初始确认</summary><div class="mt-3 flex gap-3 flex-wrap items-end"><label>确认金额<input v-model.number="openingAmount" type="number" min="0" class="input mt-2" /></label><label>核对原因<input v-model="openingReason" class="input mt-2" /></label><button class="btn btn-secondary" :disabled="busy" @click="opening">记录一次性初始额</button></div><p class="text-xs mt-2 text-gray-500">每个用户只能创建一次初始记录，不改变余额。须先核对已有成长流水，避免把已计入的充值重复回填。</p></details></template></section>
   </template>
  </div>
 </AppLayout>
</template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { apiClient } from '@/api/client'
import { getVIP, getVIPRules, saveVIPRules, saveVIPOverride, clearVIPOverride, createVIPOpening, type VIPRules, type VIPSnapshot, type VIPOverride } from '@/api/vip'
import type { Group } from '@/types'
const rules = ref<VIPRules | null>(null)
const groups = ref<Group[]>([])
const busy = ref(false)
const error = ref('')
const success = ref('')
const userId = ref(0)
const userState = ref<VIPSnapshot | null>(null)
const newCurrency = ref('')
const expiry = ref('')
const openingAmount = ref(0)
const openingReason = ref('')
const override = ref<VIPOverride>({ benefit: 'badge', value: 1, reason: '', expires_at: null })
const benefits: Record<string,string> = { discount:'普通组减免',access:'自动专属资格',concurrency:'并发',rpm:'RPM',rebate:'邀请返利比例',badge:'身份标识' }
const excluded = (g: Group) => ['zth-plus','zth-pro','ceshi','ceshi-gemini'].includes(g.name) || g.subscription_type === 'subscription'
function rule(id: number) { const r = rules.value!.groups.find(g => g.group_id === id); if (!r) throw new Error('分组规则缺失'); return r }
function addCurrency() { const currency = newCurrency.value.toUpperCase(); if (/^[A-Z]{3}$/.test(currency) && rules.value) { rules.value.exchange_rates[currency] = 1; newCurrency.value = '' } }
async function action(work: () => Promise<void>) { busy.value = true; error.value = ''; success.value = ''; try { await work(); success.value = '操作已保存' } catch(e) { error.value = e instanceof Error ? e.message : '操作失败' } finally { busy.value = false } }
async function save() { if (rules.value) await action(async () => saveVIPRules(rules.value!)) }
async function loadUser() { await action(async () => { userState.value = await getVIP(userId.value) }) }
async function restore(benefit: string) { await action(async () => { await clearVIPOverride(userId.value,benefit); userState.value = await getVIP(userId.value) }) }
async function setOverride() { await action(async () => { await saveVIPOverride(userId.value,{...override.value,expires_at:expiry.value ? new Date(expiry.value).toISOString() : null}); userState.value = await getVIP(userId.value) }) }
async function opening() { await action(async () => { await createVIPOpening(userId.value,openingAmount.value,openingReason.value); userState.value = await getVIP(userId.value) }) }
onMounted(async () => { await action(async () => { const [config,all] = await Promise.all([getVIPRules(),apiClient.get<Group[]>('/admin/groups/all')]); rules.value = config; rules.value.exchange_rates ||= { USD:1 }; groups.value = all.data; for (const g of groups.value) { if (!config.groups.some(r => r.group_id === g.id)) config.groups.push({group_id:g.id,private:excluded(g),access:false,floor:g.rate_multiplier,discounts:[0,0,0,0,0]}) } }) })
</script>
