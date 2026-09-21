import Taro from '@tarojs/taro'
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'

import { resources } from './resources'

const LANGUAGE_STORAGE_KEY = 'miniapp:language:v1'
const supportedLanguages = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'] as const

type SupportedLanguage = (typeof supportedLanguages)[number]

let initialization: Promise<unknown> | undefined

function normalizeLanguage(value: unknown): SupportedLanguage {
  if (typeof value !== 'string') {
    return 'en'
  }

  const language = value.replace('_', '-').toLowerCase()
  if (language === 'zh-tw' || language === 'zh-hk' || language === 'zh-hant') {
    return 'zh-TW'
  }
  if (language.startsWith('zh')) {
    return 'zh'
  }

  const baseLanguage = language.split('-')[0]
  return supportedLanguages.find((candidate) => candidate === baseLanguage) ?? 'en'
}

function detectLanguage(): SupportedLanguage {
  try {
    const storedLanguage = Taro.getStorageSync(LANGUAGE_STORAGE_KEY)
    if (storedLanguage) {
      return normalizeLanguage(storedLanguage)
    }
  } catch {
    // A failed preference read should not prevent the application from starting.
  }

  try {
    return normalizeLanguage(Taro.getAppBaseInfo().language)
  } catch {
    return 'en'
  }
}

export function initializeI18n() {
  if (i18n.isInitialized) {
    return Promise.resolve(i18n)
  }

  initialization ??= i18n.use(initReactI18next).init({
    resources,
    lng: detectLanguage(),
    fallbackLng: 'en',
    interpolation: { escapeValue: false },
    returnNull: false,
  })
  return initialization
}

export async function syncTabBarLabels() {
  const labels = [
    i18n.t('nav.home'),
    i18n.t('nav.models'),
    i18n.t('nav.playground'),
    i18n.t('nav.usage'),
    i18n.t('nav.profile'),
  ]

  await Promise.allSettled(
    labels.map((text, index) => Taro.setTabBarItem({ index, text }))
  )
}

export async function changeLanguage(language: string) {
  await initializeI18n()
  const normalizedLanguage = normalizeLanguage(language)

  try {
    Taro.setStorageSync(LANGUAGE_STORAGE_KEY, normalizedLanguage)
  } catch {
    // Language switching should still work when local storage is unavailable.
  }

  await i18n.changeLanguage(normalizedLanguage)
  await syncTabBarLabels()
}

export { i18n }
