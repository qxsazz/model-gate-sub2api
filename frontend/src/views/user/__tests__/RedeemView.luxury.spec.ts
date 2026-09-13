import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const redeemSource = readFileSync(resolve(dir, '../RedeemView.vue'), 'utf8')

describe('RedeemView luxury surface', () => {
  it('uses the black-diamond redemption surface instead of the legacy primary gradient', () => {
    expect(redeemSource).toContain('data-testid="redeem-luxury-page"')
    expect(redeemSource).toContain('redeem-diamond-card')
    expect(redeemSource).toContain('redeem-code-input')
    expect(redeemSource).toContain('redeem-submit-button')
    expect(redeemSource).toContain('--redeem-diamond')
    expect(redeemSource).not.toContain('bg-gradient-to-br from-primary-500 to-primary-600')
  })
})
