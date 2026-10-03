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
      'membership',
      'achievements',
      'tools',
      'about',
      'troubleshooting',
    ])

    const { previous, next } = getAdjacentDocuments('clients', 'claude-code')
    expect(previous?.slug).toBe('sdk')
    expect(next?.slug).toBe('codex')
  })

  it('documents actual VIP rules and labels unreleased activities as upcoming', () => {
    const vip = resolveDocument('membership', 'vip')
    expect(vip?.document.source).toContain('0.40 − 0.03 = 0.37')
    expect(vip?.document.source).toContain('0.075')
    expect(vip?.document.source).toContain('22.62%')
    expect(resolveDocument('achievements', 'check-in')?.document.source).toContain('$1.00')
    const rewards = resolveDocument('membership', 'rewards')?.document.source || ''
    expect(rewards).toContain('2%')
    for (const amount of ['$2.00', '$6.00', '$12.00', '$30.00', '$60.00']) expect(rewards).toContain(amount)
    expect(rewards).toContain('110')
    const activities = docsNavigation.find(group => group.slug === 'achievements')!
    expect(activities.articles.length).toBeGreaterThanOrEqual(4)
    for (const entry of activities.articles) {
      expect(entry).toMatchObject({ status: 'upcoming' })
      expect(entry.source).toContain('敬请期待')
    }
    expect(resolveDocument('tools', 'recommended')?.document.source).toContain('CC Switch')
  })

  it('keeps all document links inside the registered content map', () => {
    for (const group of docsNavigation) for (const entry of group.articles) {
      for (const match of entry.source.matchAll(/\/docs\?cat=([a-z-]+)&page=([a-z-]+)/g)) {
        expect(resolveDocument(match[1], match[2]), `${group.slug}/${entry.slug}: ${match[0]}`).not.toBeNull()
      }
    }
  })
  it('preserves legacy help URLs while separating service commitments and troubleshooting', () => {
    expect(resolveDocument('help', 'security')?.location).toEqual({ category: 'about', page: 'security' })
    expect(resolveDocument('help', 'faq')?.location).toEqual({ category: 'troubleshooting', page: 'faq' })
    expect(resolveDocument('about', 'security')?.document.source).toContain('服务底线')
    const errors = resolveDocument('troubleshooting', 'http-errors')?.document.source || ''
    for (const code of ['400', '401', '403', '404', '413', '429', '499', '500', '502', '503', '504']) expect(errors).toContain(code)
    expect(errors).toContain('API_KEY_DISABLED')
    expect(errors).toContain('INSUFFICIENT_BALANCE')
    expect(resolveDocument('troubleshooting', 'records')?.document.source).toContain('错误记录')
  })
})
