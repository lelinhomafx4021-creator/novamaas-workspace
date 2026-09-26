import { describe, expect, it } from 'vitest'

import { decodeChatStreamEvent } from '../chat-stream'

describe('decodeChatStreamEvent', () => {
  it('reads content and reasoning deltas', () => {
    expect(
      decodeChatStreamEvent('{"choices":[{"delta":{"content":"ok","reasoning_content":"r"}}]}')
    ).toEqual({ content: 'ok', done: false, reasoning: 'r' })
  })

  it('recognizes the done sentinel', () => {
    expect(decodeChatStreamEvent('[DONE]').done).toBe(true)
  })

  it('rejects malformed and provider error events', () => {
    expect(() => decodeChatStreamEvent('{broken')).toThrow('STREAM_INVALID_EVENT')
    expect(() => decodeChatStreamEvent('{"error":{"message":"upstream failed"}}')).toThrow(
      'upstream failed'
    )
  })
})
