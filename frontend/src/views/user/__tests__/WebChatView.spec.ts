import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import WebChatView from '../WebChatView.vue'
import { webChatAPI, type ChatSession } from '@/api/webChat'
import webChat from '@/i18n/locales/en/webChat'
import * as chatFiles from '@/utils/webChatAttachments'

vi.mock('@/api/webChat', () => ({
  webChatAPI: { keys: vi.fn(), models: vi.fn(), sessions: vi.fn(), session: vi.fn(), send: vi.fn(), remove: vi.fn(), attachment: vi.fn() }
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

async function pickFiles(...files: File[]) {
  const input = control('file-input')
  Object.defineProperty(input.element, 'files', { configurable: true, value: files })
  await input.trigger('change')
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

  it('previews, removes and reselects a file, then sends only raw data without a text message', async () => {
    const doc = new File(['Invoice 32'], 'note.md', { type: 'text/markdown' })
    const session = makeSession()
    session.messages[0] = { role: 'user', content: '', attachments: [
      { id: 'd936221b-ec52-408c-878f-e7099a5b68b3', name: doc.name, mime_type: doc.type, size: doc.size }
    ] }
    const reply = delayed<ChatSession>()
    api.send.mockReturnValue(reply.promise)
    await render()
    await pickFiles(doc)
    await vi.waitFor(() => expect(control('draft-attachments').text()).toContain('Invoice 32'))
    await control('remove-attachment').trigger('click')
    expect(control('send').attributes('disabled')).toBeDefined()
    await pickFiles(doc)
    await vi.waitFor(() => expect(control('send').attributes('disabled')).toBeUndefined())
    await view.get('form').trigger('submit')
    expect(api.send.mock.calls[0][0]).toEqual({
      key_id: k1.id, model: model.id, content: '',
      attachments: [{ name: doc.name, mime_type: doc.type, data: 'SW52b2ljZSAzMg==' }]
    })
    expect(view.get('.chat-timeline').text()).toContain('note.md')
    reply.resolve(session)
    await flushPromises()
    expect(control('message-attachments').get('button').attributes('aria-label')).toBe('Download note.md')
    expect(view.text()).not.toContain('Invoice 32')
    expect(localStorage.length + sessionStorage.length).toBe(0)
  })

  it('rejects an invalid selection without dropping valid files and explains an oversized upstream request', async () => {
    api.send.mockRejectedValue({ status: 413, reason: 'WEBCHAT_PAYLOAD_TOO_LARGE' })
    await render()
    await pickFiles(new File(['one'], 'keep.txt'))
    await vi.waitFor(() => expect(control('draft-attachments').text()).toContain('keep.txt'))
    await pickFiles(new File(['two'], 'two.txt'), new File(['binary'], 'program.exe'))
    expect(view.get('[role="alert"]').text()).toContain('supported text file')
    expect(control('draft-attachments').text()).not.toContain('two.txt')
    await sendMessage('read my file')
    expect(view.get('[role="alert"]').text()).toContain('Reduce the attachments')
    expect(control('draft-attachments').text()).toContain('keep.txt')
    expect(api.send.mock.calls[0][0].attachments).toHaveLength(1)
  })

  it('restores attachments for a retry, then discards them when the key is revoked', async () => {
    api.send.mockRejectedValueOnce({ status: 503, reason: 'WEBCHAT_ATTACHMENT_BUSY' })
      .mockRejectedValueOnce({ status: 403, reason: 'WEBCHAT_KEY_UNAVAILABLE' })
    await render()
    await pickFiles(new File(['one'], 'test.txt'))
    await vi.waitFor(() => expect(control('draft-attachments').text()).toContain('test.txt'))
    await sendMessage('review it')
    expect(view.get('[role="alert"]').text()).toContain('processing is busy')
    expect(control('draft-attachments').text()).toContain('test.txt')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(api.send.mock.calls[1][0]).toEqual(api.send.mock.calls[0][0])
    expect(view.find('[data-testid="chat-draft-attachments"]').exists()).toBe(false)
    expect(control('attach').attributes('disabled')).toBeDefined()
  })

  it('cancels a pending file read on key switch and clears a completed draft on opening a new chat', async () => {
    api.keys.mockResolvedValue([k1, k2])
    const late = delayed<chatFiles.DraftChatFile>()
    const reader = vi.spyOn(chatFiles, 'readChatFile').mockReturnValueOnce(late.promise)
    try {
      await render()
      await control('key').setValue(String(k1.id))
      await flushPromises()
      await pickFiles(new File(['a'], 'private.txt'))
      expect(control('send').attributes('disabled')).toBeDefined()
      await control('key').setValue(String(k2.id))
      expect(reader.mock.calls[0][1].aborted).toBe(true)
      late.resolve({ name: 'private.txt', mime_type: '', data: 'YQ==', size: 1 })
      await flushPromises()
      expect(view.text()).not.toContain('private.txt')
      await pickFiles(new File(['a'], 'second.txt'))
      await vi.waitFor(() => expect(control('draft-attachments').text()).toContain('second.txt'))
      await view.get('.chat-new').trigger('click')
      expect(control('send').attributes('disabled')).toBeDefined()
      expect(view.text()).not.toContain('second.txt')
    } finally { reader.mockRestore() }
  })

  it('fetches an image original using its conversation and key and revokes the preview on close', async () => {
    const session = makeSession()
    const image = { id: '043edfd7-1b9c-45df-a7ce-dd7e4cbaa8c8', name: 'drawing.png', mime_type: 'image/png', size: 99 }
    session.messages[0].attachments = [image]
    api.sessions.mockResolvedValue([session])
    api.session.mockResolvedValue(session)
    api.attachment.mockResolvedValue(new Blob(['test']))
    const create = vi.fn(() => 'blob:preview')
    const revoke = vi.fn()
    const urlClass = class extends URL {
      static createObjectURL = create
      static revokeObjectURL = revoke
    }
    vi.stubGlobal('URL', urlClass)
    try {
      await render()
      await view.get('.history-list button').trigger('click')
      await flushPromises()
      expect(api.attachment).not.toHaveBeenCalled()
      await control('message-attachments').get('button').trigger('click')
      await flushPromises()
      expect(api.attachment).toHaveBeenCalledWith(k1.id, session.id, image.id, expect.any(AbortSignal))
      expect(view.get('.original-image').attributes('src')).toBe('blob:preview')
      expect(view.get('a[download]').attributes('download')).toBe(image.name)
      await view.get('.chat-new').trigger('click')
      expect(revoke).toHaveBeenCalledWith('blob:preview')
      expect(view.find('.original-image').exists()).toBe(false)
    } finally { vi.unstubAllGlobals() }
  })

  it('enforces the UTF-8 byte limit without calling the provider', async () => {
    await render()
    await sendMessage('字'.repeat(11000))
    expect(view.get('#chat-length-error').exists()).toBe(true)
    expect(api.send).not.toHaveBeenCalled()
    expect(control('send').attributes('disabled')).toBeDefined()
  })
})
