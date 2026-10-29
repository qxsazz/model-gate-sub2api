import { afterEach, describe, expect, it } from 'vitest'

import { getLocale, i18n, loadLocaleMessages } from '..'

const originalLocale = i18n.global.locale.value

afterEach(() => {
  i18n.global.locale.value = originalLocale
})

describe('lazy-loaded locale runtime compatibility', () => {
  it.each([
    ['en', 'Save', '(3 selected)'],
    ['zh', '保存', '（已选 3 个）']
  ] as const)('renders %s messages and named parameters', async (locale, save, selected) => {
    await loadLocaleMessages(locale)
    i18n.global.locale.value = locale

    expect(getLocale()).toBe(locale)
    expect(i18n.global.t('common.save')).toBe(save)
    expect(i18n.global.t('common.selectedCount', { count: 3 })).toBe(selected)
  })

  it('keeps English plural selection after loading both locales', async () => {
    await loadLocaleMessages('zh')
    await loadLocaleMessages('en')
    i18n.global.locale.value = 'en'

    expect(i18n.global.t('announcements.newCount', { count: 1 }, 1)).toBe('1 new announcement')
    expect(i18n.global.t('announcements.newCount', { count: 3 }, 3)).toBe('3 new announcements')
  })
})
