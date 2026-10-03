<template>
  <AppLayout
    ><main class="settings">
      <header>
        <div>
          <p class="eyebrow">MODEL-GATE / GROWTH OPERATIONS</p>
          <h1>成就与签到设置</h1>
          <p>现金试点按账户开放，签到、收藏与佩戴独立保存。</p>
        </div>
        <router-link to="/achievements" class="btn btn-secondary"
          >查看成就册</router-link
        >
      </header>
      <p v-if="error" class="status" role="alert">
        {{ error }} <button @click="load">重新读取</button>
      </p>
      <p v-if="message" class="status" role="status">{{ message }}</p>
      <form v-if="config" @submit.prevent="save">
        <section>
          <h2>即时到账与范围</h2>
          <label class="switch"
            ><input
              v-model="config.cash_enabled"
              type="checkbox"
            />开放每日签到现金奖励</label
          ><label class="switch"
            ><input
              v-model="config.milestone_cash_enabled"
              type="checkbox"
            />开放成就里程碑奖励</label
          >
          <p>
            签到现金关闭时仍可签到和收集徽章。成就现金需要同时开放签到现金、加入试点名单并满足近期使用条件。
          </p>
          <label class="field"
            >现金试点账户 ID<textarea
              v-model="allowlist"
              rows="3"
              placeholder="例如：12, 35, 80"
            />
          </label>
          <p>
            最多 100
            个已核验的账户，用逗号或空格分隔。只对名单内账户发奖；没有近 30
            日有效充值或余额计费使用的账户仍不发奖。测试管理员可加入名单，展示等级授权不会提高现金档位。
          </p>
        </section>
        <section>
          <h2>发奖预算（USD）</h2>
          <div class="fields">
            <label class="field"
              >每日总预算<input
                v-model.number="config.daily_budget"
                type="number"
                min="0"
                max="1000"
                step="0.01"
                required /></label
            ><label class="field"
              >每月总预算<input
                v-model.number="config.monthly_budget"
                type="number"
                min="0"
                max="10000"
                step="0.01"
                required
            /></label>
          </div>
          <p>
            签到与成就共用预算，按照发奖原额累计。退款不释放预算；预算不足时签到记录保留，现金不会补发到历史日期。
          </p>
        </section>
        <section>
          <h2>VIP 每日签到权益</h2>
          <div class="tiers">
            <div v-for="(amount, index) in config.daily_rewards" :key="index">
              <span>{{ index ? 'VIP ' + index : '普通会员' }}</span
              ><strong>${{ amount.toFixed(2) }}</strong>
            </div>
          </div>
          <p>
            按签到时累计有效充值对应的实际成长等级计算。VIP
            总开关关闭时使用普通会员金额。当前配置不支持改写历史奖励；原额、抵扣和净到账均留存。签到按北京时间自然日划分。
          </p>
        </section>
        <footer>
          <button class="btn btn-primary" :disabled="busy">
            {{ busy ? '正在保存…' : '保存试点配置' }}</button
          ><span>保存操作会进入管理审计，并记录配置修订。</span>
        </footer>
      </form>
      <p v-else-if="!error">正在读取奖励配置…</p>
    </main></AppLayout
  >
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import {
  getAchievementConfig,
  saveAchievementConfig,
  type AchievementConfig,
} from '@/api/achievements'
const config = ref<AchievementConfig | null>(null),
  allowlist = ref(''),
  error = ref(''),
  message = ref(''),
  busy = ref(false)
const err = (e: unknown) => {
  const v = e as {
    message?: string
    response?: { data?: { message?: string } }
  }
  return v.response?.data?.message ?? v.message ?? '暂时无法连接'
}
async function load() {
  try {
    config.value = await getAchievementConfig()
    allowlist.value = config.value.cash_allowlist.join(', ')
    error.value = ''
  } catch (e) {
    error.value = err(e)
  }
}
async function save() {
  if (!config.value || busy.value) return
  error.value = ''
  message.value = ''
  const ids = allowlist.value.trim()
    ? allowlist.value
        .trim()
        .split(/[,，\s]+/)
        .map(Number)
    : []
  if (ids.some((id) => !Number.isSafeInteger(id) || id <= 0)) {
    error.value = '账户 ID 必须为正整数'
    return
  }
  busy.value = true
  try {
    await saveAchievementConfig({ ...config.value, cash_allowlist: ids })
    message.value = '奖励配置已保存'
    await load()
  } catch (e) {
    error.value = err(e)
  } finally {
    busy.value = false
  }
}
onMounted(load)
</script>
<style scoped>
.settings {
  max-width: 1000px;
  margin: auto;
  padding: 32px 25px;
}
.settings header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 20px;
  margin-bottom: 27px;
}
.eyebrow {
  font: 10px Consolas;
  letter-spacing: 2px;
  color: #9c804b;
}
h1,
h2 {
  font-family: 'SimSun', serif;
  font-weight: 500;
}
h1 {
  font-size: 28px;
  margin: 10px 0;
}
h2 {
  font-size: 21px;
  margin-bottom: 18px;
}
p {
  font-size: 12px;
  line-height: 1.9;
  opacity: 0.7;
}
.settings section {
  border: 1px solid #a58e5538;
  padding: 25px;
  margin: 18px 0;
}
.switch {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  margin: 16px 0;
}
.field {
  display: grid;
  gap: 8px;
  font-size: 12px;
  margin-top: 20px;
}
.field input,
.field textarea {
  background: transparent;
  border: 1px solid #a58e5550;
  padding: 12px;
  color: inherit;
}
.fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
.tiers {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 15px;
}
.tiers span {
  font-size: 11px;
}
.tiers strong {
  display: block;
  font: 23px 'Times New Roman';
  margin: 12px 0;
  color: #9c804b;
}
footer {
  display: flex;
  gap: 18px;
  align-items: center;
}
footer span {
  font-size: 11px;
  opacity: 0.65;
}
.status {
  padding: 15px;
  border: 1px solid #a58e5550;
}
.status button {
  text-decoration: underline;
  margin-left: 10px;
}
button:disabled {
  opacity: 0.5;
}
@media (max-width: 650px) {
  .fields {
    grid-template-columns: 1fr;
  }
  .tiers {
    grid-template-columns: repeat(3, 1fr);
  }
  .settings header,
  footer {
    align-items: start;
    flex-direction: column;
  }
}
</style>
