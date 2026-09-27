import { describe, expect, it } from 'vitest'

import { parseChatInline, parseChatMarkdown } from '../format-message'

describe('chat answer presentation', () => {
  it('renders the response structure shown in the mini program without raw bold or bullet markers', () => {
    const blocks = parseChatMarkdown('决定前应考虑什么？\n\n**1. 明确核心目标与价值观\n**\n- **我到底想要什么？** 不要被短期诱惑带偏。\n- **这个决定符合我的长期价值观吗？**')
    expect(blocks).toEqual([
      { kind: 'paragraph', text: '决定前应考虑什么？' },
      { kind: 'heading', level: 3, text: '**1. 明确核心目标与价值观**' },
      { kind: 'list', items: [
        { depth: 0, marker: '•', text: '**我到底想要什么？** 不要被短期诱惑带偏。' },
        { depth: 0, marker: '•', text: '**这个决定符合我的长期价值观吗？**' },
      ] },
    ])
    expect(parseChatInline(blocks[1].text || '')).toEqual([{ kind: 'strong', text: '1. 明确核心目标与价值观' }])
    expect(parseChatInline(blocks[2].items?.[0].text || '')).toEqual([
      { kind: 'strong', text: '我到底想要什么？' },
      { kind: 'text', text: ' 不要被短期诱惑带偏。' },
    ])
  })

  it('supports headings, numbered lists, quotes, code and safe link text', () => {
    expect(parseChatMarkdown('# Summary\n1. First\n2. Second\n\n> Note\n\n```go\nfmt.Println("hi")\n```')).toEqual([
      { kind: 'heading', level: 1, text: 'Summary' },
      { kind: 'list', items: [
        { depth: 0, marker: '1.', text: 'First' },
        { depth: 0, marker: '2.', text: 'Second' },
      ] },
      { kind: 'quote', text: 'Note' },
      { kind: 'code', text: 'fmt.Println("hi")' },
    ])
    expect(parseChatInline('[docs](https://example.com) and `code`')).toEqual([
      { kind: 'link', text: 'docs (https://example.com)' },
      { kind: 'text', text: ' and ' },
      { kind: 'code', text: 'code' },
    ])
  })

  it('keeps incomplete streaming markup readable', () => {
    expect(parseChatInline('A **partial')).toEqual([{ kind: 'text', text: 'A **partial' }])
  })

  it('keeps indented subpoints as separate list rows', () => {
    expect(parseChatMarkdown('- **后果**：\n  - **短期 vs 长期**：影响\n  - **可控**：区分')).toEqual([
      { kind: 'list', items: [
        { depth: 0, marker: '•', text: '**后果**：' },
        { depth: 1, marker: '•', text: '**短期 vs 长期**：影响' },
        { depth: 1, marker: '•', text: '**可控**：区分' },
      ] },
    ])
  })
})
