import { describe, expect, it } from 'vitest'

import { resources } from './resources'

describe('mini program translations', () => {
  it('keeps every supported language aligned with English', () => {
    const languages = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'] as const
    const englishKeys = Object.keys(resources.en.translation).sort()

    expect(Object.keys(resources).sort()).toEqual([...languages].sort())
    for (const language of languages) {
      expect(Object.keys(resources[language].translation).sort()).toEqual(
        englishKeys
      )
    }
  })
})
