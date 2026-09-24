import Taro from '@tarojs/taro'

export type ChatRole = 'assistant' | 'user'

export interface ChatMessage {
  content: string
  createdAt: number
  id: string
  role: ChatRole
}

export interface ConversationDraft {
  group: string
  messages: ChatMessage[]
  model: string
  version: 1
}

const STORAGE_KEY = 'miniapp:playground:v1'
const MAX_MESSAGES = 40
const MAX_CHARACTERS = 60_000

function isMessage(value: unknown): value is ChatMessage {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<ChatMessage>
  return (
    (item.role === 'user' || item.role === 'assistant') &&
    typeof item.id === 'string' &&
    typeof item.content === 'string' &&
    typeof item.createdAt === 'number'
  )
}

export function compactConversation(draft: ConversationDraft): ConversationDraft {
  const reversed: ChatMessage[] = []
  let characters = 0
  for (const message of [...draft.messages].reverse()) {
    if (reversed.length >= MAX_MESSAGES) break
    const remaining = MAX_CHARACTERS - characters
    if (remaining <= 0) break
    const content = message.content.slice(-remaining)
    reversed.push({
      content,
      createdAt: message.createdAt,
      id: message.id,
      role: message.role,
    })
    characters += content.length
  }
  return { ...draft, messages: reversed.reverse(), version: 1 }
}

export function loadConversation(): ConversationDraft {
  const empty: ConversationDraft = { group: '', messages: [], model: '', version: 1 }
  try {
    const value = Taro.getStorageSync(STORAGE_KEY) as Partial<ConversationDraft>
    if (
      !value ||
      (value.version !== 1 && value.version !== undefined) ||
      typeof value.group !== 'string' ||
      typeof value.model !== 'string' ||
      !Array.isArray(value.messages) ||
      !value.messages.every(isMessage)
    ) {
      return empty
    }
    return compactConversation(value as ConversationDraft)
  } catch {
    return empty
  }
}

export function saveConversation(draft: ConversationDraft) {
  const compacted = compactConversation(draft)
  try {
    Taro.setStorageSync(STORAGE_KEY, compacted)
  } catch {
    // The active conversation remains available in memory.
  }
  return compacted
}

export function clearConversation() {
  try {
    Taro.removeStorageSync(STORAGE_KEY)
  } catch {
    // Clearing in-memory state is sufficient when storage is unavailable.
  }
}
