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
import { z } from 'zod'

import type {
  CacheAuditReport,
  CacheAuditRequest,
  CacheAuditStatus,
} from '../cache-audit-api'

const DAY = 86400
export const CACHE_AUDIT_MAX_DAYS = 93

export const cacheAuditSchema = z
  .object({
    start: z.string().min(10),
    end: z.string().min(10),
    tokenId: z.number().int().min(0),
    model: z.string().trim().max(200),
    fixedAt: z.string(),
    protocol: z.enum([
      '',
      'messages',
      'chat_completions',
      'responses',
      'other',
    ]),
    status: z.enum([
      '',
      'affected',
      'review',
      'no_evidence',
      'not_affected',
      'refund',
    ]),
  })
  .superRefine((value, context) => {
    const days =
      (shanghaiMidnight(value.end) - shanghaiMidnight(value.start)) / DAY + 1
    if (!(days >= 1)) {
      context.addIssue({
        code: 'custom',
        path: ['end'],
        message: 'End date must not be earlier than start date',
      })
    } else if (days > CACHE_AUDIT_MAX_DAYS) {
      context.addIssue({
        code: 'custom',
        path: ['end'],
        message: 'Audit period cannot exceed 93 days',
      })
    }
  })
export type CacheAuditValues = z.infer<typeof cacheAuditSchema>

// Billing dates are Asia/Shanghai calendar days.
function shanghaiMidnight(date: string) {
  return Date.parse(`${date}T00:00:00+08:00`) / 1000
}

export function cacheAuditRequest(
  userId: number,
  value: CacheAuditValues
): CacheAuditRequest {
  return {
    user_id: userId,
    token_id: value.tokenId,
    start_at: shanghaiMidnight(value.start),
    end_at: shanghaiMidnight(value.end) + DAY,
    model: value.model.trim(),
    fixed_at: value.fixedAt
      ? Date.parse(`${value.fixedAt}:00+08:00`) / 1000
      : 0,
    status: value.status,
    protocol: value.protocol,
  }
}

// Protocol paths are literal API names and are not translated.
export const CACHE_AUDIT_PROTOCOL_OPTIONS: { value: string; label: string }[] =
  [
    { value: 'messages', label: '/v1/messages' },
    { value: 'chat_completions', label: '/v1/chat/completions' },
    { value: 'responses', label: '/v1/responses' },
  ]

export const CACHE_AUDIT_STATUS_LABELS: Record<CacheAuditStatus, string> = {
  affected: 'Cache counted as input',
  review: 'Needs review',
  no_evidence: 'Insufficient evidence',
  not_affected: 'Not affected',
  refund: 'Refund',
}

export const CACHE_AUDIT_REASON_LABELS: Record<string, string> = {
  cache_read_in_input: 'Cache reads were also reported in input_tokens',
  cache_write_present:
    'Cache writes present; logs cannot tell whether they were duplicated',
  legacy_cache_split:
    'Legacy cache split usage; compare with the original response',
  missing_request_path: 'Log has no request path',
  missing_conversion_chain: 'Log has no protocol conversion record',
  invalid_metadata: 'Log metadata is unreadable',
  not_messages_endpoint: 'Not a /v1/messages request',
  native_claude_upstream: 'Native Claude upstream',
  anthropic_usage_preserved: 'Upstream usage already followed Anthropic',
  no_cache: 'No cache usage',
  fixed_converter: 'After the fix went live',
  estimated_usage: 'Usage was estimated locally, not reported by upstream',
}

export const CACHE_AUDIT_PRICE_ISSUE_LABELS: Record<string, string> = {
  missing_price: 'Historical price missing',
  missing_cache_price: 'Historical cache price missing',
  tiered_billing: 'Tiered billing expression',
  per_call_price: 'Per-call pricing',
  unsupported_surcharge: 'Includes image, audio or tool surcharges',
}

// Per-model token report for customers; the column names stay in Chinese
// like the server export because the file is sent to customers as-is.
export function cacheAuditModelTokenCsv(report: CacheAuditReport) {
  const header = [
    '模型',
    '受影响请求数',
    '旧返回input_tokens',
    '正确input_tokens',
    '多算token(缓存被当作输入)',
    '待核对上限',
    '普通输入',
    '缓存读取',
    '缓存写入',
    '输出',
    '官价普通输入单价(每百万)',
    '官价缓存读取单价(每百万)',
    '多算金额-按官价',
    '多算金额-按客户价',
  ]
  const rows = [
    ...report.by_model.filter((row) => row.duplicate_tokens_max > 0),
    { ...report.total, key: '合计' },
  ].map((row) => [
    row.key,
    row.affected_requests,
    row.affected_reported_input,
    row.affected_correct_input,
    row.duplicate_tokens,
    row.duplicate_tokens_max - row.duplicate_tokens,
    row.tokens.input,
    row.tokens.cache_read,
    row.tokens.cache_write,
    row.tokens.output,
    row.input_price_per_million ?? '',
    row.cache_read_price_per_million ?? '',
    row.compensation.list,
    row.compensation.downstream ?? '',
  ])
  return [header, ...rows]
    .map((cells) =>
      cells.map((cell) => `"${String(cell).replaceAll('"', '""')}"`).join(',')
    )
    .join('\r\n')
}

export function formatTokens(value: number) {
  return value.toLocaleString('en-US')
}
