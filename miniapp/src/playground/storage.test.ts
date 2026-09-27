import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  compactConversationStore,
  conversationHistory,
  conversationTitle,
  createConversation,
  loadConversationStore,
  saveConversationStore,
} from './storage'

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn(),
    removeStorageSync: vi.fn(),
    setStorageSync: vi.fn(),
  },
}))

describe('playground conversation storage', () => {
  beforeEach(() => vi.clearAllMocks())

  it('migrates a v1 conversation and strips unknown fields from every message', () => {
    vi.mocked(Taro.getStorageSync).mockImplementation((key) => key === 'miniapp:playground:v1' ? {
      group: 'default',
      messages: [{ accessToken: 'must-not-survive', content: 'hello', createdAt: 1, id: '1', role: 'user' }],
      model: 'model',
    } : undefined)
    const migrated = loadConversationStore()
    expect(migrated.version).toBe(2)
    expect(migrated.conversations).toHaveLength(1)
    expect(migrated.conversations[0].messages[0].content).toBe('hello')
    expect(JSON.stringify(migrated)).not.toContain('accessToken')
    expect(saveConversationStore(migrated)).toBe(true)
    expect(vi.mocked(Taro.setStorageSync)).toHaveBeenCalledWith('miniapp:playground:v2', migrated)
    expect(vi.mocked(Taro.removeStorageSync)).toHaveBeenCalledWith('miniapp:playground:v1')
  })

  it('retains multiple sessions and the selected session after reload', () => {
    const first = createConversation('default', 'model-a')
    first.messages = [{ content: 'First question', createdAt: 1, id: '1', role: 'user' }]
    const second = createConversation('default', 'model-b')
    second.messages = [{ content: 'Second question', createdAt: 2, id: '2', role: 'user' }]
    const store = { activeId: second.id, conversations: [first, second], version: 2 as const }
    vi.mocked(Taro.getStorageSync).mockImplementation((key) => key === 'miniapp:playground:v2' ? store : undefined)

    expect(loadConversationStore()).toEqual(store)
    expect(conversationTitle(first)).toBe('First question')
    expect(conversationTitle(second)).toBe('Second question')
  })

  it('shows completed chats newest first without filling history with an empty draft', () => {
    const older = createConversation('default', 'model')
    older.updatedAt = 10
    older.messages = [{ content: 'Older question', createdAt: 10, id: 'older', role: 'user' }]
    const draft = createConversation('default', 'model')
    draft.updatedAt = 30
    const newer = createConversation('default', 'model')
    newer.updatedAt = 20
    newer.messages = [{ content: 'Newer question', createdAt: 20, id: 'newer', role: 'user' }]

    expect(conversationHistory([older, draft, newer]).map((item) => item.id)).toEqual([newer.id, older.id])
    expect([older, draft, newer]).toHaveLength(3)
  })

  it('keeps a saved image reference with its question across local history reloads', () => {
    const conversation = createConversation('default', 'vision')
    conversation.messages = [{ content: 'Describe this', createdAt: 1, id: 'image-1', imagePath: 'wxfile://saved/photo.jpg', role: 'user' }]
    const store = { activeId: conversation.id, conversations: [conversation], version: 2 as const }
    vi.mocked(Taro.getStorageSync).mockImplementation((key) => key === 'miniapp:playground:v2' ? store : undefined)

    expect(loadConversationStore().conversations[0].messages[0].imagePath).toBe('wxfile://saved/photo.jpg')
    expect(saveConversationStore(store)).toBe(true)
    expect(vi.mocked(Taro.setStorageSync)).toHaveBeenCalledWith('miniapp:playground:v2', expect.objectContaining({
      conversations: [expect.objectContaining({ messages: [expect.objectContaining({ imagePath: 'wxfile://saved/photo.jpg' })] })],
    }))
  })

  it('caps each session independently without storing credentials', () => {
    const first = createConversation()
    first.messages = Array.from({ length: 50 }, (_, index) => ({
      content: 'x'.repeat(2_000),
      createdAt: index,
      id: String(index),
      role: index % 2 ? ('assistant' as const) : ('user' as const),
    }))
    const second = createConversation()
    second.messages = [{ content: 'still here', createdAt: 100, id: 'second', role: 'user' }]
    const compacted = compactConversationStore({ activeId: first.id, conversations: [first, second], version: 2 })
    expect(compacted.conversations[0].messages.length).toBeLessThanOrEqual(40)
    expect(compacted.conversations[0].messages.reduce((sum, item) => sum + item.content.length, 0)).toBeLessThanOrEqual(60_000)
    expect(compacted.conversations[1].messages[0].content).toBe('still here')
  })

  it('does not remove the old copy when the new storage write fails', () => {
    vi.mocked(Taro.setStorageSync).mockImplementation(() => { throw new Error('storage full') })
    const conversation = createConversation()
    expect(saveConversationStore({ activeId: conversation.id, conversations: [conversation], version: 2 })).toBe(false)
    expect(vi.mocked(Taro.removeStorageSync)).not.toHaveBeenCalled()
  })
})
