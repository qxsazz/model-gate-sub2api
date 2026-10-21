import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CanvasReviewView from '../CanvasReviewView.vue'
const { get, remove } = vi.hoisted(() => ({ get: vi.fn(), remove: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { get, delete: remove } }))
function mountView() {
  return mount(CanvasReviewView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<div v-if="show" role="dialog"><slot /><button data-test="confirm-delete" @click="$emit(\'confirm\')">Confirm</button></div>' }
  } } })
}
describe('canvas administrator deletion', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    get.mockResolvedValue({ data: { items: [{ id: 'asset-id', user_id: 7, mime_type: 'image/png', available: true, review_status: 'unreviewed', created_at: '2026-10-20T00:00:00Z', expires_at: '2026-10-27T00:00:00Z' }], has_more: false } })
    remove.mockResolvedValue({ data: { deleted: true } })
  })
  it('requires a reason and removes the deleted card instead of leaving a placeholder', async () => {
    const wrapper = mountView(); await flushPromises()
    get.mockResolvedValue({ data: { items: [], has_more: false } })
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    expect(remove).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请填写删除原因')
    await wrapper.get('textarea').setValue('违规素材处理')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(remove).toHaveBeenCalledWith('/admin/canvas/assets/asset-id', { data: { reason: '违规素材处理' } })
    expect(wrapper.find('article').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('素材已删除或过期')
    expect(wrapper.text()).toContain('暂无符合条件的素材')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.findAll('button').some(button => button.text() === '删除素材')).toBe(false)
    wrapper.unmount()
  })
  it('keeps the card removed when refreshing the list fails after successful deletion', async () => {
    const wrapper = mountView(); await flushPromises()
    get.mockRejectedValue(new Error('列表暂不可用'))
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('textarea').setValue('test reason')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(wrapper.find('article').exists()).toBe(false)
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('列表暂不可用')
    wrapper.unmount()
  })
  it('returns to the previous page after deleting the last card on a later page', async () => {
    const first = { id: 'first', available: true, mime_type: 'image/png', prompt: '第一页素材' }
    const last = { id: 'last', available: true, mime_type: 'image/png', prompt: '最后一页素材' }
    get.mockReset()
    get.mockResolvedValueOnce({ data: { items: [first], has_more: true } })
      .mockResolvedValueOnce({ data: { items: [last], has_more: false } })
      .mockResolvedValueOnce({ data: { items: [first], has_more: false } })
    const wrapper = mountView(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '下一页')!.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('第 2 页')
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('textarea').setValue('test reason')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('第 1 页')
    expect(wrapper.text()).toContain('第一页素材')
    expect(wrapper.text()).not.toContain('最后一页素材')
    expect(get).toHaveBeenLastCalledWith('/admin/canvas/assets', { params: { user_id: '', status: '', page: 1 } })
    wrapper.unmount()
  })
  it('refills the current page after deletion when other cards remain', async () => {
    const first = { id: 'first', available: true, mime_type: 'image/png', prompt: '被删除素材' }
    const second = { id: 'second', available: true, mime_type: 'image/png', prompt: '保留素材' }
    const next = { id: 'next', available: true, mime_type: 'image/png', prompt: '补入素材' }
    get.mockResolvedValueOnce({ data: { items: [first, second], has_more: true } })
      .mockResolvedValueOnce({ data: { items: [second, next], has_more: false } })
    const wrapper = mountView(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('textarea').setValue('test reason')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.text()).not.toContain('被删除素材')
    expect(wrapper.text()).toContain('保留素材')
    expect(wrapper.text()).toContain('补入素材')
    expect(wrapper.findAll('button').find(button => button.text() === '下一页')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('ignores an older list response that arrives after deletion and its refresh', async () => {
    let resolveOld!: (value: unknown) => void
    const wrapper = mountView(); await flushPromises()
    const stale = { data: { items: [{ id: 'asset-id', available: true, mime_type: 'image/png', prompt: '旧记录' }], has_more: false } }
    get.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce({ data: { items: [], has_more: false } })
    await wrapper.findAll('button').find(button => button.text() === '查询')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('textarea').setValue('test reason')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    resolveOld(stale); await flushPromises()
    expect(wrapper.find('article').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('旧记录')
    wrapper.unmount()
  })
  it('keeps the material and confirmation visible when deletion fails', async () => {
    remove.mockRejectedValue(new Error('删除失败'))
    const wrapper = mountView(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('textarea').setValue('test reason')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('删除失败')
    expect(wrapper.text()).toContain('查看内容（记录访问）')
    wrapper.unmount()
  })
})
