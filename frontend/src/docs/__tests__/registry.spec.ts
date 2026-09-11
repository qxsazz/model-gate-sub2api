import { describe, expect, it } from 'vitest'

import {
  DEFAULT_DOC_LOCATION,
  docsNavigation,
  getAdjacentDocuments,
  resolveDocument,
} from '@/docs/registry'

describe('documentation registry', () => {
  it('resolves the default article when no query parameters are provided', () => {
    const result = resolveDocument()

    expect(result?.location).toEqual(DEFAULT_DOC_LOCATION)
    expect(result?.document.title).toBe('快速开始')
  })

  it('resolves a registered category and page', () => {
    const result = resolveDocument('clients', 'claude-code')

    expect(result?.document.title).toBe('Claude Code 接入')
    expect(result?.document.source).toContain('# Claude Code 接入')
  })

  it('rejects unregistered values instead of treating them as file paths', () => {
    expect(resolveDocument('tutorial', '../../secrets')).toBeNull()
    expect(resolveDocument('../private', 'quick-start')).toBeNull()
    expect(resolveDocument('tutorial', 'missing')).toBeNull()
  })

  it('keeps navigation ordering stable and exposes adjacent articles', () => {
    expect(docsNavigation.map((group) => group.slug)).toEqual([
      'tutorial',
      'clients',
      'platform',
      'help',
    ])

    const { previous, next } = getAdjacentDocuments('clients', 'claude-code')
    expect(previous?.slug).toBe('api-reference')
    expect(next?.slug).toBe('codex')
  })
})
