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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import {
  exportCacheAudit,
  runCacheAudit,
  searchCacheAuditKeys,
  type CacheAuditReport,
  type CacheAuditTotals,
} from '../cache-audit-api'
import { CacheAuditPanel } from '../components/cache-audit-panel'
import { cacheAuditRequest, cacheAuditSchema } from '../lib/cache-audit'

vi.mock('../cache-audit-api', () => ({
  runCacheAudit: vi.fn(),
  searchCacheAuditKeys: vi.fn(),
  exportCacheAudit: vi.fn(),
}))

afterEach(() => vi.clearAllMocks())

const tokens = {
  total_input: 4921,
  input: 96,
  cache_read: 4825,
  cache_write: 0,
  output: 86,
}
const totals: CacheAuditTotals = {
  key: 'total',
  requests: 1,
  affected_requests: 1,
  review_requests: 0,
  no_evidence_requests: 0,
  priced_requests: 1,
  tokens,
  duplicate_tokens: 4825,
  duplicate_tokens_max: 4825,
  actual_amount: '0.010086',
  list_amount: '0.020170',
  discounted_amount: '0.010085',
  compensation: {
    list: '0.096500',
    group: '0.048250',
    list_max: '0.096500',
    group_max: '0.048250',
  },
  unpriced_duplicate_tokens: 0,
}
const report: CacheAuditReport = {
  request: {
    user_id: 4,
    token_id: 0,
    start_at: 0,
    end_at: 86400,
    model: '',
    fixed_at: 0,
    status: '',
  },
  currency: { code: 'USD', symbol: '$' },
  total: totals,
  by_day: [{ ...totals, key: '2026-10-16' }],
  by_model: [{ ...totals, key: 'gpt-5' }],
  by_token: [{ ...totals, key: 'prod (#3)' }],
  by_user: [{ ...totals, key: 'customer (#4)' }],
  rows: [
    {
      log_id: 1,
      request_id: 'req-cache-1',
      created_at: 1760600000,
      type: 2,
      user_id: 4,
      username: 'customer',
      token_id: 3,
      token_name: 'prod',
      model: 'gpt-5',
      group: 'default',
      is_stream: false,
      request_path: '/v1/messages',
      usage_semantic: 'openai',
      status: 'affected',
      reason: 'cache_read_in_input',
      group_ratio: '0.5',
      actual_quota: 5043,
      actual_amount: '0.010086',
      reported_input_tokens: 4921,
      correct_input_tokens: 96,
      duplicate_tokens: 4825,
      duplicate_tokens_max: 4825,
      tokens,
      list_cost: {
        input: '0.001920',
        cache_read: '0.009650',
        cache_write: '0.000000',
        output: '0.008600',
        total: '0.020170',
      },
      discounted_cost: {
        input: '0.000960',
        cache_read: '0.004825',
        cache_write: '0.000000',
        output: '0.004300',
        total: '0.010085',
      },
    },
  ],
  truncated: false,
}

test('audit periods use Asia/Shanghai calendar days and reject over 93 days', () => {
  const value = {
    start: '2026-10-01',
    end: '2026-10-16',
    tokenId: 3,
    model: ' gpt-5 ',
    fixedAt: '2026-10-16T22:00',
    status: 'affected' as const,
  }
  expect(cacheAuditRequest(4, value)).toEqual({
    user_id: 4,
    token_id: 3,
    start_at: Date.parse('2026-10-01T00:00:00+08:00') / 1000,
    end_at: Date.parse('2026-10-17T00:00:00+08:00') / 1000,
    model: 'gpt-5',
    fixed_at: Date.parse('2026-10-16T22:00:00+08:00') / 1000,
    status: 'affected',
  })
  expect(cacheAuditSchema.safeParse(value).success).toBe(true)
  expect(
    cacheAuditSchema.safeParse({ ...value, start: '2026-07-01' }).success
  ).toBe(false)
  expect(
    cacheAuditSchema.safeParse({ ...value, end: '2026-09-30' }).success
  ).toBe(false)
})

test('runs the audit for the selected customer and shows duplicated cache tokens', async () => {
  vi.mocked(searchCacheAuditKeys).mockResolvedValue([
    { id: 3, user_id: 4, name: 'prod', deleted: false },
    { id: 5, user_id: 4, name: 'old', deleted: true },
  ])
  vi.mocked(runCacheAudit).mockResolvedValue(report)
  vi.mocked(exportCacheAudit).mockResolvedValue()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <CacheAuditPanel userId={4} />
    </QueryClientProvider>
  )
  expect(screen.getByRole('button', { name: 'Export CSV' })).toBeDisabled()
  await screen.findByRole('option', { name: 'old (#5) (deleted)' })
  fireEvent.change(screen.getByLabelText('API key'), { target: { value: '3' } })
  fireEvent.click(screen.getByRole('button', { name: 'Run audit' }))

  await waitFor(() => expect(runCacheAudit).toHaveBeenCalledTimes(1))
  expect(vi.mocked(runCacheAudit).mock.calls[0][0]).toMatchObject({
    user_id: 4,
    token_id: 3,
  })
  expect(await screen.findByText('req-cache-1')).toBeInTheDocument()
  expect(screen.getAllByText('4,825').length).toBeGreaterThan(0)
  expect(screen.getByText('$0.020170', { selector: 'p' })).toBeInTheDocument()
  expect(
    screen.getByText('Cache reads were also reported in input_tokens')
  ).toBeInTheDocument()

  expect(screen.getByText('How much to compensate')).toBeInTheDocument()
  vi.mocked(runCacheAudit).mockResolvedValue({
    ...report,
    total: {
      ...totals,
      compensation: {
        ...totals.compensation,
        downstream: '0.014475',
        downstream_max: '0.014475',
      },
    },
  })
  fireEvent.change(screen.getByPlaceholderText('0.00'), {
    target: { value: '3' },
  })
  fireEvent.click(
    screen.getByRole('button', { name: 'Recalculate compensation' })
  )
  await waitFor(() => expect(runCacheAudit).toHaveBeenCalledTimes(2))
  expect(vi.mocked(runCacheAudit).mock.calls[1][0]).toMatchObject({
    token_id: 3,
    downstream_input_prices: { 'gpt-5': '3' },
  })
  expect((await screen.findAllByText('$0.014475')).length).toBeGreaterThan(0)

  fireEvent.click(screen.getByRole('button', { name: 'Export CSV' }))
  await waitFor(() =>
    expect(exportCacheAudit).toHaveBeenCalledWith(
      expect.objectContaining({ user_id: 4, token_id: 3 }),
      'csv'
    )
  )
})
