import Taro from '@tarojs/taro'

export type ChatRole = 'assistant' | 'user'

export interface ChatMessage {
  content: string
  createdAt: number
  id: string
  imagePath?: string
  role: ChatRole
}

export interface ConversationDraft {
  group: string
  messages: ChatMessage[]
  model: string
  version: 1
}

export interface ChatConversation {
  id: string
  group: string
  messages: ChatMessage[]
  model: string
  updatedAt: number
}

export interface ConversationStore {
  activeId: string
  conversations: ChatConversation[]
  version: 2
}

const LEGACY_KEY = 'miniapp:playground:v1'
const STORAGE_KEY = 'miniapp:playground:v2'
const MAX_MESSAGES = 40
const MAX_CHARACTERS = 60_000

function isMessage(value: unknown): value is ChatMessage {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<ChatMessage>
  return (
    (item.role === 'user' || item.role === 'assistant') &&
    typeof item.id === 'string' &&
    typeof item.content === 'string' &&
    (item.imagePath === undefined || typeof item.imagePath === 'string') &&
    typeof item.createdAt === 'number' &&
    Number.isFinite(item.createdAt)
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
    reversed.push({ content, createdAt: message.createdAt, id: message.id, imagePath: message.imagePath, role: message.role })
    characters += content.length
  }
  return { group: draft.group, messages: reversed.reverse(), model: draft.model, version: 1 }
}

export function createConversation(group = '', model = ''): ChatConversation {
  return {
    id: `chat-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    group,
    messages: [],
    model,
    updatedAt: Date.now(),
  }
}

export function conversationTitle(conversation: ChatConversation): string {
  const firstQuestion = conversation.messages.find((message) => message.role === 'user')?.content
  return firstQuestion?.replace(/\s+/g, ' ').trim().slice(0, 42) || ''
}

export function conversationHistory(conversations: ChatConversation[]): ChatConversation[] {
  return conversations.filter((conversation) => conversation.messages.length > 0)
    .sort((left, right) => right.updatedAt - left.updatedAt)
}

export function compactConversationStore(store: ConversationStore): ConversationStore {
  return {
    activeId: store.activeId,
    conversations: store.conversations.map((conversation) => {
      const compacted = compactConversation({
        group: conversation.group,
        messages: conversation.messages,
        model: conversation.model,
        version: 1,
      })
      return {
        id: conversation.id,
        group: compacted.group,
        messages: compacted.messages,
        model: compacted.model,
        updatedAt: conversation.updatedAt,
      }
    }),
    version: 2,
  }
}

export function loadConversationStore(): ConversationStore {
  try {
    const value = Taro.getStorageSync(STORAGE_KEY) as Partial<ConversationStore>
    if (value?.version === 2 && Array.isArray(value.conversations)) {
      const conversations = value.conversations.filter((item): item is ChatConversation => {
        if (!item || typeof item !== 'object') return false
        return typeof item.id === 'string' && typeof item.group === 'string' &&
          typeof item.model === 'string' && typeof item.updatedAt === 'number' &&
          Number.isFinite(item.updatedAt) && Array.isArray(item.messages) &&
          item.messages.every(isMessage)
      })
      if (conversations.length > 0) {
        const activeId = conversations.some((item) => item.id === value.activeId)
          ? value.activeId as string : conversations[0].id
        return compactConversationStore({ activeId, conversations, version: 2 })
      }
    }

    const legacy = Taro.getStorageSync(LEGACY_KEY) as Partial<ConversationDraft>
    if (legacy && (legacy.version === 1 || legacy.version === undefined) &&
      typeof legacy.group === 'string' && typeof legacy.model === 'string' &&
      Array.isArray(legacy.messages) && legacy.messages.every(isMessage)) {
      const compacted = compactConversation(legacy as ConversationDraft)
      const conversation: ChatConversation = {
        id: 'chat-legacy-v1',
        group: compacted.group,
        messages: compacted.messages,
        model: compacted.model,
        updatedAt: compacted.messages.at(-1)?.createdAt ?? Date.now(),
      }
      return { activeId: conversation.id, conversations: [conversation], version: 2 }
    }
  } catch {
    // Storage may be unavailable; keep the current session in memory.
  }
  const conversation = createConversation()
  return { activeId: conversation.id, conversations: [conversation], version: 2 }
}

export function saveConversationStore(store: ConversationStore): boolean {
  try {
    Taro.setStorageSync(STORAGE_KEY, compactConversationStore(store))
  } catch {
    return false
  }
  try {
    Taro.removeStorageSync(LEGACY_KEY)
  } catch {
    // The v2 copy was saved; legacy cleanup can be retried on the next save.
  }
  return true
}

export function removeConversationImages(messages: ChatMessage[]) {
  for (const path of new Set(messages.map((message) => message.imagePath).filter((value): value is string => !!value))) {
    void Taro.removeSavedFile({ filePath: path }).catch(() => undefined)
  }
}

export function clearConversation() {
  try {
    const stored = Taro.getStorageSync(STORAGE_KEY) as Partial<ConversationStore>
    for (const conversation of stored?.conversations ?? []) {
      if (Array.isArray(conversation?.messages)) removeConversationImages(conversation.messages)
    }
  } catch {
    // Continue clearing session data even if saved media cannot be inspected.
  }
  try {
    Taro.removeStorageSync(STORAGE_KEY)
  } catch {
    // Continue to remove the legacy copy if possible.
  }
  try {
    Taro.removeStorageSync(LEGACY_KEY)
  } catch {
    // Storage may be unavailable to the runtime.
  }
}
