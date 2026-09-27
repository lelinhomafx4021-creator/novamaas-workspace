export interface ChatInlineSegment {
  kind: 'text' | 'strong' | 'emphasis' | 'code' | 'link'
  text: string
}

export interface ChatBlock {
  kind: 'paragraph' | 'heading' | 'list' | 'quote' | 'code' | 'rule'
  text?: string
  level?: number
  items?: ChatListItem[]
}

export interface ChatListItem {
  depth: number
  marker: string
  text: string
}

const blockStart = /^(?:#{1,6}\s|[-*+]\s|\d+[.)]\s|>\s?|```|~~~|(?:-{3,}|\*{3,}|_{3,})\s*$)/

export function parseChatMarkdown(content: string): ChatBlock[] {
  const lines = content.replace(/\r\n?/g, '\n').split('\n')
  const blocks: ChatBlock[] = []
  let index = 0

  while (index < lines.length) {
    const line = lines[index].trim()
    if (!line) {
      index += 1
      continue
    }

    if (/^(```|~~~)/.test(line)) {
      const fence = line.slice(0, 3)
      const code: string[] = []
      index += 1
      while (index < lines.length && !lines[index].trim().startsWith(fence)) {
        code.push(lines[index])
        index += 1
      }
      if (index < lines.length) index += 1
      blocks.push({ kind: 'code', text: code.join('\n') })
      continue
    }

    if (/^(?:-{3,}|\*{3,}|_{3,})$/.test(line)) {
      blocks.push({ kind: 'rule' })
      index += 1
      continue
    }

    const heading = /^(#{1,6})\s+(.+)$/.exec(line)
    if (heading) {
      blocks.push({ kind: 'heading', level: heading[1].length, text: heading[2] })
      index += 1
      continue
    }

    const listMatch = /^(\s*)([-*+]|\d+[.)])\s+(.+)$/.exec(lines[index])
    if (listMatch) {
      const items: ChatListItem[] = []
      while (index < lines.length) {
        const next = /^(\s*)([-*+]|\d+[.)])\s+(.+)$/.exec(lines[index])
        if (next) {
          items.push({
            depth: Math.min(3, Math.floor(next[1].length / 2)),
            marker: /^\d/.test(next[2]) ? next[2].replace(/\)$/, '.') : '•',
            text: next[3],
          })
          index += 1
          continue
        }
        if (/^\s{2,}\S/.test(lines[index]) && items.length > 0) {
          items[items.length - 1].text += ` ${lines[index].trim()}`
          index += 1
          continue
        }
        break
      }
      blocks.push({ kind: 'list', items })
      continue
    }

    if (/^>\s?/.test(line)) {
      const quoted: string[] = []
      while (index < lines.length && /^>\s?/.test(lines[index].trim())) {
        quoted.push(lines[index].trim().replace(/^>\s?/, ''))
        index += 1
      }
      blocks.push({ kind: 'quote', text: quoted.join('\n') })
      continue
    }

    const paragraph = [line]
    index += 1
    while (index < lines.length && lines[index].trim() && !blockStart.test(lines[index].trim())) {
      paragraph.push(lines[index].trim())
      index += 1
    }
    // Some providers emit the closing bold marker on the following line.
    const text = paragraph.join('\n').replace(/\*\*([^*\n]+)\n\*\*/g, '**$1**')
    blocks.push(/^\*\*\d+[.)].+\*\*$/.test(text)
      ? { kind: 'heading', level: 3, text }
      : { kind: 'paragraph', text })
  }

  return blocks
}

export function parseChatInline(content: string): ChatInlineSegment[] {
  const segments: ChatInlineSegment[] = []
  const token = /(\*\*[^*\n]+\*\*|__[^_\n]+__|`[^`\n]+`|\*[^*\n]+\*|_[^_\n]+_|\[[^\]\n]+\]\([^)\n]+\))/g
  let cursor = 0
  for (const match of content.matchAll(token)) {
    const position = match.index ?? 0
    if (position > cursor) segments.push({ kind: 'text', text: content.slice(cursor, position) })
    const raw = match[0]
    if (raw.startsWith('**') || raw.startsWith('__')) {
      segments.push({ kind: 'strong', text: raw.slice(2, -2) })
    } else if (raw.startsWith('`')) {
      segments.push({ kind: 'code', text: raw.slice(1, -1) })
    } else if (raw.startsWith('[')) {
      const closing = raw.indexOf('](')
      segments.push({ kind: 'link', text: `${raw.slice(1, closing)} (${raw.slice(closing + 2, -1)})` })
    } else {
      segments.push({ kind: 'emphasis', text: raw.slice(1, -1) })
    }
    cursor = position + raw.length
  }
  if (cursor < content.length) segments.push({ kind: 'text', text: content.slice(cursor) })
  return segments
}
