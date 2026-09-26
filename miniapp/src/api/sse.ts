export interface SseEvent {
  data: string
  event: string
  id: string
}

class Utf8StreamDecoder {
  private pending = new Uint8Array(0)

  decode(chunk: ArrayBuffer | Uint8Array, final = false) {
    const incoming = chunk instanceof Uint8Array ? chunk : new Uint8Array(chunk)
    const bytes = new Uint8Array(this.pending.length + incoming.length)
    bytes.set(this.pending)
    bytes.set(incoming, this.pending.length)

    let result = ''
    let index = 0
    while (index < bytes.length) {
      const first = bytes[index]
      let length = 1
      let codePoint = first
      if (first >= 0xc2 && first <= 0xdf) {
        length = 2
        codePoint = first & 0x1f
      } else if (first >= 0xe0 && first <= 0xef) {
        length = 3
        codePoint = first & 0x0f
      } else if (first >= 0xf0 && first <= 0xf4) {
        length = 4
        codePoint = first & 0x07
      } else if (first >= 0x80) {
        result += '\uFFFD'
        index += 1
        continue
      }

      if (index + length > bytes.length) {
        if (!final) break
        result += '\uFFFD'
        index += 1
        continue
      }

      let valid = true
      for (let offset = 1; offset < length; offset += 1) {
        const continuation = bytes[index + offset]
        if ((continuation & 0xc0) !== 0x80) {
          valid = false
          break
        }
        codePoint = (codePoint << 6) | (continuation & 0x3f)
      }
      const overlong =
        (length === 2 && codePoint < 0x80) ||
        (length === 3 && codePoint < 0x800) ||
        (length === 4 && codePoint < 0x10000)
      if (!valid || overlong || codePoint > 0x10ffff || (codePoint >= 0xd800 && codePoint <= 0xdfff)) {
        result += '\uFFFD'
        index += 1
        continue
      }
      result += String.fromCodePoint(codePoint)
      index += length
    }

    this.pending = final ? new Uint8Array(0) : bytes.slice(index)
    return result
  }
}

export class SseParser {
  private dataLines: string[] = []
  private decoder = new Utf8StreamDecoder()
  private event = ''
  private id = ''
  private lineBuffer = ''

  constructor(private readonly onEvent: (event: SseEvent) => void) {}

  push(chunk: ArrayBuffer | Uint8Array) {
    this.consumeText(this.decoder.decode(chunk))
  }

  finish() {
    this.consumeText(this.decoder.decode(new Uint8Array(0), true), true)
  }

  private consumeText(text: string, final = false) {
    this.lineBuffer += text
    let index = 0
    while (index < this.lineBuffer.length) {
      const character = this.lineBuffer[index]
      if (character !== '\n' && character !== '\r') {
        index += 1
        continue
      }
      if (character === '\r' && index + 1 === this.lineBuffer.length && !final) break
      const line = this.lineBuffer.slice(0, index)
      const delimiterLength = character === '\r' && this.lineBuffer[index + 1] === '\n' ? 2 : 1
      this.lineBuffer = this.lineBuffer.slice(index + delimiterLength)
      index = 0
      this.consumeLine(line)
    }
    if (final && this.lineBuffer) {
      this.consumeLine(this.lineBuffer)
      this.lineBuffer = ''
    }
    if (final) this.dispatch()
  }

  private consumeLine(line: string) {
    if (!line) {
      this.dispatch()
      return
    }
    if (line.startsWith(':')) return

    const separator = line.indexOf(':')
    const field = separator === -1 ? line : line.slice(0, separator)
    let value = separator === -1 ? '' : line.slice(separator + 1)
    if (value.startsWith(' ')) value = value.slice(1)

    if (field === 'data') this.dataLines.push(value)
    else if (field === 'event') this.event = value
    else if (field === 'id' && !value.includes('\0')) this.id = value
  }

  private dispatch() {
    if (this.dataLines.length === 0) return
    this.onEvent({ data: this.dataLines.join('\n'), event: this.event, id: this.id })
    this.dataLines = []
    this.event = ''
  }
}
