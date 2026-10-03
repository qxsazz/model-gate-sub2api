<template>
  <section class="exploration">
    <header>
      <div>
        <p class="eyebrow">THE EXPLORATION ATLAS</p>
        <h2>从初航开始，收藏探索篇章</h2>
        <p>知识与实践各六个主题，10 题答对 8 题即可通关；不同主题各计一次。</p>
      </div>
      <span class="stamp"
        >初航篇章<br /><strong>{{ themeProgress }} / 4</strong></span
      >
    </header>
    <div class="theme-track">
      <span
        v-for="t in firstVoyage"
        :key="t.key"
        :class="{ done: hasPass(t.kind, t.key) }"
        >{{ hasPass(t.kind, t.key) ? '✓' : '○' }} {{ t.name }}</span
      >
    </div>
    <p class="theme-note">
      完成初识平台、选择接入地址、构造首次请求和核对使用成本，即可收藏「初航」篇章。实践为场景演练，不消耗调用额度。后续篇章待开放。
    </p>
    <p v-if="error" role="alert" class="activity-notice">
      {{ error }} <button @click="load">重试</button>
    </p>
    <div class="topic-grid">
      <button
        v-for="t in topics"
        :key="t.kind + t.key"
        :disabled="busy"
        @click="start(t)"
      >
        <span class="eyebrow">{{
          t.kind === 'knowledge' ? 'KNOWLEDGE' : 'PRACTICE'
        }}</span>
        <h3>{{ t.name }}</h3>
        <p>{{ t.description }}</p>
        <span class="topic-state">{{
          hasPass(t.kind, t.key) ? '✓ 已通关 · 再次练习' : '开始探索 →'
        }}</span>
      </button>
    </div>
    <dialog ref="dialog" class="quiz-dialog" @close="close">
      <template v-if="quiz"
        ><button
          class="close"
          :disabled="busy"
          aria-label="关闭练习"
          @click="dialog?.close()"
        >
          ×
        </button>
        <p class="eyebrow">
          {{ quiz.topic.kind === 'knowledge' ? 'KNOWLEDGE' : 'PRACTICE' }} / 10
          QUESTIONS
        </p>
        <h2>{{ quiz.topic.name }}</h2>
        <p class="quiz-rule">
          答对 8 题即可通关 · 提交后由服务器核验 · 24 小时内最多提交 30 次
        </p>
        <form @submit.prevent="submit">
          <fieldset
            v-for="(q, index) in quiz.questions"
            :key="index"
            :disabled="busy || !!result || !!submittedAnswers"
          >
            <legend>{{ index + 1 }}. {{ q.prompt }}</legend>
            <label v-for="(option, i) in q.options" :key="i"
              ><input
                v-model="answers[index]"
                type="radio"
                :name="'q' + index"
                :value="i"
                required
              />{{ option }}</label
            >
          </fieldset>
          <p v-if="quizError" class="activity-notice" role="alert">
            {{ quizError }}
            <span v-if="submittedAnswers">重试会核验已提交的同一份答案。</span>
            <button
              v-if="submittedAnswers"
              type="button"
              :disabled="busy"
              @click="newAttempt"
            >
              修改答案并开始新尝试
            </button>
          </p>
          <div v-if="result" class="quiz-result" role="status">
            <strong
              >{{ result.score }} / 10 ·
              {{ result.passed ? '已通关' : '继续探索' }}</strong
            >
            <p>
              {{
                result.passed
                  ? '这一主题已收入你的探索记录。'
                  : '至少答对 8 题才能通关。可以查阅使用文档后再次练习。'
              }}
            </p>
            <button type="button" @click="dialog?.close()">回到探索册</button>
          </div>
          <button
            v-else
            class="submit"
            :disabled="busy || answers.some((a) => a < 0)"
          >
            {{ busy ? '正在核验…' : '提交探索答案' }}
          </button>
        </form></template
      >
    </dialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  getActivityTopics,
  getActivityQuiz,
  submitActivityQuiz,
  type ActivityTopic,
  type ActivityQuiz,
} from '@/api/achievements'
const props = defineProps<{ passes: { kind: string; topic: string }[] }>(),
  emit = defineEmits<{ updated: [] }>()
const topics = ref<ActivityTopic[]>([]),
  error = ref(''),
  quizError = ref(''),
  quiz = ref<ActivityQuiz | null>(null),
  answers = ref<number[]>([]),
  submittedAnswers = ref<number[] | null>(null),
  busy = ref(false),
  dialog = ref<HTMLDialogElement>(),
  result = ref<{ score: number; passed: boolean } | null>(null),
  requestKey = ref('')
const firstVoyage = [
  { kind: 'knowledge', key: 'intro', name: '初识平台' },
  { kind: 'practice', key: 'endpoint', name: '选择接入地址' },
  { kind: 'practice', key: 'request', name: '构造首次请求' },
  { kind: 'practice', key: 'cost', name: '核对使用成本' },
]
const hasPass = (kind: string, key: string) =>
  props.passes.some((p) => p.kind === kind && p.topic === key)
