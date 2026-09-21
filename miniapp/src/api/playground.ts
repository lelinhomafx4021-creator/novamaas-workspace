import Taro from '@tarojs/taro'

import type { ChatMessage } from '@/playground/storage'

import { decodeChatStreamEvent } from './chat-stream'
import { getAuthorizedMiniSession, getConfiguredApiBaseUrl } from './request'
import { SseParser } from './sse'
import { buildApiUrl } from './url'

export interface StreamingChat {
  abort: () => void
  completion: Promise<void>
}

export interface StreamingChatCallbacks {
  onDelta: (content: string, reasoning: string) => void
  onDone: () => void
}

export async function startStreamingChat(
  model: string,
  group: string,
  messages: ChatMessage[],
  callbacks: StreamingChatCallbacks
): Promise<StreamingChat> {
  const session = await getAuthorizedMiniSession()
  let task: Taro.RequestTask<unknown> | undefined
  let resolveCompletion!: () => void
  let rejectCompletion!: (error: Error) => void
  let settled = false
  const completion = new Promise<void>((resolve, reject) => {
    resolveCompletion = resolve
    rejectCompletion = reject
  })

  const finish = () => {
    if (settled) return
    settled = true
    callbacks.onDone()
    resolveCompletion()
  }
  const fail = (error: Error) => {
    if (settled) return
    settled = true
    task?.abort()
    rejectCompletion(error)
  }
  const parser = new SseParser((event) => {
    try {
      const decoded = decodeChatStreamEvent(event.data)
      if (decoded.done) finish()
      else if (decoded.content || decoded.reasoning) {
        callbacks.onDelta(decoded.content, decoded.reasoning)
      }
    } catch (error) {
      fail(error instanceof Error ? error : new Error('STREAM_INVALID_EVENT'))
    }
  })

  task = Taro.request<unknown>({
    url: buildApiUrl(getConfiguredApiBaseUrl(), '/pg/chat/completions'),
    method: 'POST',
    enableChunked: true,
    header: {
      Accept: 'text/event-stream',
      Authorization: `Bearer ${session.accessToken}`,
      'Content-Type': 'application/json',
    },
    data: {
      group,
      messages: messages.map(({ content, role }) => ({ content, role })),
      model,
      stream: true,
    },
    success: (response) => {
      parser.finish()
      if (response.statusCode >= 200 && response.statusCode < 300) finish()
      else fail(new Error(`STREAM_HTTP_${response.statusCode}`))
    },
    fail: (error) => fail(new Error(error.errMsg || 'STREAM_REQUEST_FAILED')),
  })

  task.onChunkReceived(({ data }) => parser.push(data))
  return {
    abort: () => {
      fail(new Error('STREAM_ABORTED'))
    },
    completion,
  }
}
