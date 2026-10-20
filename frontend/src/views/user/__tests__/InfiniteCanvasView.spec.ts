import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import InfiniteCanvasView from '../InfiniteCanvasView.vue'

const { deleteKey, request } = vi.hoisted(() => ({ deleteKey: vi.fn(), request: vi.fn() }))
vi.mock('@/api/keys', () => ({ deleteKey }))
vi.mock('@/api/client', () => ({ apiClient: { request } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 7 } }) }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

function setup() {
  const wrapper = mount(InfiniteCanvasView, { attachTo: document.body, global: {
    stubs: { AppLayout: { template: '<div><slot /></div>' } }
  } })
  const source = wrapper.get('iframe').element.contentWindow!
  const reply = vi.spyOn(source, 'postMessage')
  const send = (path: string, method: string, origin = location.origin) => {
    window.dispatchEvent(new MessageEvent('message', { origin, source, data: {
      type: 'mg-canvas-request', id: 'delete-test', path, method
    } }))
  }
  return { wrapper, reply, send }
}

describe('canvas shared API Key deletion bridge', () => {
  beforeEach(() => { vi.clearAllMocks(); deleteKey.mockResolvedValue({ message: 'deleted' }) })

  it('deletes through the main API Key flow and acknowledges only after completion', async () => {
    const { wrapper, reply, send } = setup()
    send('/keys/25', 'DELETE'); await flushPromises()
    expect(deleteKey).toHaveBeenCalledWith(25)
    expect(reply).toHaveBeenCalledWith(expect.objectContaining({ value: { message: 'deleted' }, error: undefined }), location.origin)
    expect(request).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('rejects arbitrary key operations and messages from another origin', async () => {
    const { wrapper, reply, send } = setup()
    send('/keys/25', 'DELETE', 'https://untrusted.example')
    send('/admin/keys/25', 'DELETE')
    send('/keys/25', 'PUT')
    send('/keys/9007199254740992', 'DELETE')
    await flushPromises()
    expect(deleteKey).not.toHaveBeenCalled()
    expect(request).not.toHaveBeenCalled()
    expect(reply.mock.calls).toHaveLength(3)
    wrapper.unmount()
  })

  it('returns deletion errors so the configuration can keep its channel', async () => {
    deleteKey.mockRejectedValue(new Error('Key 删除失败'))
    const { wrapper, reply, send } = setup()
    send('/keys/25', 'DELETE'); await flushPromises()
    expect(reply).toHaveBeenCalledWith(expect.objectContaining({ error: 'Key 删除失败' }), location.origin)
    wrapper.unmount()
  })
})
