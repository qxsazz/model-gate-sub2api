import { describe, expect, it } from 'vitest'
import { isOfficialGeminiBaseUrl } from '../geminiExplicitCache'

describe('Gemini account quota provider classification', () => {
  it.each(['', undefined, null, 'https://generativelanguage.googleapis.com', 'https://generativelanguage.googleapis.com/'])(
    'recognizes official defaults: %s', (value) => {
      expect(isOfficialGeminiBaseUrl(value)).toBe(true)
    }
  )

  it.each(['https://relay.example.com', 'https://generativelanguage.googleapis.com/relay', 'https://generativelanguage.googleapis.com.evil.example', 'http://generativelanguage.googleapis.com', 'invalid'])(
    'keeps third party relays out of simulated Google quotas: %s', (value) => {
      expect(isOfficialGeminiBaseUrl(value)).toBe(false)
    }
  )
})
