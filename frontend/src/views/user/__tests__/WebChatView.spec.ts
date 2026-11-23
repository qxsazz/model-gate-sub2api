import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import WebChatView from '../WebChatView.vue'
import { webChatAPI, type ChatSession } from '@/api/webChat'
import webChat from '@/i18n/locales/en/webChat'

vi.mock('@/api/webChat', () => ({
  webChatAPI: { keys: vi.fn(), models: vi.fn(), sessions: vi.fn(), session: vi.fn(), send: vi.fn(), remove: vi.fn() }
}))

const api = vi.mocked(webChatAPI)
const k1 = { id: 11, name: 'Team' }, k2 = { id: 22, name: 'Personal' }
const model = { id: 'model-a' }
const makeSession = (id = 'd751a8d3-00e8-4942-a318-9be71ca5aeeb'): ChatSession => ({
  id, title: 'Saved thread', updated_at: new Date().toISOString(), expires_at: new Date(Date.now() + 5 * 86400000).toISOString(),
  messages: [{ role: 'user', content: 'Hello' }, { role: 'assistant', content: '<img src=x onerror=alert(1)>' }]
})
function delayed<T>() {
  let resolve!: (result: T) => void
  const promise = new Promise<T>(done => { resolve = done })
  return { promise, resolve }
}
let view: VueWrapper
async function render() {
  view = mount(WebChatView, {
    global: {
      plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: { webChat, common: { cancel: 'Cancel', confirm: 'Confirm' } } } })],
      stubs: {
        AppLayout: { template: '<main><slot/></main>' },
        RouterLink: { props: ['to'], template: '<a :href="to"><slot/></a>' },
        Teleport: true, Transition: false
      }
    }
  })
  await flushPromises()
}
const control = (name: string) => view.get(`[data-testid="chat-${name}"]`)
const sendMessage = async (text: string) => {
  await control('draft').setValue(text)
  await view.get('form').trigger('submit')
  await flushPromises()
}

beforeEach(() => {
  vi.resetAllMocks()
  localStorage.clear()
  sessionStorage.clear()
  api.keys.mockResolvedValue([k1])
  api.models.mockResolvedValue([model])
  api.sessions.mockResolvedValue([])
})
afterEach(() => { view?.unmount(); vi.useRealTimers() })