const themeProgress = computed(
  () => firstVoyage.filter((t) => hasPass(t.kind, t.key)).length,
)
const err = (e: unknown) => {
  const v = e as {
    message?: string
    response?: { data?: { message?: string } }
  }
  return v.response?.data?.message ?? v.message ?? '暂时无法连接'
}
async function load() {
  try {
    topics.value = await getActivityTopics()
    error.value = ''
  } catch (e) {
    error.value = err(e)
  }
}
async function start(t: ActivityTopic) {
  busy.value = true
  try {
    quiz.value = await getActivityQuiz(t.kind, t.key)
    answers.value = Array(10).fill(-1)
    submittedAnswers.value = null
    result.value = null
    quizError.value = ''
    requestKey.value = crypto.randomUUID()
    dialog.value?.showModal()
  } catch (e) {
    error.value = err(e)
  } finally {
    busy.value = false
  }
}
async function submit() {
  if (!quiz.value || busy.value) return
  busy.value = true
  if (!submittedAnswers.value) submittedAnswers.value = [...answers.value]
  try {
    result.value = await submitActivityQuiz(
      quiz.value.topic.kind,
      quiz.value.topic.key,
      submittedAnswers.value,
      requestKey.value,
    )
    emit('updated')
  } catch (e) {
    quizError.value = err(e)
  } finally {
    busy.value = false
  }
}
function close() {
  quiz.value = null
  result.value = null
}
onMounted(load)
function newAttempt() {
  submittedAnswers.value = null
  requestKey.value = crypto.randomUUID()
  quizError.value = ''
}
</script>
<style scoped>
.exploration {
  background: var(--paper);
  border: 1px solid var(--line);
  padding: 26px;
  margin-top: 27px;
}
.exploration header {
  display: flex;
  justify-content: space-between;
  gap: 25px;
}
.eyebrow {
  font:
    10px Consolas,
    monospace;
  letter-spacing: 2px;
  color: var(--gold);
}
h2,
h3 {
  font-family: 'SimSun', serif;
  font-weight: 500;
}
h2 {
  font-size: 23px;
  margin: 9px 0;
}
p {
  font-size: 11px;
  color: var(--muted);
  line-height: 1.8;
}
.stamp {
  font-size: 11px;
  text-align: center;
  min-width: 95px;
  border: 1px solid var(--line);
  padding: 10px;
  color: var(--gold);
}
.stamp strong {
  display: block;
  font: 24px Consolas;
  margin-top: 8px;
}
.theme-track {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
  margin-top: 22px;
}
.theme-track span {
  font-size: 12px;
  padding: 13px;
  background: var(--wash);
  color: var(--muted);
}
.theme-track .done {
  color: var(--gold);
}
.theme-note {
  margin-top: 12px;
}
.topic-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 14px;
  margin-top: 22px;
}
.topic-grid > button {
  text-align: left;
  padding: 19px;
  border: 1px solid var(--line);
  background: transparent;
  color: var(--ink);
}
h3 {
  font-size: 18px;
  margin: 10px 0;
}
.topic-state {
  font-size: 11px;
  color: var(--gold);
  display: block;
  margin-top: 15px;
}
.quiz-dialog {
  background: var(--paper, #fffefa);
  color: var(--ink, #292c29);
  width: min(670px, 94vw);
  max-height: 90vh;
  overflow: auto;
  border: 1px solid var(--line, #e3dfd3);
  padding: 30px;
}
.quiz-dialog::backdrop {
  background: #101812aa;
}
.close {
  position: absolute;
  right: 12px;
  top: 10px;
  font-size: 25px;
  border: 0;
  background: none;
  color: inherit;
}
.quiz-rule {
  margin: 15px 0;
}
.quiz-dialog fieldset {
  margin: 20px 0;
  padding: 15px;
  border: 1px solid var(--line, #e3dfd3);
  font-size: 13px;
}
.quiz-dialog legend {
  padding: 0 7px;
}
.quiz-dialog label {
  display: flex;
  gap: 10px;
  align-items: center;
  margin: 12px 0;
  font-size: 12px;
}
.quiz-dialog input {
  accent-color: var(--gold, #806635);
}
.submit,
.quiz-result button {
  width: 100%;
  background: var(--gold, #806635);
  color: var(--paper, #fffefa);
  border: 0;
  padding: 13px;
}
.quiz-result {
  padding: 20px;
  text-align: center;
  border: 1px solid var(--line, #e3dfd3);
}
.quiz-result strong {
  font-size: 19px;
}
.activity-notice {
  padding: 12px;
  border: 1px solid var(--line);
  color: var(--gold);
}
button {
  cursor: pointer;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
button:focus-visible,
input:focus-visible {
  outline: 2px solid var(--gold);
  outline-offset: 3px;
}
:global(.dark .quiz-dialog) {
  --paper: #1e2421;
  --ink: #e6e2d6;
  --muted: #a0a497;
  --gold: #d2ba86;
  --line: #3b4239;
}
@media (max-width: 850px) {
  .topic-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .theme-track {
    grid-template-columns: repeat(2, 1fr);
  }
}
@media (max-width: 520px) {
  .exploration {
    padding: 20px 15px;
  }
  .topic-grid {
    grid-template-columns: 1fr;
  }
  .quiz-dialog {
    padding: 24px 18px;
  }
}
</style>
