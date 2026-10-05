export function getPreviewErrorCode(detail: unknown) {
  if (!detail || typeof detail !== 'object') return ''
  const values = detail as Record<string, unknown>
  for (const key of ['errCode', 'code', 'errno', 'statusCode']) {
    const value = values[key]
    if (typeof value === 'number' && Number.isFinite(value)) return String(value)
    if (typeof value === 'string' && value.trim()) return value.trim()
  }
  const message = values.errMsg
  if (typeof message === 'string' && /^(\d+|[A-Z][A-Z0-9_]+)$/.test(message.trim())) return message.trim()
  return ''
}