describe('user web chat', () => {
  it('shows key management and five-day retention when no usable key exists', async () => {
    api.keys.mockResolvedValue([])
    await render()
    expect(view.get('a[href="/keys"]').text()).toBe('Manage API keys')
    expect(control('send').attributes('disabled')).toBeDefined()
    expect(api.models).not.toHaveBeenCalled()
    expect(api.sessions).not.toHaveBeenCalled()
    expect(view.text()).toContain('5 days after the last successful reply')
    expect(view.get('a[href="https://openwebui.com/"]').text()).toBe('Powered by Open WebUI')
  })

  it('requires an explicit choice between keys and ignores late results from the previous key', async () => {
    api.keys.mockResolvedValue([k1, k2])
    const old = delayed<{ id: string }[]>()
    api.models.mockImplementation(id => id === k1.id ? old.promise : Promise.resolve([{ id: 'personal-model' }]))
    api.sessions.mockImplementation(id => Promise.resolve(id === k1.id ? [makeSession()] : []))
    await render()
    expect(api.models).not.toHaveBeenCalled()
    await control('key').setValue(String(k1.id))
    await control('key').setValue(String(k2.id))
    await flushPromises()
    old.resolve([model])
    await flushPromises()
    expect((control('model').element as HTMLSelectElement).value).toBe('personal-model')
    expect(view.text()).not.toContain('Saved thread')
    expect(api.models.mock.calls[0][1].aborted).toBe(true)
  })

  it('sends once, renders replies as text, and continues under the same key and server session', async () => {
    const first = delayed<ChatSession>()
    const session = makeSession()
    api.send.mockReturnValueOnce(first.promise).mockResolvedValueOnce(session)
    await render()
    await control('draft').setValue('  Hello  ')
    await view.get('form').trigger('submit')
    await view.get('form').trigger('submit')
    expect(api.send).toHaveBeenCalledTimes(1)
    expect(api.send.mock.calls[0][0]).toEqual({ key_id: 11, model: 'model-a', content: 'Hello' })
    first.resolve(session)
    await flushPromises()
    expect(view.findAll('[data-testid="chat-message"]')).toHaveLength(2)
    expect(view.find('article img').exists()).toBe(false)
    expect(view.text()).toContain('<img src=x onerror=alert(1)>')
    await sendMessage('A follow-up')
    expect(api.send.mock.calls[1][0]).toEqual({ key_id: 11, model: 'model-a', content: 'A follow-up', session_id: session.id })
    expect(localStorage.length).toBe(0)
    expect(sessionStorage.length).toBe(0)
  })

  it('clears the selected conversation and disables sending on a revoked key', async () => {
    const session = makeSession()
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    api.send.mockRejectedValue({ status: 403, reason: 'WEBCHAT_KEY_UNAVAILABLE' })
    await render()
    await control('history').get('button[aria-current], .history-list button').trigger('click')
    await flushPromises()
    expect(view.findAll('article')).toHaveLength(2)
    await sendMessage('try')
    expect(view.get('[role="alert"]').text()).toContain('no longer available')
    expect(view.find('article').exists()).toBe(false)
    expect(control('send').attributes('disabled')).toBeDefined()
    expect(view.get('a[href="/keys"]').exists()).toBe(true)
  })

  it('evicts an expired conversation without silently sending its draft into a new one', async () => {
    const session = makeSession()
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    api.send.mockRejectedValue({ status: 404, reason: 'WEBCHAT_SESSION_NOT_FOUND' })
    await render()
    await view.get('.history-list button').trigger('click')
    await flushPromises()
    await sendMessage('continue')
    expect(view.get('[role="alert"]').text()).toContain('expired')
    expect(view.find('.history-list button').exists()).toBe(false)
    expect(view.find('article').exists()).toBe(false)
    await view.get('form').trigger('submit')
    expect(api.send).toHaveBeenCalledTimes(1)
  })

  it('confirms deletion for the opened session only', async () => {
    const session = makeSession()
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    api.remove.mockResolvedValue(undefined)
    await render()
    await view.get('.history-list button').trigger('click')
    await flushPromises()
    await view.get('button[aria-label="Delete conversation"]').trigger('click')
    expect(api.remove).not.toHaveBeenCalled()
    await view.get('[role="dialog"] .modal-footer button:last-child').trigger('click')
    await flushPromises()
    expect(api.remove).toHaveBeenCalledWith(11, session.id, expect.any(AbortSignal))
    expect(view.find('article').exists()).toBe(false)
  })

  it('requires a fresh read after a turn conflict and keeps the unsent draft', async () => {
    const session = { ...makeSession(), model: 'model-b' }
    api.models.mockResolvedValue([model, { id: 'model-b' }])
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    api.send.mockRejectedValueOnce({ reason: 'WEBCHAT_SESSION_CONFLICT', status: 409 }).mockResolvedValue(session)
    await render()
    await view.get('.history-list button').trigger('click')
    await flushPromises()
    expect((control('model').element as HTMLSelectElement).value).toBe('model-b')
    await sendMessage('second turn')
    expect(control('send').attributes('disabled')).toBeDefined()
    expect((control('draft').element as HTMLTextAreaElement).value).toBe('second turn')
    await view.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(api.session).toHaveBeenCalledTimes(2)
    expect(control('send').attributes('disabled')).toBeUndefined()
  })

  it('gates send when providers are unavailable while keeping key management separate', async () => {
    api.models.mockRejectedValue({ reason: 'WEBCHAT_UPSTREAM_ERROR', status: 502 })
    await render()
    expect(view.get('[role="alert"]').text()).toContain('temporarily unavailable')
    expect(control('send').attributes('disabled')).toBeDefined()
    expect(view.find('a[href="/keys"]').exists()).toBe(false)
    api.models.mockResolvedValue([model])
    await view.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(view.find('[role="alert"]').exists()).toBe(false)
    expect(control('draft').attributes('disabled')).toBeUndefined()
  })

  it('removes expired history from the open page', async () => {
    vi.useFakeTimers({ toFake: ['Date', 'setTimeout', 'clearTimeout'] })
    const session = { ...makeSession(), expires_at: new Date(Date.now() + 4800).toISOString() }
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    await render()
    await view.get('.history-list button').trigger('click')
    await flushPromises()
    expect(view.findAll('article')).toHaveLength(2)
    await vi.advanceTimersByTimeAsync(4801)
    expect(view.find('article').exists()).toBe(false)
    expect(view.find('.history-list button').exists()).toBe(false)
    expect(view.get('[role="alert"]').text()).toContain('expired')
  })

  it('enforces the UTF-8 byte limit without calling the provider', async () => {
    await render()
    await sendMessage('字'.repeat(11000))
    expect(view.get('#chat-length-error').exists()).toBe(true)
    expect(api.send).not.toHaveBeenCalled()
    expect(control('send').attributes('disabled')).toBeDefined()
  })
})
