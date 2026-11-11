import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import ChatBindingsView from '../ChatBindingsView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), get: vi.fn(), authorize: vi.fn(), revoke: vi.fn(), refresh: vi.fn() }))
const route = reactive({ params: { challengeId: 'challenge-a' } })
const auth = reactive({ user: { id: 1, email: 'current@example.com', username: '当前账号' }, refreshUser: mocks.refresh })
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/api/chatBindings', () => ({ listChatBindings: mocks.list, getChatBindingChallenge: mocks.get, authorizeChatBinding: mocks.authorize, revokeChatBinding: mocks.revoke }))
const identity = { platform: 'qq_official', bot_id: 'test-bot', group_id: 'test-group', sender_id: 'member-openid' }
function render() { return mount(ChatBindingsView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, RouterLink: { template: '<a><slot /></a>' }, ConfirmDialog: true } } }) }

describe('群聊绑定双向确认', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    route.params.challengeId = 'challenge-a'
    auth.user.id = 1
    mocks.refresh.mockResolvedValue(undefined)
    mocks.list.mockResolvedValue([])
    mocks.get.mockResolvedValue({ challenge_id: 'challenge-a', identity, state: 'pending', expires_at: new Date(Date.now() + 600000).toISOString() })
    mocks.authorize.mockResolvedValue({ code: 'ABCD-EFGH-IJKL-MNOP', expires_at: new Date(Date.now() + 600000).toISOString() })
  })
  it('shows the current account and exact source, and requires explicit consent', async () => {
    const w = render(); await flushPromises()
    expect(w.text()).toContain('current@example.com')
    for (const value of Object.values(identity).slice(1)) expect(w.text()).toContain(value)
    expect(mocks.authorize).not.toHaveBeenCalled()
    expect(w.get('[data-authorize]').attributes('disabled')).toBeDefined()
    await w.get('[data-consent]').setValue(true)
    await w.get('[data-authorize]').trigger('click'); await flushPromises()
    expect(mocks.authorize).toHaveBeenCalledWith('challenge-a')
    expect(w.get('[data-code]').text()).toBe('/绑定 ABCD-EFGH-IJKL-MNOP')
    auth.user.id = 2; await flushPromises()
    expect(w.find('[data-code]').exists()).toBe(false)
    expect(w.get('[data-authorize]').attributes('disabled')).toBeDefined()
    w.unmount()
  })
  it('prevents authorization of expired requests', async () => {
    mocks.get.mockResolvedValue({ identity, state: 'expired', expires_at: new Date(Date.now() - 1000).toISOString() })
    const w = render(); await flushPromises()
    expect(w.text()).toContain('此绑定请求已失效')
    expect(w.find('[data-authorize]').exists()).toBe(false)
    expect(mocks.authorize).not.toHaveBeenCalled(); w.unmount()
  })
  it('does not unlink until confirmation and uses the owned binding ID', async () => {
    route.params.challengeId = ''
    mocks.list.mockResolvedValue([{ id: 17, identity }])
    const w = render(); await flushPromises()
    await w.get('[data-revoke="17"]').trigger('click')
    expect(mocks.revoke).not.toHaveBeenCalled()
    w.getComponent({ name: 'ConfirmDialog' }).vm.$emit('confirm'); await flushPromises()
    expect(mocks.revoke).toHaveBeenCalledWith(17); w.unmount()
  })
  it('fails closed when account refresh fails', async () => {
    mocks.refresh.mockRejectedValue(new Error('expired session'))
    const w = render(); await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('重新登录')
    expect(w.find('[data-authorize]').exists()).toBe(false); w.unmount()
  })
})
