<template>
  <section class="exploration">
    <p v-if="error" role="alert" class="quiz-error">
      {{ error }} <button @click="load">重试</button>
    </p>
    <section v-if="group === 'chapter'" class="chapter-tasks">
      <div class="section-head">
        <div>
          <h3>初航篇章 · 完成以下 4 项任务</h3>
          <p>这里的任务分别来自知识与实践，可直接进入，无需来回寻找。</p>
        </div>
        <span class="muted">{{ themeProgress }} / 4</span>
      </div>
      <div class="topic-grid chapter-list">
        <div
          v-for="t in visibleTopics"
          :key="t.kind + t.key"
          class="chapter-task"
          :class="{ completed: hasPass(t.kind, t.key) }"
        >
          <span>{{ hasPass(t.kind, t.key) ? '✓' : '○' }} {{ t.name }}</span
          ><button class="text-link" :disabled="busy" @click="start(t)">
            {{ hasPass(t.kind, t.key) ? '再练习' : '去完成' }}
            <AchievementIcon name="arrow" />
          </button>
        </div>
      </div>
      <p class="quiet-note">
        <AchievementIcon
          name="info"
        />篇章同行、万象收藏暂未开放，不会计入当前可领取奖励。
      </p>
    </section>
    <template v-else
      ><div class="task-layout">
        <section class="task-list">
          <div class="task-list-head">
            <div>
              <h3>本系列任务</h3>
              <p>每个主题 10 题，答对至少 8 题即可通关</p>
            </div>
            <button @click="hideCompleted = !hideCompleted">
              {{ hideCompleted ? '显示已通过' : '收起已通过' }}
            </button>
          </div>
          <div class="topic-grid">
            <div v-for="t in taskTopics" :key="t.kind + t.key" class="task-row">
              <span class="task-icon"
                ><AchievementIcon
                  :name="
                    activityPresentation[t.kind + ':' + t.key]?.icon || 'book'
                  "
              /></span>
              <div class="task-copy">
                <strong>{{ t.name }}</strong
                ><span v-if="hasPass(t.kind, t.key)" class="completed-check"
                  >✓ 已通过</span
                >
                <p>{{ t.description }}</p>
              </div>
              <span class="task-meta">10 题 · 8 题通关</span
              ><button class="task-action" :disabled="busy" @click="start(t)">
                {{ hasPass(t.kind, t.key) ? '再次练习' : '开始挑战'
                }}<AchievementIcon name="arrow" />
              </button>
            </div>
          </div>
          <p v-if="!taskTopics.length" class="quiet-note">
            已通过的任务已收起，你可以随时再次练习。
          </p>
        </section>
        <aside v-if="goal" class="next-goal">
          <p class="eyebrow">
            {{ goal.unlocked ? '本系列已点亮' : '下一枚徽章' }}
          </p>
          <MedalArt :medal="goal.key" :name="goal.name" class="goal-art" />
          <h3>{{ goal.name }}</h3>
          <p>
            {{
              goal.unlocked
                ? '本系列徽章已全部解锁，可领取尚未领取的奖励。'
                : `再通过 ${Math.max(0, goal.target - goal.progress)} 个${group === 'knowledge' ? '知识主题' : '实践关卡'}即可解锁。`
            }}
          </p>
          <div class="goal-track">
            <template v-for="(n, index) in stages" :key="n"
              ><span :class="{ done: passedCount >= n }">{{
                passedCount >= n ? '✓' : n
              }}</span
              ><i v-if="index < stages.length - 1"
            /></template>
          </div>
          <p>
            单枚勋章获得补签卡。<template v-if="seriesReward !== undefined"
              >集齐 3 枚后，另可领取 ${{ seriesReward.toFixed(2) }}。</template
            >
          </p>
          <button
            class="primary"
            :disabled="busy"
            @click="nextTopic ? start(nextTopic) : emit('showSeries')"
          >
            {{ nextTopic ? '继续下个任务' : '查看系列奖励'
            }}<AchievementIcon name="arrow" />
          </button>
        </aside>
      </div>
      <p class="quiet-note">
        <AchievementIcon name="info" />{{
          group === 'practice'
            ? '实践为场景答题演练，不调用真实模型，也不消耗账户额度。'
            : '不同知识主题分别计入进度，重复练习不会重复增加收集数量。'
        }}
      </p></template
    >
    <dialog
      ref="dialog"
      class="quiz-dialog"
      @close="close"
      @cancel="protectAttempt"
    >
      <template v-if="quiz"
        ><div class="dialog-top">
          <strong
            >{{ quiz.topic.kind === 'knowledge' ? '知识挑战' : '实践演练' }} /
            {{ quizTitle }}</strong
          ><button
            class="close"
            :disabled="busy || (!!submittedAnswers && !result)"
            aria-label="关闭练习"
            @click="dialog?.close()"
          >
            <AchievementIcon name="close" />
          </button>
        </div>
        <div class="quiz-body">
          <div class="quiz-label">
            <span>{{
              quiz.topic.kind === 'knowledge'
                ? '学习与理解'
                : '场景判断 · 不消耗调用额度'
            }}</span
            ><span>答对 8 题通关</span>
          </div>
          <h2>{{ quizTitle }}</h2>
          <div v-if="result" class="quiz-result" role="status">
            <strong
              >{{ result.score
              }}<small> / {{ quiz.questions.length }}</small></strong
            >
            <h3>{{ result.passed ? '挑战通过' : '再试一次，把知识掌握好' }}</h3>
            <p>
              {{
                result.passed
                  ? '该主题已计入对应系列的勋章进度。'
                  : '至少答对 8 题才能通关，可查阅使用文档后重新练习。'
              }}
            </p>
            <button
              v-if="!result.passed"
              class="primary"
              type="button"
              @click="newAttempt"
            >
              重新挑战</button
            ><button class="primary" type="button" @click="dialog?.close()">
              返回任务，查看勋章进度</button
            ><span class="sr-only"
              >{{ result.score }} / {{ quiz.questions.length }} ·
              {{ result.passed ? '已通关' : '继续探索' }}</span
            >
          </div>
          <form v-else @submit.prevent="submit">
            <div class="question-progress">
              <span
                v-for="(_, index) in quiz.questions"
                :key="index"
                :class="{
                  filled: answers[index] >= 0 || index === questionIndex,
                }"
              />
            </div>
            <p class="question-number">
              第 {{ questionIndex + 1 }} / {{ quiz.questions.length }} 题
            </p>
            <fieldset
              :key="questionIndex"
              :disabled="busy || !!submittedAnswers"
            >
              <legend class="question-title">
                {{ quiz.questions[questionIndex].prompt }}
              </legend>
              <div class="answer-options">
                <label
                  v-for="(option, index) in quiz.questions[questionIndex]
                    .options"
                  :key="index"
                  class="answer-option"
                  ><input
                    v-model="answers[questionIndex]"
                    type="radio"
                    :name="'q' + questionIndex"
                    :value="index"
                    required
                  /><span class="answer-letter">{{
                    String.fromCharCode(65 + index)
                  }}</span
                  ><span>{{ option }}</span></label
                >
              </div>
            </fieldset>
            <div v-if="quizError" class="quiz-error" role="alert">
              {{ quizError
              }}<span v-if="submittedAnswers"
                >重试会核验已提交的同一份答案。</span
              ><button
                v-if="submittedAnswers"
                type="button"
                :disabled="busy"
                @click="newAttempt"
              >
                修改答案并开始新尝试
              </button>
            </div>
            <div class="quiz-bottom">
              <button
                class="text-link"
                type="button"
                :disabled="questionIndex === 0 || busy"
                @click="questionIndex--"
              >
                上一题</button
              ><span class="muted"
                >已完成 {{ answers.filter((a) => a >= 0).length }} /
                {{ quiz.questions.length }} 题</span
              ><button
                v-if="
                  questionIndex < quiz.questions.length - 1 && !submittedAnswers
                "
                type="button"
                class="primary"
                data-next-question
                :disabled="answers[questionIndex] < 0 || busy"
                @click="questionIndex++"
              >
                下一题 <AchievementIcon name="arrow" /></button
              ><button
                v-else
                class="primary submit"
                :disabled="busy || answers.some((a) => a < 0)"
              >
                {{
                  busy
                    ? '正在核验…'
                    : submittedAnswers
                      ? '重试提交'
                      : '提交答案'
                }}<AchievementIcon name="arrow" />
              </button>
            </div>
            <p class="quiz-rule">
              每个主题单独计入进度，重复通过不会重复增加收集数量。提交后由服务器核验，24
              小时内最多提交 30 次。
            </p>
          </form>
        </div>
      </template>
    </dialog>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import MedalArt from './MedalArt.vue'
