export function normalizeApiBaseUrl(value: string) {
  const normalized = value.trim().replace(/\/+$/, '')
  if (!normalized) {
    return ''
  }
  if (!/^https?:\/\//i.test(normalized)) {
    throw new Error('MINIAPP_API_BASE_URL must use http:// or https://')
  }
  return normalized
}

export function buildApiUrl(baseUrl: string, path: string) {
  const normalizedBaseUrl = normalizeApiBaseUrl(baseUrl)
  if (!normalizedBaseUrl) {
    throw new Error('MINIAPP_API_BASE_URL is not configured')
  }

  return `${normalizedBaseUrl}/${path.replace(/^\/+/, '')}`
}
