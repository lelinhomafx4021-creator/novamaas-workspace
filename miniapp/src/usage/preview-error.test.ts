import { describe, expect, it } from 'vitest'

import { getPreviewErrorCode } from './preview-error'

describe('preview error codes', () => {
  it('keeps HTTP and player error codes for an actionable dialog', () => {
    expect(getPreviewErrorCode({ statusCode: 403 })).toBe('403')
    expect(getPreviewErrorCode({ errCode: 1001, errMsg: 'load failed' })).toBe('1001')
    expect(getPreviewErrorCode({ code: 'MEDIA_ERR_NETWORK' })).toBe('MEDIA_ERR_NETWORK')
    expect(getPreviewErrorCode({ errMsg: 'MEDIA_ERR_DECODE' })).toBe('MEDIA_ERR_DECODE')
    expect(getPreviewErrorCode({ errMsg: 'load failed' })).toBe('')
  })
})
