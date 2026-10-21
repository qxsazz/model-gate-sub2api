import { describe, expect, it } from 'vitest'
import type { Group } from '@/types'
import { supportsKeyPurpose, parseKeyPurpose } from './keyPurpose'
const group = (caps: Group['key_capabilities'], status: Group['status'] = 'active') => ({ status, key_capabilities: caps }) as Group
describe('Key creation purposes', () => {
  it('keeps chat-only Grok out of image and video creation', () => {
    const chat = group({ image: false, video: false, text: true })
    expect(supportsKeyPurpose(chat, 'image')).toBe(false)
    expect(supportsKeyPurpose(chat, 'video')).toBe(false)
    expect(supportsKeyPurpose(chat, 'text')).toBe(true)
  })
  it('filters separate model capabilities and inactive groups', () => {
    const image = group({ image: true, video: false, text: false })
    expect(supportsKeyPurpose(image, 'image')).toBe(true)
    expect(supportsKeyPurpose(image, 'text')).toBe(false)
    expect(supportsKeyPurpose(group({ image: true, video: true, text: true }, 'inactive'), 'all')).toBe(false)
    expect(supportsKeyPurpose(group(undefined), 'image')).toBe(false)
    expect(parseKeyPurpose('untrusted')).toBe('all')
  })
})
