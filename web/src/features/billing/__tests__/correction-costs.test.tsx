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
import { render, screen, within } from '@testing-library/react'
import { afterEach, beforeEach, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { CorrectionHistory } from '../components/correction-history'
import { CorrectionReview } from '../components/correction-review'
import type { CorrectionBatch } from '../correction-api'

const batch: CorrectionBatch = {
  id: 'platform-cost-preview',
  user_id: 4,
  created_by: 1,
  created_at: 100,
  start_at: 100,
  end_at: 200,
  expires_at: 9999999999,
  reason: 'Recalculate platform cost',
  mode: 'model_pricing',
  models: '["doubao-seedance-2-5"]',
  target_group: '',
  target_rate: '',
  status: 'preview',
  can_apply: true,
  net_delta: 250000,
  charge_delta: 250000,
  refund_delta: 0,
  current_cost_quota: 500000,
  corrected_cost_quota: 600000,
  cost_delta: 100000,
  sha256: 'cost-evidence-digest',
  audit_logged_at: 0,
  reversal_audit_logged_at: 0,
  rows: [
    {
      source_entry_id: 10,
      model_name: 'doubao-seedance-2-5',
      posted_at: 110,
      original_group: 'default',
      original_rate: '1',
      original_quota: 500000,
      effective_quota: 500000,
      corrected_quota: 750000,
      delta: 250000,
      blocked: '',
      current_cost_quota: 500000,
      corrected_cost_quota: 600000,
      cost_delta: 100000,
      cost_discount: '0.8',
      cost_blocked: '',
    },
  ],
}

const originalCurrency = useSystemConfigStore.getState().config.currency
beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7,
    },
  })
})
afterEach(() => {
  useSystemConfigStore.getState().setConfig({ currency: originalCurrency })
})

test('previews distinguish platform costs and profit changes from wallet charges', () => {
  render(<CorrectionReview batch={batch} />)
  const summary = screen.getByRole('region', { name: 'Platform cost changes' })
  expect(summary).toHaveTextContent('Current platform cost¥7')
  expect(summary).toHaveTextContent('Recalculated platform cost¥8.4')
  expect(summary).toHaveTextContent('Platform cost change¥1.4')
  expect(summary).toHaveTextContent('Profit change¥2.1')
  expect(summary).toHaveTextContent(
    'Cost corrections do not add wallet charges.'
  )
  const wallet = screen.getByRole('status', { name: 'Wallet balance change' })
  expect(wallet).toHaveTextContent('¥3.5 will be deducted')
  const table = screen.getByRole('table', { name: 'Adjustment records' })
  expect(
    within(table).getByRole('columnheader', { name: 'Current platform cost' })
  ).toBeVisible()
  expect(
    within(table).getByRole('columnheader', {
      name: 'Recalculated platform cost',
    })
  ).toBeVisible()
  expect(
    within(table).getByRole('columnheader', { name: 'Platform cost change' })
  ).toBeVisible()
  expect(within(table).getByText('0.8x')).toBeVisible()
  expect(table.parentElement).toHaveClass('overflow-x-auto')
})

