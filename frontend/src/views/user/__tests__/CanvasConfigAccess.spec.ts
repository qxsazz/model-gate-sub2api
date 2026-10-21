import { describe, expect, it } from 'vitest'
import { accessibleConfigTab, canAccessConfigTab } from '../../../../../canvas/web/src/lib/config-access'

describe('canvas administrator configuration access', () => {
  it.each(['local-proxy', 'webdav', 'prompt-sources'])('blocks ordinary users from %s, including direct initial tabs', key => {
    expect(canAccessConfigTab(key, false)).toBe(false)
    expect(accessibleConfigTab(key, false)).toBe('channels')
    expect(canAccessConfigTab(key, true)).toBe(true)
    expect(accessibleConfigTab(key, true)).toBe(key)
  })
  it.each(['channels', 'preferences', 'local-storage'])('keeps %s available to ordinary users', key => {
    expect(canAccessConfigTab(key, false)).toBe(true)
    expect(accessibleConfigTab(key, false)).toBe(key)
  })
})
