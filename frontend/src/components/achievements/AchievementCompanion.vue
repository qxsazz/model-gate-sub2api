<template>
  <div class="calendar-extras">
    <section class="day-letter" aria-label="日期小笺">
      <p class="eyebrow">
        {{ selectedDate === state.date ? '今日小笺' : '这一天的小笺' }}
      </p>
      <div class="letter-date">
        <strong>{{ Number(selectedDate.slice(8)) }}</strong>
        <div>
          <span>{{ Number(selectedDate.slice(5, 7)) }}月 · {{ weekday }}</span
          ><small>{{ lunarDate(selectedDate) }}</small>
        </div>
      </div>
      <div class="letter-whisper">
        <span class="whisper-seal">宜</span>
        <p>{{ whisper[0] }}</p>
      </div>
      <div class="letter-whisper">
        <span class="whisper-seal quiet">忌</span>
        <p>{{ whisper[1] }}</p>
      </div>
      <p class="letter-footnote">一份随心的日常提示，愿今天从容一些。</p>
    </section>
    <section class="zodiac-letter" aria-label="星座能量签">
      <div class="zodiac-heading">
        <span class="eyebrow">星座能量签</span
        ><select
          :value="state.zodiac || ''"
          :disabled="saving"
          aria-label="选择星座"
          @change="save($event)"
        >
          <option value="">选择你的星座</option>
          <option v-for="z in zodiacSigns" :key="z[0]" :value="z[0]">
            {{ z[1] }}
          </option>
        </select>
      </div>
      <div class="zodiac-intro">
        <span class="zodiac-symbol"
          ><template v-if="sign">{{ sign[2] }}</template
          ><AchievementIcon v-else name="sun"
        /></span>
        <div>
          <span class="zodiac-keyword">{{
            sign ? '今日关键词 · ' + sign[3] : '给今天一点温柔的能量'
          }}</span>
          <h3>{{ sign ? energy[0] : '收下一点好心情' }}</h3>
        </div>
        <span class="zodiac-day">{{
          selectedDate.slice(5).replace('-', ' / ')
        }}</span>
      </div>
      <p class="zodiac-message">
        {{
          sign
            ? energy[1]
            : '选一个星座，收下一句轻松的小建议。不需要提供生日，也不必急着决定。'
        }}
      </p>
      <div class="energy-suggestion">
        <AchievementIcon name="sun" />
        <div>
          <span>给你的一点能量</span>
          <p>
            {{
              sign ? energy[2] : '先完成最重要的一小步，再给自己一个短暂休息。'
            }}
          </p>
        </div>
      </div>
      <p class="letter-footnote">
        趣味陪伴，随心参考。今天怎样度过，仍由你决定。
      </p>
      <p v-if="error" class="quiz-error" role="alert">{{ error }}</p>
    </section>
  </div>
  <section
    class="memory-ribbon"
    :aria-label="'相伴小记，自 ' + (state.joined_date || '初次相遇')"
  >
    <div class="memory-mark"><AchievementIcon name="book" /></div>
    <div>
      <p class="eyebrow">相伴小记</p>
      <p>
        第
        <strong>{{ state.companionship_days ?? '—' }}</strong>
        天，我们仍在这里相逢。<strong>{{
          state.medals.filter((m) => m.unlocked).length
        }}</strong>
        枚徽章，记下了你的好奇与坚持。不用把每一天都过成冲刺，按自己的节奏走，也会遇见新的风景。
      </p>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import AchievementIcon from './AchievementIcon.vue'
import './achievement-ui.css'
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
const sign = computed(() =>
  zodiacSigns.find((z) => z[0] === props.state.zodiac),
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
