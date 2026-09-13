import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const layoutSource = readFileSync(resolve(dir, '../AppLayout.vue'), 'utf8')
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const styleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')
const logoSource = readFileSync(resolve(dir, '../../../../public/model-gate-mg-luxury.svg'), 'utf8')

describe('user console luxury theme contract', () => {
  it('scopes the luxury theme to non-admin routes', () => {
    expect(layoutSource).toContain("!route.path.startsWith('/admin')")
    expect(layoutSource).toContain("'user-console-shell': isUserConsole")
  })

  it('uses dedicated expanded and collapsed logo dimensions', () => {
    expect(sidebarSource).toContain('sidebar-logo-user-expanded')
    expect(sidebarSource).toContain('sidebar-logo-user-collapsed')
    expect(sidebarSource).toContain('width: 3rem;')
    expect(sidebarSource).toContain('width: 2.75rem;')
  })

  it('uses the Model-Gate brand and hides the version in the user console', () => {
    expect(sidebarSource).toContain("isUserConsole.value ? 'Model-Gate'")
    expect(sidebarSource).toContain('<VersionBadge v-if="!isUserConsole"')
    expect(sidebarSource).toContain('{{ displaySiteName }}')
  })

  it('keeps the MG monogram legible at compact sidebar sizes', () => {
    expect(logoSource).toContain('x="128" y="154"')
    expect(logoSource).toContain('font-size="96"')
    expect(logoSource).toContain('letter-spacing="0"')
    expect(logoSource).not.toContain('stroke="#B79A5C"')
  })

  it('provides route-specific editorial context in the user header', () => {
    expect(headerSource).toContain('userPageEyebrow')
    expect(headerSource).toContain('OVERVIEW')
    expect(headerSource).toContain('ACCESS KEYS')
    expect(headerSource).toContain('USAGE ATELIER')
  })

  it('defines the shared pearl, ink, and champagne visual tokens', () => {
    expect(styleSource).toContain('--mg-ink-950: #121316;')
    expect(styleSource).toContain('--mg-gold-500: #c9b477;')
    expect(styleSource).toContain('--mg-pearl-50: #fbfaf7;')
    expect(styleSource).toContain('min-height: 4.5rem;')
    expect(styleSource).not.toContain('letter-spacing: 0.08em;')
    expect(styleSource).not.toContain('letter-spacing: 0.12em;')
  })
})
