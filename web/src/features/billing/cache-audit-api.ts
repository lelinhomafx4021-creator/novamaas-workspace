/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

export type CacheAuditStatus =
  | 'affected'
  | 'review'
  | 'no_evidence'
  | 'not_affected'
  | 'refund'

export type CacheAuditProtocol =
  | 'messages'
  | 'chat_completions'
  | 'responses'
  | 'other'

export interface CacheAuditRequest {
  user_id: number
  token_id: number
  start_at: number
  end_at: number
  model: string
  fixed_at: number
  status: CacheAuditStatus | ''
  protocol: CacheAuditProtocol | ''
  downstream_input_prices?: Record<string, string>
}

export interface CacheAuditCompensation {
  list: string
  group: string
  downstream?: string
  list_max: string
  group_max: string
  downstream_max?: string
}

export interface CacheAuditTokens {
  total_input: number
  input: number
  cache_read: number
  cache_write: number
  output: number
}

export interface CacheAuditCost {
  input: string
  cache_read: string
  cache_write: string
  output: string
  total: string
}

export interface CacheAuditRow {
  log_id: number
  request_id: string
  created_at: number
  type: number
  user_id: number
  username: string
  token_id: number
  token_name: string
  model: string
  group: string
  is_stream: boolean
  request_path: string
  protocol: CacheAuditProtocol
  usage_semantic: string
  status: CacheAuditStatus
  reason?: string
  price_issue?: string
  price_mismatch?: boolean
  group_ratio: string
  actual_quota: number
  actual_amount: string
  reported_input_tokens: number
  correct_input_tokens: number
  duplicate_tokens: number
  duplicate_tokens_max: number
  tokens: CacheAuditTokens
  list_cost?: CacheAuditCost
  discounted_cost?: CacheAuditCost
  input_price_per_million?: string
  cache_read_price_per_million?: string
  compensation?: CacheAuditCompensation
}

export interface CacheAuditTotals {
  key: string
  requests: number
  affected_requests: number
  review_requests: number
  no_evidence_requests: number
  priced_requests: number
  tokens: CacheAuditTokens
  duplicate_tokens: number
  duplicate_tokens_max: number
  affected_reported_input: number
  affected_correct_input: number
  input_price_per_million?: string
  cache_read_price_per_million?: string
  price_mixed?: boolean
  list_cost: CacheAuditCost
  discounted_cost: CacheAuditCost
  rates: string[]
  actual_amount: string
  list_amount: string
  discounted_amount: string
  compensation: CacheAuditCompensation
  unpriced_duplicate_tokens: number
}

export interface CacheAuditReport {
  request: CacheAuditRequest
  currency: { code: string; symbol: string }
  total: CacheAuditTotals
  by_day: CacheAuditTotals[]
  by_model: CacheAuditTotals[]
  by_token: CacheAuditTotals[]
  by_user: CacheAuditTotals[]
  rows: CacheAuditRow[]
  truncated: boolean
}

export interface CacheAuditKey {
  id: number
  user_id: number
  name: string
  deleted: boolean
}

interface Response<T> {
  success: boolean
  data: T
  message?: string
}

function unwrap<T>(response: Response<T>): T {
  if (!response.success) {
    throw new Error(response.message || 'Something went wrong!')
  }
  return response.data
}

export async function runCacheAudit(input: CacheAuditRequest) {
  return unwrap(
    (
      await api.post<Response<CacheAuditReport>>(
        '/api/billing/admin/cache-usage-audit',
        input
      )
    ).data
  )
}

export async function searchCacheAuditKeys(userId: number, keyword: string) {
  return unwrap(
    (
      await api.get<Response<CacheAuditKey[]>>(
        '/api/billing/admin/cache-usage-audit/keys',
        { params: { user_id: userId, keyword } }
      )
    ).data
  )
}

export async function exportCacheAudit(
  input: CacheAuditRequest,
  format: 'csv' | 'json'
) {
  const response = await api.post<Blob>(
    '/api/billing/admin/cache-usage-audit/export',
    input,
    { params: { format }, responseType: 'blob' }
  )
  const url = URL.createObjectURL(response.data)
  const anchor = document.createElement('a')
  anchor.href = url
  const disposition = response.headers['content-disposition']
  const match =
    typeof disposition === 'string'
      ? disposition.match(/filename="([^"]+)"/)
      : null
  anchor.download = match?.[1] || `cache-usage-audit.${format}`
  anchor.click()
  URL.revokeObjectURL(url)
}
