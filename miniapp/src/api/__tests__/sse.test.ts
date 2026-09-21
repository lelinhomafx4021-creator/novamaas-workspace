import { describe, expect, it } from 'vitest'

import { SseParser, type SseEvent } from '../sse'

describe('SseParser', () => {
  it('preserves split UTF-8 characters and fragmented lines', () => {
    const events: SseEvent[] = []
    const parser = new SseParser((event) => events.push(event))
    const bytes = new TextEncoder().encode('data: {"text":"你好"}\n\n')
    parser.push(bytes.slice(0, bytes.length - 4))
    parser.push(bytes.slice(bytes.length - 4))
    parser.finish()
    expect(events).toEqual([{ data: '{"text":"你好"}', event: '', id: '' }])
  })

  it('joins multiline data and handles CRLF and done events', () => {
    const events: SseEvent[] = []
    const parser = new SseParser((event) => events.push(event))
    parser.push(new TextEncoder().encode('id: 7\r\nevent: chunk\r\ndata: first\r\ndata: second\r\n\r\ndata: [DONE]\n\n'))
    parser.finish()
    expect(events).toEqual([
      { data: 'first\nsecond', event: 'chunk', id: '7' },
      { data: '[DONE]', event: '', id: '7' },
    ])
  })

  it('flushes a final event without a trailing blank line', () => {
    const events: SseEvent[] = []
    const parser = new SseParser((event) => events.push(event))
    parser.push(new TextEncoder().encode('data: final'))
    parser.finish()
    expect(events[0]?.data).toBe('final')
  })
})
