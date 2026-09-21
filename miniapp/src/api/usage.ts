import { apiRequest, getAuthorizedMiniSession, getConfiguredApiBaseUrl } from './request'
import { buildQuery } from './query'
import { buildApiUrl } from './url'

export interface UsageLog {
  channel: number
  channel_name: string
  completion_tokens: number
  content: string
  created_at: number
  group: string
  id: number
  ip: string
  is_stream: boolean
  model_name: string
  other: string
  prompt_tokens: number
  quota: number
  request_id: string
  token_id: number
  token_name: string
  type: number
  upstream_request_id: string
  use_time: number
  user_id: number
  username: string
}

export interface PlatformTask {
  action: string
  channel_id: number
  created_at: number
  data: unknown
  fail_reason: string
  finish_time: number
  group: string
  id: number
  platform: string
  progress: string
  properties?: {
    input?: string
    origin_model_name?: string
    upstream_model_name?: string
  }
  quota: number
  request_body_available: boolean
  result_url: string
  start_time: number
  status: string
  submit_time: number
  task_id: string
  updated_at: number
  user_id: number
  username?: string
}

export interface DrawingTask {
  action: string
  channel_id: number
  code: number
  description: string
  fail_reason: string
  finish_time: number
  id: number
  image_url: string
  mj_id: string
  progress: string
  prompt: string
  prompt_en: string
  properties: string
  quota: number
  start_time: number
  status: string
  submit_time: number
  user_id: number
  video_url: string
  video_urls: string
}

export interface BillingStatement {
  confirmed_at: number
  created_at: number
  due_at: number
  end_at: number
  id: string
  issued_at: number
  month: string
  revision: number
  start_at: number
  status: string
}

export interface BillingArtifact {
  id: number
  kind: string
  ordinal: number
  rows: number
  sha256: string
  size: number
}

export interface BillingEvent {
  action?: string
  actor_username?: string
  created_at?: number
  id?: number
  note?: string
}

export interface BillingStatementDetail {
  artifacts: BillingArtifact[]
  customer: Record<string, unknown>
  detail_count: number
  events: BillingEvent[]
  source_warning: string
  statement: BillingStatement
}

interface Page<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export interface UsageFilter {
  channel?: number
  endTimestamp?: number
  group?: string
  isAdmin?: boolean
  logType?: number
  modelName?: string
  page?: number
  pageSize?: number
  requestId?: string
  startTimestamp?: number
  tokenName?: string
  upstreamRequestId?: string
  username?: string
}

export function getUsageLogs(filter: UsageFilter) {
  const query = buildQuery({
    end_timestamp: filter.endTimestamp,
    channel: filter.channel,
    group: filter.group,
    model_name: filter.modelName,
    p: filter.page ?? 1,
    page_size: filter.pageSize ?? 20,
    request_id: filter.requestId,
    start_timestamp: filter.startTimestamp,
    token_name: filter.tokenName,
    type: filter.logType,
    upstream_request_id: filter.upstreamRequestId,
    username: filter.username,
  })
  return apiRequest<Page<UsageLog>>(`${filter.isAdmin ? '/api/log' : '/api/log/self'}?${query}`)
}

export interface MediaFilter {
  channelId?: number
  endTimestamp?: number
  id?: string
  isAdmin?: boolean
  page?: number
  pageSize?: number
  platform?: string
  startTimestamp?: number
  status?: string
}

export function getTasks(filter: MediaFilter = {}) {
  const query = buildQuery({
    channel_id: filter.channelId,
    end_timestamp: filter.endTimestamp,
    p: filter.page ?? 1,
    page_size: filter.pageSize ?? 20,
    platform: filter.platform,
    start_timestamp: filter.startTimestamp,
    status: filter.status,
    task_id: filter.id,
  })
  return apiRequest<Page<PlatformTask>>(`${filter.isAdmin ? '/api/task/' : '/api/task/self'}?${query}`)
}

export function getDrawingTasks(filter: MediaFilter = {}) {
  const query = buildQuery({
    channel_id: filter.channelId,
    end_timestamp: filter.endTimestamp,
    mj_id: filter.id,
    p: filter.page ?? 1,
    page_size: filter.pageSize ?? 20,
    start_timestamp: filter.startTimestamp,
  })
  return apiRequest<Page<DrawingTask>>(`${filter.isAdmin ? '/api/mj/' : '/api/mj/self'}?${query}`)
}

export function getBillingStatements(before = '', beforeId = '') {
  const query = buildQuery({ before, before_id: beforeId })
  return apiRequest<BillingStatement[]>(`/api/billing/statements${query ? `?${query}` : ''}`)
}

export function getBillingStatement(id: string) {
  return apiRequest<BillingStatementDetail>(`/api/billing/statements/${id}`)
}

export async function getBillingArtifactDownloadOptions(
  statementId: string,
  kind: string,
  ordinal: number
) {
  const session = await getAuthorizedMiniSession()
  return {
    header: { Authorization: `Bearer ${session.accessToken}` },
    url: buildApiUrl(
      getConfiguredApiBaseUrl(),
      `/api/billing/statements/${statementId}/files/${encodeURIComponent(kind)}/${ordinal}`
    ),
  }
}

export async function getTaskResultDownloadOptions(resultUrl: string) {
  const baseUrl = getConfiguredApiBaseUrl()
  const isRelative = resultUrl.startsWith('/')
  const isPlatformUrl = resultUrl === baseUrl || resultUrl.startsWith(`${baseUrl}/`)
  if (!isRelative && !isPlatformUrl) return { url: resultUrl }
  const session = await getAuthorizedMiniSession()
  return {
    header: { Authorization: `Bearer ${session.accessToken}` },
    url: isRelative ? buildApiUrl(baseUrl, resultUrl) : resultUrl,
  }
}
