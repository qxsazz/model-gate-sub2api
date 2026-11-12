import { apiClient } from './client'

export interface ChatBinding {
  id: number
  group_name?: string
  created_at: string
}
export interface ChatBindingChallenge {
  challenge_id: string
  group_name?: string
  expires_at: string
  state: 'pending' | 'authorized' | 'completed' | 'declined' | 'expired'
  binding_mode?: 'email' | 'code'
  account_confirmation: string
  account_id: number
}
export type ChatBindingAuthorization =
  | { status: 'bound' | 'declined'; expires_at: string; code?: never }
  | { code: string; expires_at: string; status?: never }
export const listChatBindings = async () =>
  (await apiClient.get<ChatBinding[]>('/user/chat-bindings')).data
export const getChatBindingChallenge = async (id: string) =>
  (await apiClient.get<ChatBindingChallenge>(`/user/chat-bindings/challenges/${encodeURIComponent(id)}`)).data
export const authorizeChatBinding = async (id: string, accountConfirmation: string, confirm: boolean) =>
  (await apiClient.post<ChatBindingAuthorization>(`/user/chat-bindings/challenges/${encodeURIComponent(id)}/authorize`, { confirm, account_confirmation: accountConfirmation })).data
export const revokeChatBinding = async (id: number) =>
  (await apiClient.delete(`/user/chat-bindings/${id}`)).data
