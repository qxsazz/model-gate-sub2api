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
    mocks.get.mockResolvedValue({ challenge_id: 'challenge-a', identity, group_name: '账单群', state: 'pending', expires_at: new Date(Date.now() + 600000).toISOString(), account_confirmation: 'account-a-preview', account_id: 1 })
    mocks.authorize.mockResolvedValue({ code: 'ABCD-EFGH-IJKL-MNOP', expires_at: new Date(Date.now() + 600000).toISOString() })
  })
  it('shows the group name and requires explicit consent', async () => {
    const w = render(); await flushPromises()
    expect(w.text()).toContain('账单群')
    for (const value of Object.values(identity)) expect(w.text()).not.toContain(value)
    expect(mocks.authorize).not.toHaveBeenCalled()
    expect(w.get('[data-authorize]').attributes('disabled')).toBeDefined()
    await w.get('[data-consent]').setValue(true)
    await w.get('[data-authorize]').trigger('click'); await flushPromises()
    expect(mocks.authorize).toHaveBeenCalledWith('challenge-a', 'account-a-preview', true)
    expect(w.get('[data-code]').text()).toBe('/绑定 ABCD-EFGH-IJKL-MNOP')
    auth.user.id = 2; await flushPromises()
    expect(w.find('[data-code]').exists()).toBe(false)
    expect(w.find('[data-authorize]').exists()).toBe(false)
    w.unmount()
  })
  it('prevents authorization of expired requests', async () => {
    mocks.get.mockResolvedValue({ identity, state: 'expired', expires_at: new Date(Date.now() - 1000).toISOString(), account_id: 1 })
    const w = render(); await flushPromises()
    expect(w.text()).toContain('此绑定请求已失效')
    expect(w.find('[data-authorize]').exists()).toBe(false)
    expect(mocks.authorize).not.toHaveBeenCalled(); w.unmount()
  })
  it('completes an email binding on the website without displaying a code', async () => {
    const preview = {
      challenge_id: 'challenge-a', identity, group_name: '账单群', state: 'pending', binding_mode: 'email',
      expires_at: new Date(Date.now() + 600000).toISOString(),
      account_id: 1, account_confirmation: 'account-a-preview',
    }
    mocks.get.mockResolvedValue(preview)
    mocks.authorize.mockResolvedValue({ status: 'bound', expires_at: preview.expires_at })
    mocks.list.mockResolvedValueOnce([]).mockResolvedValue([{ id: 19, identity, group_name: '账单群' }])
    const w = render(); await flushPromises()
    expect(w.get('[data-authorize]').text()).toBe('确认绑定')
    expect(w.get('[data-authorize]').attributes('disabled')).toBeDefined()
    await w.get('[data-consent]').setValue(true)
    await w.get('[data-authorize]').trigger('click'); await flushPromises()
    expect(mocks.authorize).toHaveBeenCalledWith('challenge-a', 'account-a-preview', true)
    expect(w.text()).toContain('绑定已完成，可以回到原群发送 /签到')
    expect(w.find('[data-code]').exists()).toBe(false)
    expect(w.find('[data-revoke="19"]').exists()).toBe(true)
    w.unmount()
  })
  it('records an explicit decline without creating a binding', async () => {
    mocks.get.mockResolvedValue({
      challenge_id: 'challenge-a', identity, group_name: '账单群', state: 'pending', binding_mode: 'email',
      expires_at: new Date(Date.now() + 600000).toISOString(),
      account_id: 1, account_confirmation: 'account-a-preview',
    })
    mocks.authorize.mockResolvedValue({ status: 'declined' })
    const w = render(); await flushPromises()
    await w.get('[data-decline]').trigger('click'); await flushPromises()
    expect(mocks.authorize).toHaveBeenCalledWith('challenge-a', 'account-a-preview', false)
    expect(w.text()).toContain('已取消这次绑定请求')
    expect(w.find('[data-authorize]').exists()).toBe(false)
    w.unmount()
  })
  it('does not unlink until confirmation and uses the owned binding ID', async () => {
    route.params.challengeId = ''
    mocks.list.mockResolvedValue([{ id: 17, identity, group_name: '账单群' }])
    const w = render(); await flushPromises()
    expect(w.text()).toContain('账单群')
    for (const value of Object.values(identity)) expect(w.text()).not.toContain(value)
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
  it('rejects a preview issued for a different account even before authorization', async () => {
    mocks.get.mockResolvedValue({ identity, state: 'pending', expires_at: new Date(Date.now() + 600000).toISOString(), account_id: 2, account_confirmation: 'account-b-preview' })
    const w = render(); await flushPromises()
    expect(w.get('[role="alert"]').text()).toContain('登录账号已变化')
    expect(w.find('[data-authorize]').exists()).toBe(false)
    expect(mocks.authorize).not.toHaveBeenCalled(); w.unmount()
  })
})
