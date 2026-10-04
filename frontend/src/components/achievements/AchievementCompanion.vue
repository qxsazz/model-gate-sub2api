<template>
  <div class="calendar-extras">
    <section class="day-letter" aria-label="今日小笺">
      <p class="eyebrow">
        {{ selectedDate === state.date ? '今日小笺' : '这一天的小笺' }} · TOKEN
        使用建议
      </p>
      <div class="letter-date">
        <strong>{{ Number(selectedDate.slice(8)) }}</strong>
        <div>
          {{ Number(selectedDate.slice(5, 7)) }} 月 · {{ weekday
          }}<small>{{ lunarDate(selectedDate) }}</small>
        </div>
      </div>
      <div class="whisper">
        <span>宜</span>
        <p>{{ whisper[0] }}</p>
      </div>
      <div class="whisper avoid">
        <span>忌</span>
        <p>{{ whisper[1] }}</p>
      </div>
    </section>
    <section class="zodiac-letter" aria-label="星座小签">
      <div class="zodiac-head">
        <p class="eyebrow">星座小签</p>
        <select
          :value="state.zodiac || ''"
          :disabled="saving"
          aria-label="选择星座"
          @change="save($event)"
        >
          <option value="">选择你的星座</option>
          <option v-for="z in zodiacSigns" :key="z[0]" :value="z[0]">
            {{ z[2] }} {{ z[1] }}
          </option>
        </select>
      </div>
      <template v-if="state.zodiac"
        ><h3>{{ energy[0] }}</h3>
        <p>{{ energy[1] }}</p>
        <p class="energy-advice">{{ energy[2] }}</p></template
      >
      <template v-else
        ><h3>给今天一点温柔的能量</h3>
        <p>选一个星座，收下一句轻松的小建议。不需要提供生日。</p></template
      >
      <small>仅作娱乐与日常灵感参考。选择会保存至当前账户。</small>
      <p v-if="error" role="alert">{{ error }}</p>
    </section>
  </div>
  <section class="memory-ribbon">
    <span>一路相伴</span>
    <p>
      从 {{ state.joined_date || '初次相遇' }} 开始，我们已相伴
      <strong>{{ state.companionship_days ?? '—' }}</strong>
      天。每一次回来，都值得被好好记住。你已点亮
      {{ state.medals.filter((m) => m.unlocked).length }} 枚徽章，累计签到
      {{ state.total_days }} 天。
    </p>
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  saveAchievementZodiac,
  type AchievementState,
} from '@/api/achievements'
import {
  dayIndex,
  dayWhispers,
  zodiacSigns,
  energyLines,
  lunarDate,
} from './companion'
const props = defineProps<{ state: AchievementState; date?: string }>()
const emit = defineEmits<{ updated: [] }>()
const selectedDate = computed(() => props.date || props.state.date)
const saving = ref(false),
  error = ref('')
const weekday = computed(() =>
  new Intl.DateTimeFormat('zh-CN', {
    weekday: 'long',
    timeZone: 'Asia/Shanghai',
  }).format(new Date(selectedDate.value + 'T04:00:00Z')),
)
const whisper = computed(
  () => dayWhispers[dayIndex(selectedDate.value) % dayWhispers.length],
)
const energy = computed(
  () =>
    energyLines[
      (dayIndex(selectedDate.value) +
        Math.max(
          0,
          zodiacSigns.findIndex((z) => z[0] === props.state.zodiac),
        )) %
        energyLines.length
    ],
)
async function save(event: Event) {
  const select = event.target as HTMLSelectElement
  const zodiac = select.value
  saving.value = true
  error.value = ''
  try {
    await saveAchievementZodiac(zodiac)
    emit('updated')
  } catch {
    select.value = props.state.zodiac || ''
    error.value = '选择暂未保存，请稍后重试。'
  } finally {
    saving.value = false
  }
}
</script>
<style scoped>
.calendar-extras {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  margin-top: 24px;
  align-items: stretch;
}
.day-letter,
.zodiac-letter,
.memory-ribbon {
  background: var(--companion-surface);
  border: 1px solid var(--companion-border);
  border-radius: 12px;
  padding: 26px 30px;
  color: var(--muted);
}
.eyebrow {
  font-size: 11px;
  letter-spacing: 2px;
  color: var(--gold);
  margin: 0 0 18px;
}
.letter-date {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 20px;
  color: var(--ink);
}
.letter-date strong {
  font-size: 36px;
  font-weight: 500;
}
.letter-date small {
  display: block;
  margin-top: 5px;
  color: var(--muted);
}
.whisper {
  display: flex;
  gap: 14px;
  margin: 14px 0;
  align-items: flex-start;
}
.whisper span {
  color: var(--gold);
  background: var(--companion-emphasis);
  border-radius: 5px;
  padding: 2px 7px;
  flex-shrink: 0;
}
.whisper p {
  margin: 0;
  line-height: 1.9;
}
.avoid span {
  color: #b77d61;
  background: var(--companion-soft);
}
.zodiac-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}
.zodiac-head select {
  background: var(--companion-soft);
  border: 1px solid var(--companion-border);
  border-radius: 6px;
  color: var(--ink);
  padding: 6px;
  max-width: 160px;
}
h3 {
  font-family: 'SimSun', serif;
  font-size: 25px;
  font-weight: 500;
  color: var(--ink);
  margin: 12px 0;
}
.zodiac-letter p {
  line-height: 1.9;
}
.energy-advice {
  background: var(--companion-emphasis);
  padding: 12px;
  border-radius: 6px;
}
.zodiac-letter small {
  font-size: 11px;
  line-height: 1.7;
  display: block;
}
.memory-ribbon {
  margin-top: 24px;
  padding: 20px 30px;
}
.memory-ribbon > span {
  font-size: 11px;
  letter-spacing: 2px;
  color: var(--gold);
}
.memory-ribbon p {
  line-height: 1.9;
  margin: 10px 0 0;
}
.memory-ribbon strong {
  color: var(--gold);
  font-weight: 500;
  font-variant-numeric: tabular-nums;
}
@media (max-width: 650px) {
  .calendar-extras {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .day-letter,
  .zodiac-letter,
  .memory-ribbon {
    padding: 22px;
  }
}
</style>
