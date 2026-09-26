import { apiRequest } from './request'
import { buildQuery } from './query'

export interface AccountSummary {
  display_name: string
  email: string
  group: string
  id: number
  quota: number
  request_count: number
  role: number
  status: number
  used_quota: number
  username: string
}

export interface UsageStats {
  cost_quota?: number
  profit_quota?: number
  quota: number
  records: number
  requests: number
  revenue_quota?: number
  rpm: number
  tpm: number
}

export interface UsageStatsFilter {
  channel?: number
  endTimestamp?: number
  group?: string
  isAdmin?: boolean
  logType?: number
  modelName?: string
  requestId?: string
  startTimestamp?: number
  tokenName?: string
  upstreamRequestId?: string
  username?: string
}

export interface UserSubscription {
  amount_total: number
  amount_used: number
  end_time: number
  id: number
  plan_id: number
  start_time: number
  status: string
}

export interface SubscriptionSummary {
  billing_preference: string
  subscriptions: Array<{ subscription: UserSubscription }>
}

export function getAccountSummary() {
  return apiRequest<AccountSummary>('/api/user/self')
}

export function getSubscriptionSummary() {
  return apiRequest<SubscriptionSummary>('/api/subscription/self')
}

export function getUsageStats(filter: UsageStatsFilter = {}) {
  const suffix = buildQuery({
    channel: filter.channel,
    end_timestamp: filter.endTimestamp,
    group: filter.group,
    model_name: filter.modelName,
    request_id: filter.requestId,
    start_timestamp: filter.startTimestamp,
    token_name: filter.tokenName,
    type: filter.logType,
    upstream_request_id: filter.upstreamRequestId,
    username: filter.username,
  })
  const basePath = filter.isAdmin ? '/api/log/stat' : '/api/log/self/stat'
  return apiRequest<UsageStats>(`${basePath}${suffix ? `?${suffix}` : ''}`)
}

export function getPlatformNotice() {
  return apiRequest<string>('/api/notice', { auth: false })
}
