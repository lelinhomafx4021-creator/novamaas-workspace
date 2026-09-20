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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  backfillCostAccounting,
  repriceCostAccounting,
} from '../cost-accounting-api'
import { CostAccountingSection } from '../cost-accounting-section'
import type {
  CostAccountingBackfillResult,
  CostAccountingRepriceResult,
} from '../cost-accounting-types'

vi.mock('../cost-accounting-api', () => ({
  backfillCostAccounting: vi.fn(),
  repriceCostAccounting: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const previewResult: CostAccountingBackfillResult = {
  batch_id: 'backfill-batch-1',
  scanned: 4,
  ready: 2,
  existing: 1,
  unresolved: 1,
  applied: 0,
  next_offset: 4,
  has_more: true,
  cost_quota: 12500,
  revenue_quota: 25000,
}

const repricePreviewResult: CostAccountingRepriceResult = {
  batch_id: 'reprice-batch-1',
  scanned: 3,
  changed: 2,
  unchanged: 1,
  applied: 0,
  next_offset: 3,
  has_more: false,
  current_cost_quota: 25000,
  new_cost_quota: 20000,
  delta_cost_quota: -5000,
}

let queryClient: QueryClient

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(backfillCostAccounting).mockResolvedValue(previewResult)
  vi.mocked(repriceCostAccounting).mockResolvedValue(repricePreviewResult)
})

afterEach(() => queryClient?.clear())

function renderSection() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <CostAccountingSection />
    </QueryClientProvider>
  )
}

function backfillCard() {
  const title = screen.getByText('Historical cost backfill')
  const card = title.closest('[data-slot="card"]')
  if (!card) throw new Error('Backfill card not found')
  return within(card as HTMLElement)
}

function fillBackfillRange() {
  const card = backfillCard()
  fireEvent.change(card.getByLabelText('Start Time'), {
    target: { value: '2026-09-01T00:00' },
  })
  fireEvent.change(card.getByLabelText('End Time'), {
    target: { value: '2026-09-02T12:30' },
  })
  fireEvent.change(card.getByLabelText('Channel ID'), {
    target: { value: '18' },
  })
  fireEvent.change(card.getByLabelText('Upstream cost discount'), {
    target: { value: '0.765432' },
  })
  fireEvent.change(card.getByLabelText('Batch size'), {
    target: { value: '250' },
  })
  return card
}

function repriceCard() {
  const title = screen.getByText('Reprice cost snapshots')
  const card = title.closest('[data-slot="card"]')
  if (!card) throw new Error('Reprice card not found')
  return within(card as HTMLElement)
}

