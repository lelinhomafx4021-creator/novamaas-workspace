import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { compactConversation, loadConversation, saveConversation } from './storage'

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn(),
    removeStorageSync: vi.fn(),
    setStorageSync: vi.fn(),
  },
}))

describe('playground conversation storage', () => {
  beforeEach(() => vi.clearAllMocks())

  it('rejects an unknown storage version', () => {
    vi.mocked(Taro.getStorageSync).mockReturnValue({ version: 2, messages: [] })
    expect(loadConversation()).toEqual({ group: '', messages: [], model: '', version: 1 })
  })

  it('migrates an unversioned draft and strips unknown fields', () => {
    vi.mocked(Taro.getStorageSync).mockReturnValue({
      group: 'default',
      messages: [
        { accessToken: 'must-not-survive', content: 'hello', createdAt: 1, id: '1', role: 'user' },
      ],
      model: 'model',
    })
    const migrated = loadConversation()
    expect(migrated.version).toBe(1)
    expect(JSON.stringify(migrated)).not.toContain('accessToken')
  })

  it('caps message count and content without storing credentials', () => {
    const messages = Array.from({ length: 50 }, (_, index) => ({
      content: 'x'.repeat(2_000),
      createdAt: index,
      id: String(index),
      role: index % 2 ? ('assistant' as const) : ('user' as const),
    }))
    const compacted = compactConversation({ group: 'default', messages, model: 'model', version: 1 })
    expect(compacted.messages.length).toBeLessThanOrEqual(40)
    expect(compacted.messages.reduce((sum, item) => sum + item.content.length, 0)).toBeLessThanOrEqual(60_000)

    saveConversation(compacted)
    const stored = vi.mocked(Taro.setStorageSync).mock.calls[0]?.[1]
    expect(JSON.stringify(stored)).not.toContain('accessToken')
  })
})
