import { apiClient } from './client'

export interface ChatIdentity {
  platform: 'qq_official' | 'wechat_native'
  bot_id: string
  group_id: string
  sender_id: string
}
export interface ChatBinding {
  id: number
  identity: ChatIdentity
  created_at: string
}
export interface ChatBindingChallenge {
  challenge_id: string
  identity: ChatIdentity
  expires_at: string
  state: 'pending' | 'authorized' | 'completed' | 'expired'
  binding_mode?: 'email' | 'code'
  account_confirmation: string
  account_id: number
}
export type ChatBindingAuthorization =
  | { status: 'bound'; expires_at: string; code?: never }
  | { code: string; expires_at: string; status?: never }
export const listChatBindings = async () =>
  (await apiClient.get<ChatBinding[]>('/user/chat-bindings')).data
export const getChatBindingChallenge = async (id: string) =>
  (await apiClient.get<ChatBindingChallenge>(`/user/chat-bindings/challenges/${encodeURIComponent(id)}`)).data
export const authorizeChatBinding = async (id: string, accountConfirmation: string) =>
  (await apiClient.post<ChatBindingAuthorization>(`/user/chat-bindings/challenges/${encodeURIComponent(id)}/authorize`, { confirm: true, account_confirmation: accountConfirmation })).data
export const revokeChatBinding = async (id: number) =>
  (await apiClient.delete(`/user/chat-bindings/${id}`)).data