test('cost-only previews keep the wallet unchanged and show a profit decrease', () => {
  render(
    <CorrectionReview batch={{ ...batch, net_delta: 0, charge_delta: 0 }} />
  )
  expect(
    screen.getByRole('status', { name: 'Wallet balance change' })
  ).toHaveTextContent('Wallet balance unchanged')
  expect(
    screen.getByRole('region', { name: 'Platform cost changes' })
  ).toHaveTextContent('Profit change-¥1.4')
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test.each([
  {
    correctedCost: 500000,
    netDelta: 250000,
    costChange: '¥0',
    profitChange: '¥3.5',
  },
  {
    correctedCost: 600000,
    netDelta: 250000,
    costChange: '¥1.4',
    profitChange: '¥2.1',
  },
  { correctedCost: 500000, netDelta: 0, costChange: '¥0', profitChange: '¥0' },
])(
  'omitted cost deltas retain costs $correctedCost with sales delta $netDelta',
  (example) => {
    const preview: CorrectionBatch = {
      ...batch,
      net_delta: example.netDelta,
      charge_delta: example.netDelta,
      corrected_cost_quota: example.correctedCost,
      rows: [
        {
          ...batch.rows[0],
          corrected_cost_quota: example.correctedCost,
        },
      ],
    }
    delete preview.cost_delta
    delete preview.rows[0].cost_delta
    render(<CorrectionReview batch={preview} />)
    const summary = screen.getByRole('region', {
      name: 'Platform cost changes',
    })
    expect(summary).toHaveTextContent(
      `Platform cost change${example.costChange}`
    )
    expect(summary).toHaveTextContent(`Profit change${example.profitChange}`)
    expect(
      screen.queryByText('Platform costs are not included in this batch.')
    ).not.toBeInTheDocument()
    const row = screen.getByText('#10').closest('tr')
    if (!row) throw new Error('Expected the source row with cost evidence')
    expect(within(row).getAllByRole('cell')[11]).toHaveTextContent(
      example.costChange
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  }
)

test('history distinguishes omitted zero cost deltas from legacy batches without cost evidence', () => {
  const unchangedCost: CorrectionBatch = {
    ...batch,
    corrected_cost_quota: batch.current_cost_quota,
  }
  delete unchangedCost.cost_delta
  render(
    <CorrectionHistory
      items={[
        { ...unchangedCost, status: 'applied' },
        { ...unchangedCost, id: 'reversed-zero-cost', status: 'reversed' },
        {
          ...unchangedCost,
          id: 'legacy-without-cost',
          status: 'applied',
          current_cost_quota: undefined,
          corrected_cost_quota: undefined,
        },
      ]}
      loading={false}
      failed={false}
      refreshing={false}
      disabled={false}
      onRefresh={() => undefined}
      onSelect={() => undefined}
    />
  )
  const table = screen.getByRole('table', { name: 'Adjustment history' })
  for (const id of [batch.id, 'reversed-zero-cost']) {
    const row = within(table).getByText(id).closest('tr')
    if (!row) throw new Error('Expected the zero cost history row')
    const cells = within(row).getAllByRole('cell')
    expect(cells[5]).toHaveTextContent('¥0')
    expect(cells[5]).not.toHaveTextContent('-¥0')
    expect(cells[6]).toHaveTextContent(id === batch.id ? '¥3.5' : '-¥3.5')
    expect(within(row).queryByText('Not included')).not.toBeInTheDocument()
  }
  const legacyRow = within(table).getByText('legacy-without-cost').closest('tr')
  if (!legacyRow) throw new Error('Expected the legacy history row')
  expect(within(legacyRow).getByText('Not included')).toBeVisible()
})

test('reversed batches display the restored cost and opposite cost and profit changes', () => {
  render(<CorrectionReview batch={{ ...batch, status: 'reversed' }} />)
  const summary = screen.getByRole('region', { name: 'Reversal cost changes' })
  expect(summary).toHaveTextContent('Current platform cost¥8.4')
  expect(summary).toHaveTextContent('Recalculated platform cost¥7')
  expect(summary).toHaveTextContent('Platform cost change-¥1.4')
  expect(summary).toHaveTextContent('Profit change-¥2.1')
})

test('signed refund costs and explicit zero costs remain distinct from missing evidence', () => {
  render(
    <CorrectionReview
      batch={{
        ...batch,
        rows: [
          {
            ...batch.rows[0],
            current_cost_quota: -500000,
            corrected_cost_quota: 0,
            cost_delta: 500000,
            cost_discount: '0',
          },
        ],
      }}
    />
  )
  const row = screen.getByText('#10').closest('tr')
  if (!row) throw new Error('Expected the refunded source row')
  expect(within(row).getByText('-¥7')).toBeVisible()
  expect(within(row).getByText('¥0')).toBeVisible()
  expect(within(row).getByText('0x')).toBeVisible()
  expect(within(row).queryByText('Not included')).not.toBeInTheDocument()
})

test.each([
  { reason: 'missing_cost_evidence', message: 'Cost evidence is missing' },
  {
    reason: 'cost_direction_changed',
    message:
      'Adjustment is blocked. Resolve all preview checks and preview again.',
  },
  {
    reason: 'invalid_cost_discount',
    message: 'Channel cost discount is invalid',
  },
  {
    reason: 'cost_recalculation_blocked',
    message: 'Cost cannot be safely recalculated',
  },
  {
    reason: 'future_cost_block',
    message: 'Cost cannot be safely recalculated',
  },
])(
  'blocked cost evidence $reason displays its reason and keeps execution blocked',
  (example) => {
    render(
      <CorrectionReview
        batch={{
          ...batch,
          can_apply: false,
          rows: [
            {
              ...batch.rows[0],
              blocked:
                example.reason === 'cost_direction_changed'
                  ? example.reason
                  : '',
              cost_blocked: example.reason,
            },
          ],
        }}
      />
    )
    expect(screen.getByText(example.message)).toBeVisible()
    expect(screen.queryByText('Ready')).not.toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('prevent execution')
  }
)

test('historical batches without cost fields explicitly identify the missing cost correction', () => {
  render(
    <CorrectionReview
      batch={{
        ...batch,
        current_cost_quota: undefined,
        corrected_cost_quota: undefined,
        cost_delta: undefined,
        rows: [],
      }}
    />
  )
  expect(
    screen.getByText('Platform costs are not included in this batch.')
  ).toBeVisible()
  expect(
    screen.queryByRole('region', { name: 'Platform cost changes' })
  ).not.toBeInTheDocument()
})

test('history displays cost and profit changes and reverses their signs for reversed batches', () => {
  render(
    <CorrectionHistory
      items={[
        { ...batch, status: 'applied' },
        { ...batch, id: 'reversed-cost', status: 'reversed' },
      ]}
      loading={false}
      failed={false}
      refreshing={false}
      disabled={false}
      onRefresh={() => undefined}
      onSelect={() => undefined}
    />
  )
  const table = screen.getByRole('table', { name: 'Adjustment history' })
  expect(
    within(table).getByRole('columnheader', { name: 'Platform cost change' })
  ).toBeVisible()
  expect(
    within(table).getByRole('columnheader', { name: 'Profit change' })
  ).toBeVisible()
  const applied = within(table).getByText(batch.id).closest('tr')
  const reversed = within(table).getByText('reversed-cost').closest('tr')
  if (!applied || !reversed) throw new Error('Expected both cost history rows')
  expect(within(applied).getByText('¥1.4')).toBeVisible()
  expect(within(applied).getByText('¥2.1')).toBeVisible()
  expect(within(reversed).getByText('-¥1.4')).toBeVisible()
  expect(within(reversed).getByText('-¥2.1')).toBeVisible()
})
