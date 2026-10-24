const OFFICIAL_GEMINI_API_HOST = 'generativelanguage.googleapis.com'

/** Gemini API Key 账号的 base_url 是否为官方地址；留空视为官方地址。 */
export function isOfficialGeminiBaseUrl(raw: string | null | undefined): boolean {
  const value = (raw ?? '').trim()
  if (!value) return true
  try {
    const url = new URL(value)
    const path = url.pathname.replace(/\/+$/, '')
    return (
      url.protocol === 'https:' &&
      url.hostname.toLowerCase() === OFFICIAL_GEMINI_API_HOST &&
      url.port === '' &&
      path === '' &&
      url.search === ''
    )
  } catch {
    return false
  }
}
