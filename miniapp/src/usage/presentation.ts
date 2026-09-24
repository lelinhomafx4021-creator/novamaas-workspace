import type { DrawingTask, PlatformTask, UsageLog } from '@/api/usage'

export interface UsageLogOther {
  billing_source?: string
  billing_mode?: string
  cache_creation_tokens?: number
  cache_creation_tokens_1h?: number
  cache_creation_tokens_5m?: number
  cache_tokens?: number
  completion_ratio?: number
  frt?: number
  group?: string
  group_ratio?: number
  login_method?: string
  matched_tier?: string
  model_price?: number
  model_ratio?: number
  request_path?: string
  stream_status?: string
  subscription_plan_title?: string
  subscription_remain?: number
  subscription_total?: number
  upstream_model_name?: string
  user_group_ratio?: number
  user_agent?: string
}

export interface MediaAsset {
  kind: 'audio' | 'image' | 'video'
  url: string
}

const logTypeKeys: Record<number, string> = {
  0: 'usage.logType.unknown',
  1: 'usage.logType.topup',
  2: 'usage.logType.consume',
  3: 'usage.logType.manage',
  4: 'usage.logType.system',
  5: 'usage.logType.error',
  6: 'usage.logType.refund',
  7: 'usage.logType.login',
}

const terminalStatuses = new Set([
  'CANCELLED',
  'FAILED',
  'FAILURE',
  'SUCCEEDED',
  'SUCCESS',
])

function parseUnknownJson(value: unknown): unknown {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return undefined
  }
}

function mediaKind(key: string, url: string): MediaAsset['kind'] | undefined {
  const normalizedKey = key.toLowerCase()
  if (normalizedKey.includes('audio')) return 'audio'
  if (normalizedKey.includes('video')) return 'video'
  if (normalizedKey.includes('image') || normalizedKey.includes('cover')) return 'image'
  const path = url.split('?')[0].toLowerCase()
  if (/\.(mp3|m4a|aac|wav|ogg|flac)$/.test(path)) return 'audio'
  if (/\.(mp4|mov|m4v|webm|mkv)$/.test(path)) return 'video'
  if (/\.(png|jpe?g|gif|webp|avif)$/.test(path)) return 'image'
  return undefined
}

function collectMedia(value: unknown, key: string, assets: MediaAsset[], depth: number) {
  if (assets.length >= 12 || depth > 5 || value == null) return
  if (typeof value === 'string') {
    const url = value.trim()
    if (!/^(https?:\/\/|\/)/i.test(url)) return
    const kind = mediaKind(key, url)
    if (kind && !assets.some((asset) => asset.url === url)) assets.push({ kind, url })
    return
  }
  if (Array.isArray(value)) {
    for (const item of value) collectMedia(item, key, assets, depth + 1)
    return
  }
  if (typeof value === 'object') {
    for (const [childKey, childValue] of Object.entries(value as Record<string, unknown>)) {
      collectMedia(childValue, `${key}.${childKey}`, assets, depth + 1)
    }
  }
}

export function getUsageLogTypeKey(type: number) {
  return logTypeKeys[type] ?? logTypeKeys[0]
}

export function showsUsageCost(log: UsageLog) {
  return log.type === 0 || log.type === 2 || log.type === 5 || log.type === 6
}

export function parseUsageLogOther(value: string): UsageLogOther {
  const parsed = parseUnknownJson(value)
  return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
    ? (parsed as UsageLogOther)
    : {}
}

export function getCacheTokens(other: UsageLogOther) {
  const cacheWrite =
    Number(other.cache_creation_tokens_5m || 0) + Number(other.cache_creation_tokens_1h || 0)
  return {
    read: Number(other.cache_tokens || 0),
    write: cacheWrite || Number(other.cache_creation_tokens || 0),
  }
}

export function getEffectiveGroupRatio(other: UsageLogOther) {
  if (
    Number.isFinite(other.user_group_ratio) &&
    other.user_group_ratio !== -1
  ) {
    return other.user_group_ratio
  }
  return Number.isFinite(other.group_ratio) ? other.group_ratio : undefined
}

export function getTaskModel(task: PlatformTask) {
  return task.properties?.origin_model_name || task.properties?.upstream_model_name || ''
}

export function getTaskDuration(task: PlatformTask) {
  if (!task.finish_time || !task.submit_time) return undefined
  return Math.max(0, task.finish_time - task.submit_time)
}

export function getDrawingDuration(task: DrawingTask) {
  if (!task.finish_time || !task.submit_time) return undefined
  return Math.max(0, (task.finish_time - task.submit_time) / 1000)
}

export function isTerminalTaskStatus(status: string) {
  return terminalStatuses.has(status.toUpperCase())
}

export function getTaskStatusKey(status: string) {
  const normalized = status.toUpperCase()
  if (normalized === 'SUCCESS' || normalized === 'SUCCEEDED') return 'usage.taskStatus.success'
  if (normalized === 'FAILURE' || normalized === 'FAILED') return 'usage.taskStatus.failure'
  if (normalized === 'IN_PROGRESS' || normalized === 'PROCESSING') return 'usage.taskStatus.processing'
  if (normalized === 'QUEUED' || normalized === 'SUBMITTED') return 'usage.taskStatus.queued'
  if (normalized === 'NOT_START') return 'usage.taskStatus.pending'
  if (normalized === 'CANCELLED') return 'usage.taskStatus.cancelled'
  return 'usage.taskStatus.unknown'
}

export function getTaskMedia(task: PlatformTask) {
  const assets: MediaAsset[] = []
  const audioTask = task.platform?.toLowerCase() === 'suno' ||
    /(audio|music|song|speech|tts)/i.test(task.action || '')
  collectMedia(task.result_url, audioTask ? 'audio_url' : 'video_url', assets, 0)
  collectMedia(parseUnknownJson(task.data), 'data', assets, 0)
  return assets
}

export function getDrawingMedia(task: DrawingTask) {
  const assets: MediaAsset[] = []
  collectMedia(task.image_url, 'image_url', assets, 0)
  collectMedia(task.video_url, 'video_url', assets, 0)
  collectMedia(parseUnknownJson(task.video_urls), 'video_urls', assets, 0)
  return assets
}
