import { describe, expect, it } from 'vitest'

import { buildMediaPreviewRoute, readMediaPreviewRoute } from '../media-preview-route'

describe('media preview navigation', () => {
  it('round-trips a downloaded local file path through the preview route', () => {
    const route = buildMediaPreviewRoute('video', 'wxfile://tmp/session 100%.mp4')
    const params = Object.fromEntries(new URLSearchParams(route.split('?')[1]))

    expect(readMediaPreviewRoute(params)).toEqual({
      kind: 'video',
      filePath: 'wxfile://tmp/session 100%.mp4',
    })
  })

  it('rejects a missing or unsupported preview target', () => {
    expect(readMediaPreviewRoute({ kind: 'audio', src: 'wxfile://tmp/a.mp3' })).toBeNull()
    expect(readMediaPreviewRoute({ kind: 'image' })).toBeNull()
  })
})
