import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const directory = dirname(fileURLToPath(import.meta.url))
const authLayoutSource = readFileSync(resolve(directory, '../AuthLayout.vue'), 'utf8')

describe('AuthLayout luxury theme', () => {
  it('uses the shared Model-Gate authentication surface and uploaded logo only', () => {
    expect(authLayoutSource).toContain('auth-luxury-shell')
    expect(authLayoutSource).toContain('auth-luxury-card')
    expect(authLayoutSource).toContain('auth-brand-logo')
    expect(authLayoutSource).toContain('auth-font-display')
    expect(authLayoutSource).toContain('MODEL-GATE AUTHORIZED ACCESS')
    expect(authLayoutSource).toContain('AI API GATEWAY')
    expect(authLayoutSource).toContain('MODEL-GATE 授权平台')
    expect(authLayoutSource).not.toContain("|| '/logo.svg'")
    expect(authLayoutSource).not.toContain("|| '/model-gate-mg-luxury.svg'")
    expect(authLayoutSource).toContain('siteLogo')
  })
})
