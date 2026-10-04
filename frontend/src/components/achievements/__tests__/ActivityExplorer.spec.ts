import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi } from 'vitest'
import ActivityExplorer from '../ActivityExplorer.vue'
const mocks = vi.hoisted(() => ({
  topics: vi.fn(),
  quiz: vi.fn(),
  submit: vi.fn(),
}))
vi.mock('@/api/achievements', () => ({
  getActivityTopics: mocks.topics,
  getActivityQuiz: mocks.quiz,
  submitActivityQuiz: mocks.submit,
}))
describe('activity receipt retry', () => {
  it('collapses passed tasks and resets the filter when the category changes', async () => {
    mocks.topics.mockResolvedValue([
      { kind: 'knowledge', key: 'intro', name: '初识平台', description: '' },
      { kind: 'practice', key: 'endpoint', name: '接入地址', description: '' },
    ])
    const w = mount(ActivityExplorer, {
      props: { passes: [{ kind: 'knowledge', topic: 'intro' }] },
    })
    await flushPromises()
    await w.get('.task-list-head button').trigger('click')
    expect(w.findAll('.topic-grid button')).toHaveLength(0)
    await w.setProps({ group: 'practice' })
    expect(w.findAll('.topic-grid button')).toHaveLength(1)
  })
  it('lists precisely the four first-voyage tasks', async () => {
    mocks.topics.mockResolvedValue([
      { kind: 'knowledge', key: 'intro', name: '初识平台', description: '' },
      ...['endpoint', 'request', 'cost', 'debug'].map((key) => ({
        kind: 'practice',
        key,
        name: key,
        description: '',
      })),
    ])
    const w = mount(ActivityExplorer, {
      props: { passes: [], group: 'chapter' },
    })
    await flushPromises()
    expect(w.findAll('.chapter-task')).toHaveLength(4)
    expect(w.text()).not.toContain('排障演练')
  })
  it('freezes submitted answers until replay or explicit new attempt', async () => {
    HTMLDialogElement.prototype.showModal = vi.fn()
    mocks.topics.mockResolvedValue([
      {
        kind: 'knowledge',
        key: 'intro',
        name: '初识平台',
        description: 'test',
      },
    ])
    mocks.quiz.mockResolvedValue({
      topic: { kind: 'knowledge', key: 'intro', name: '初识平台' },
      questions: Array.from({ length: 10 }, (_, i) => ({
        prompt: '问题' + i,
        options: ['甲', '乙', '丙'],
      })),
    })
    mocks.submit
      .mockRejectedValueOnce(new Error('响应丢失'))
      .mockResolvedValue({ score: 10, passed: true })
    const w = mount(ActivityExplorer, { props: { passes: [] } })
    await flushPromises()
    await w.get('.topic-grid button').trigger('click')
    await flushPromises()
    expect(w.findAll('fieldset')).toHaveLength(1)
    for (let i = 0; i < 10; i++) {
      await w.get('fieldset input').setValue(true)
      if (i < 9) await w.get('[data-next-question]').trigger('click')
    }
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(w.get('fieldset').attributes('disabled')).toBeDefined()
    const uncertainCancel = new Event('cancel', { cancelable: true })
    w.get('dialog').element.dispatchEvent(uncertainCancel)
    expect(uncertainCancel.defaultPrevented).toBe(true)
    const first = mocks.submit.mock.calls[0]!
    await w.get('form').trigger('submit')
    await flushPromises()
    expect(mocks.submit.mock.calls[1]![2]).toEqual(first[2])
    expect(mocks.submit.mock.calls[1]![3]).toEqual(first[3])
    expect(w.text()).toContain('已通关')
  })
  it('shows only tasks belonging to the selected badge series', async () => {
    mocks.topics.mockResolvedValue([
      {
        kind: 'knowledge',
        key: 'intro',
        name: '初识平台',
        description: 'test',
      },
      {
        kind: 'practice',
        key: 'endpoint',
        name: '接入地址',
        description: 'test',
      },
    ])
    const w = mount(ActivityExplorer, {
      props: { passes: [], group: 'knowledge' },
    })
    await flushPromises()
    expect(w.findAll('.topic-grid button')).toHaveLength(1)
    expect(w.text()).toContain('初识平台')
    expect(w.text()).not.toContain('接入地址')
    await w.setProps({ group: 'practice' })
    expect(w.text()).toContain('接入地址')
    expect(w.text()).not.toContain('初识平台')
  })
  it('prevents Escape from discarding an in-flight attempt', async () => {
    let finish!: (v: { score: number; passed: boolean }) => void
    mocks.submit.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        }),
    )
    const w = mount(ActivityExplorer, { props: { passes: [] } })
    await flushPromises()
    await w.get('.topic-grid button').trigger('click')
    await flushPromises()
    for (let i = 0; i < 10; i++) {
      await w.get('fieldset input').setValue(true)
      if (i < 9) await w.get('[data-next-question]').trigger('click')
    }
    await w.get('form').trigger('submit')
    const cancel = new Event('cancel', { cancelable: true })
    w.get('dialog').element.dispatchEvent(cancel)
    expect(cancel.defaultPrevented).toBe(true)
    expect(w.get('.close').attributes('disabled')).toBeDefined()
    finish({ score: 10, passed: true })
    await flushPromises()
    expect(w.text()).toContain('已通关')
  })
})
