import type { Group } from '@/types'
export type KeyPurpose = 'image' | 'video' | 'text'
export type KeyPurposeChoice = KeyPurpose | 'all'
export const KEY_PURPOSES: Array<{ value: KeyPurposeChoice; label: string }> = [
  { value: 'all', label: '通用' }, { value: 'image', label: '生图' },
  { value: 'video', label: '视频' }, { value: 'text', label: '文本' }
]
export function supportsKeyPurpose(group: Group, purpose: KeyPurposeChoice): boolean {
  return group.status === 'active' && (purpose === 'all' || group.key_capabilities?.[purpose] === true)
}
export function parseKeyPurpose(value: unknown): KeyPurposeChoice {
  return value === 'image' || value === 'video' || value === 'text' ? value : 'all'
}