describe('cost accounting operations', () => {
  test('requires preview and confirmation before applying a backfill batch', async () => {
    renderSection()
    const card = fillBackfillRange()
    const applyButton = card.getByRole('button', { name: 'Apply this batch' })
    expect(applyButton).toBeDisabled()

    fireEvent.click(card.getByRole('button', { name: 'Preview' }))

    await waitFor(() =>
      expect(backfillCostAccounting).toHaveBeenCalledWith({
        start_timestamp: 1788192000,
        end_timestamp: 1788323400,
        channel_id: 18,
        cost_discount: '0.765432',
        offset: 0,
        limit: 250,
        apply: false,
        batch_id: '',
      })
    )
    expect(await card.findByText(/backfill-batch-1/)).toBeVisible()
    expect(card.getByText('Turnover')).toBeVisible()
    expect(applyButton).toBeEnabled()

    fireEvent.click(applyButton)
    expect(screen.getByRole('alertdialog')).toHaveTextContent(
      '2 immutable cost snapshots'
    )
    expect(backfillCostAccounting).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Apply snapshots' }))

    await waitFor(() =>
      expect(backfillCostAccounting).toHaveBeenLastCalledWith(
        expect.objectContaining({
          apply: true,
          batch_id: 'backfill-batch-1',
          offset: 0,
        })
      )
    )
  })

  test('invalidates the preview when an accounting input changes', async () => {
    renderSection()
    const card = fillBackfillRange()
    fireEvent.click(card.getByRole('button', { name: 'Preview' }))
    await card.findByText(/backfill-batch-1/)

    fireEvent.change(card.getByLabelText('Upstream cost discount'), {
      target: { value: '0.700000' },
    })

    expect(
      card.getByText(
        'Inputs changed after this preview. Run a new preview before applying.'
      )
    ).toBeVisible()
    expect(
      card.getByRole('button', { name: 'Apply this batch' })
    ).toBeDisabled()
  })

  test('carries an all-existing backfill selection into cost recalculation', async () => {
    vi.mocked(backfillCostAccounting).mockResolvedValue({
      ...previewResult,
      scanned: 340,
      ready: 0,
      existing: 340,
      unresolved: 0,
      has_more: false,
      cost_quota: 0,
      revenue_quota: 0,
    })
    renderSection()
    const card = fillBackfillRange()

    fireEvent.click(card.getByRole('button', { name: 'Preview' }))

    const repriceButton = await card.findByRole('button', {
      name: 'Continue to cost recalculation',
    })
    expect(
      card.getByRole('button', { name: 'Apply this batch' })
    ).toBeDisabled()

    fireEvent.click(repriceButton)

    const target = repriceCard()
    expect(target.getByLabelText('Start Time')).toHaveValue('2026-09-01T00:00')
    expect(target.getByLabelText('End Time')).toHaveValue('2026-09-02T12:30')
    expect(target.getByLabelText('Channel ID')).toHaveValue(18)
    expect(target.getByLabelText('Upstream cost discount')).toHaveValue(
      '0.765432'
    )
    expect(target.getByLabelText('Batch size')).toHaveValue(250)
  })

  test('shows the backend reason when preview fails', async () => {
    vi.mocked(backfillCostAccounting).mockRejectedValue({
      isAxiosError: true,
      message: 'Request failed with status code 400',
      response: {
        data: {
          success: false,
          message: 'invalid cost accounting backfill range',
        },
      },
    })
    renderSection()
    const card = fillBackfillRange()
    fireEvent.click(card.getByRole('button', { name: 'Preview' }))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'invalid cost accounting backfill range'
      )
    )
  })

  test('requires a reason and confirmation before appending cost adjustments', async () => {
    renderSection()
    const card = repriceCard()
    fireEvent.change(card.getByLabelText('Start Time'), {
      target: { value: '2026-09-01T00:00' },
    })
    fireEvent.change(card.getByLabelText('End Time'), {
      target: { value: '2026-09-02T12:30' },
    })
    fireEvent.change(card.getByLabelText('Channel ID'), {
      target: { value: '18' },
    })
    fireEvent.change(card.getByLabelText('Upstream cost discount'), {
      target: { value: '0.800000' },
    })

    fireEvent.click(card.getByRole('button', { name: 'Preview' }))
    expect(await card.findByText('Adjustment reason is required')).toBeVisible()
    expect(repriceCostAccounting).not.toHaveBeenCalled()

    fireEvent.change(card.getByLabelText('Adjustment reason'), {
      target: { value: 'Supplier contract changed' },
    })
    fireEvent.click(card.getByRole('button', { name: 'Preview' }))

    await waitFor(() =>
      expect(repriceCostAccounting).toHaveBeenCalledWith(
        expect.objectContaining({
          apply: false,
          batch_id: '',
          reason: 'Supplier contract changed',
        })
      )
    )
    expect(card.getByRole('button', { name: 'Apply this batch' })).toBeEnabled()

    fireEvent.click(card.getByRole('button', { name: 'Apply this batch' }))
    expect(screen.getByRole('alertdialog')).toHaveTextContent(
      'original snapshots remain unchanged'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Apply adjustments' }))

    await waitFor(() =>
      expect(repriceCostAccounting).toHaveBeenLastCalledWith(
        expect.objectContaining({
          apply: true,
          batch_id: 'reprice-batch-1',
          reason: 'Supplier contract changed',
        })
      )
    )
  })
})
