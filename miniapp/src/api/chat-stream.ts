interface ChatDelta {
  content?: string
  reasoning_content?: string
}

interface ChatChunk {
  choices?: Array<{ delta?: ChatDelta; finish_reason?: string | null }>
  error?: { message?: string }
}

export type ChatStreamEvent =
  | { done: true; content: ''; reasoning: '' }
  | { done: false; content: string; reasoning: string }

export function decodeChatStreamEvent(data: string): ChatStreamEvent {
  if (data.trim() === '[DONE]') return { content: '', done: true, reasoning: '' }
  let chunk: ChatChunk
  try {
    chunk = JSON.parse(data) as ChatChunk
  } catch {
    throw new Error('STREAM_INVALID_EVENT')
  }
  if (chunk.error?.message) throw new Error(chunk.error.message)
  const delta = chunk.choices?.[0]?.delta
  return {
    content: delta?.content ?? '',
    done: false,
    reasoning: delta?.reasoning_content ?? '',
  }
}
