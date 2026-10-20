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
  it('requires a reason, deletes the selected asset and removes content actions', async () => {
    const wrapper = mountView(); await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === '删除素材')!.trigger('click')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click')
    expect(remove).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('请填写删除原因')
    await wrapper.get('textarea').setValue('违规素材处理')
    await wrapper.get('[data-test="confirm-delete"]').trigger('click'); await flushPromises()
    expect(remove).toHaveBeenCalledWith('/admin/canvas/assets/asset-id', { data: { reason: '违规素材处理' } })
    expect(wrapper.text()).toContain('素材已删除或过期')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.findAll('button').some(button => button.text() === '删除素材')).toBe(false)
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
