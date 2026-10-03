import {mount,flushPromises} from '@vue/test-utils'
import {describe,it,expect,vi} from 'vitest'
import ActivityExplorer from '../ActivityExplorer.vue'
const mocks=vi.hoisted(()=>({topics:vi.fn(),quiz:vi.fn(),submit:vi.fn()}))
vi.mock('@/api/achievements',()=>({getActivityTopics:mocks.topics,getActivityQuiz:mocks.quiz,submitActivityQuiz:mocks.submit}))
describe('activity receipt retry',()=>{
 it('freezes submitted answers until replay or explicit new attempt',async()=>{
  HTMLDialogElement.prototype.showModal=vi.fn()
  mocks.topics.mockResolvedValue([{kind:'knowledge',key:'intro',name:'初识平台',description:'test'}])
  mocks.quiz.mockResolvedValue({topic:{kind:'knowledge',key:'intro',name:'初识平台'},questions:Array.from({length:10},(_,i)=>({prompt:'问题'+i,options:['甲','乙','丙']}))})
  mocks.submit.mockRejectedValueOnce(new Error('响应丢失')).mockResolvedValue({score:10,passed:true})
  const w=mount(ActivityExplorer,{props:{passes:[]}});await flushPromises()
  await w.get('.topic-grid button').trigger('click');await flushPromises()
  for(const field of w.findAll('fieldset'))await field.find('input').setValue(true)
  await w.get('form').trigger('submit');await flushPromises()
  expect(w.get('fieldset').attributes('disabled')).toBeDefined()
  const first=mocks.submit.mock.calls[0]!
  await w.get('form').trigger('submit');await flushPromises()
  expect(mocks.submit.mock.calls[1]![2]).toEqual(first[2])
  expect(mocks.submit.mock.calls[1]![3]).toEqual(first[3])
  expect(w.text()).toContain('已通关')
 })
})
