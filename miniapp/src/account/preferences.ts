export const supportedLanguages = [
  'en',
  'zh',
  'zh-TW',
  'fr',
  'ja',
  'ru',
  'vi',
] as const

export type SupportedLanguage = (typeof supportedLanguages)[number]

export function parseAccountLanguage(setting: string): SupportedLanguage | null {
  try {
    const value = JSON.parse(setting) as { language?: unknown }
    return supportedLanguages.includes(value.language as SupportedLanguage)
      ? (value.language as SupportedLanguage)
      : null
  } catch {
    return null
  }
}

export function isAccountDeletionConfirmed(input: string, username: string) {
  return username !== '' && input === username
}
