import { describe, expect, it } from 'vitest'

import { buildApiUrl, normalizeApiBaseUrl } from '../url'

describe('API URL handling', () => {
  it('normalizes whitespace and trailing slashes', () => {
    expect(normalizeApiBaseUrl(' https://api.example.com/// ')).toBe(
      'https://api.example.com'
    )
  })

  it('builds a URL without duplicate path separators', () => {
    expect(buildApiUrl('https://api.example.com/', '/api/status')).toBe(
      'https://api.example.com/api/status'
    )
  })

  it('rejects an empty or unsupported base URL', () => {
    expect(() => buildApiUrl('', '/api/status')).toThrow(
      'MINIAPP_API_BASE_URL is not configured'
    )
    expect(() => normalizeApiBaseUrl('ftp://api.example.com')).toThrow(
      'MINIAPP_API_BASE_URL must use http:// or https://'
    )
  })
})