import AchievementIcon from './AchievementIcon.vue'
import './achievement-ui.css'
import { activityPresentation } from './activityPresentation'
import {
  getActivityTopics,
  getActivityQuiz,
  submitActivityQuiz,
  type ActivityTopic,
  type ActivityQuiz,
  type Medal,
} from '@/api/achievements'
const props = withDefaults(
    defineProps<{
      passes: { kind: string; topic: string }[]
      group?: 'knowledge' | 'practice' | 'chapter'
      goal?: Medal
      seriesReward?: number
    }>(),
    { group: 'knowledge' },
  ),
  emit = defineEmits<{ updated: []; showSeries: [] }>()
const hideCompleted = ref(false)
const questionIndex = ref(0)
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
const visibleTopics = computed(() =>
  topics.value.filter((t) =>
    props.group === 'chapter'
      ? firstVoyage.some((v) => v.kind === t.kind && v.key === t.key)
      : t.kind === props.group,
  ),
)
const nextTopic = computed(() =>
  visibleTopics.value.find((t) => !hasPass(t.kind, t.key)),
)
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
    topics.value = (await getActivityTopics()).map((t) => ({
      ...t,
      ...(activityPresentation[t.kind + ':' + t.key] || {}),
    }))
    error.value = ''
  } catch (e) {
    error.value = err(e)
  }
}
async function start(t: ActivityTopic) {
  busy.value = true
  try {
    quiz.value = await getActivityQuiz(t.kind, t.key)
    questionIndex.value = 0
    answers.value = Array(quiz.value.questions.length).fill(-1)
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
  if (!quiz.value || busy.value || answers.value.some((a) => a < 0)) return
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
function protectAttempt(event: Event) {
  if (busy.value || (submittedAnswers.value && !result.value))
    event.preventDefault()
}
onMounted(load)
function newAttempt() {
  if (result.value && quiz.value)
    answers.value = Array(quiz.value.questions.length).fill(-1)
  result.value = null
  questionIndex.value = 0
  submittedAnswers.value = null
  requestKey.value = crypto.randomUUID()
  quizError.value = ''
}

const quizTitle = computed(() =>
  quiz.value
    ? (activityPresentation[quiz.value.topic.kind + ':' + quiz.value.topic.key]
        ?.name ?? quiz.value.topic.name)
    : '',
)
watch(
  () => props.group,
  () => {
    hideCompleted.value = false
  },
)
const taskTopics = computed(() =>
  visibleTopics.value.filter(
    (t) => !hideCompleted.value || !hasPass(t.kind, t.key),
  ),
)
const passedCount = computed(
  () => visibleTopics.value.filter((t) => hasPass(t.kind, t.key)).length,
)
const stages = computed(() =>
  props.group === 'knowledge' ? [1, 3, 6] : [2, 4, 6],
)
</script>
