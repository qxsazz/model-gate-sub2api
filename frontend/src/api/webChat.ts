import { apiClient } from './client'

export interface ChatKey { id: number; name: string }
export interface ChatModel { id: string }
export interface ChatSessionSummary {
  id: string
  title: string
  model?: string
  updated_at: string
  expires_at: string
}
export interface ChatAttachment { id: string; name: string; mime_type: string; size: number }
export interface ChatAttachmentUpload { name: string; mime_type: string; data: string }
export interface ChatSession extends ChatSessionSummary {
  messages: { role: 'user' | 'assistant'; content: string; attachments?: ChatAttachment[] }[]
}
interface SendMessage {
  key_id: number
  model: string
  content: string
  session_id?: string
  attachments?: ChatAttachmentUpload[]
}

const root = '/webchat'
const sessionPath = (id: string) => root + '/sessions/' + encodeURIComponent(id)

export const webChatAPI = {
  async keys(signal: AbortSignal) {
    return (await apiClient.get<{ items: ChatKey[] }>(root + '/keys', { signal })).data.items
  },
  async models(keyId: number, signal: AbortSignal) {
    return (await apiClient.get<{ items: ChatModel[] }>(root + '/models', { params: { key_id: keyId }, signal })).data.items
  },
  async sessions(keyId: number, signal: AbortSignal) {
    return (await apiClient.get<{ items: ChatSessionSummary[] }>(root + '/sessions', { params: { key_id: keyId }, signal })).data.items
  },
  async session(keyId: number, id: string, signal: AbortSignal) {
    return (await apiClient.get<ChatSession>(sessionPath(id), { params: { key_id: keyId }, signal })).data
  },
  async attachment(keyId: number, sessionId: string, id: string, signal: AbortSignal) {
    const path = sessionPath(sessionId) + '/attachments/' + encodeURIComponent(id)
    return (await apiClient.get<Blob>(path, { params: { key_id: keyId }, responseType: 'blob', signal })).data
  },
  async remove(keyId: number, id: string, signal: AbortSignal) {
    await apiClient.delete(sessionPath(id), { params: { key_id: keyId }, signal })
  },
  async send(message: SendMessage, signal: AbortSignal) {
    return (await apiClient.post<ChatSession>(root + '/messages', message, { signal, timeout: 15 * 60 * 1000 })).data
  }
}
