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
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { CorrectionPanel } from '../components/correction-panel'
import {
  previewCorrection,
  getCorrection,
  correctionGroups,
  listCorrections,
  actOnCorrection,
  type CorrectionBatch,
} from '../correction-api'

const verificationStart = vi.hoisted(() => vi.fn())

vi.mock('../correction-api', () => ({
  previewCorrection: vi.fn(),
  correctionGroups: vi.fn(),
  listCorrections: vi.fn(),
  actOnCorrection: vi.fn(),
  getCorrection: vi.fn(),
}))
vi.mock('@/features/auth/secure-verification', () => ({
  useSecureVerification: () => ({
    state: {},
    methods: {},
    open: false,
    isLoading: false,
    startVerification: verificationStart,
  }),
  SecureVerificationDialog: () => null,
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const batch: CorrectionBatch = {
  id: 'preview-batch',
  user_id: 4,
  created_by: 1,
  created_at: 100,
  start_at: 100,
  end_at: 200,
  reason: 'Wrong group',
  audit_logged_at: 0,
  reversal_audit_logged_at: 0,
  models: '["wan-prime"]',
  target_group: 'wan-prime',
  target_rate: '0.77',
  status: 'preview',
  can_apply: true,
  net_delta: 70715,
  charge_delta: 141430,
  refund_delta: 70715,
  sha256: 'frozen-digest',
  expires_at: 9999999999,
  rows: [],
}

beforeEach(() => {
  vi.resetAllMocks()
  verificationStart.mockImplementation(
    (call: (proof: string) => Promise<unknown>) => call('verified-proof')
  )
  vi.mocked(correctionGroups).mockResolvedValue([
    { group: 'wan-prime', rate: '0.77' },
  ])
  vi.mocked(listCorrections).mockResolvedValue([])
  vi.mocked(previewCorrection).mockResolvedValue(batch)
  vi.mocked(actOnCorrection).mockResolvedValue({ ...batch, status: 'applied' })
})

function mountPanel(canManage = true) {
  return render(
    <QueryClientProvider
      client={
        new QueryClient({
          defaultOptions: {
            queries: { retry: false },
            mutations: { retry: false },
          },
        })
      }
    >
      <CorrectionPanel userId={4} actorId={1} canManage={canManage} />
    </QueryClientProvider>
  )
}

async function fillPreview() {
  await screen.findByRole('option', { name: 'wan-prime / 77.00%' })
  fireEvent.change(screen.getByLabelText('Target billing group'), {
    target: { value: 'wan-prime' },
  })
  fireEvent.change(screen.getByLabelText('Models to correct'), {
    target: { value: 'wan-prime' },
  })
  fireEvent.change(screen.getByLabelText('Adjustment reason'), {
    target: { value: 'Wrong group configuration' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Preview adjustment' }))
  await screen.findByText('frozen-digest')
}

test('money moves only after reviewing a current preview and confirming the account', async () => {
  mountPanel()
  expect(
    screen.queryByRole('button', { name: 'Apply adjustment' })
  ).not.toBeInTheDocument()
  await fillPreview()
  expect(actOnCorrection).not.toHaveBeenCalled()
  expect(
    screen.getByRole('button', { name: 'Apply adjustment' })
  ).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Apply adjustment' }))
  await waitFor(() =>
    expect(actOnCorrection).toHaveBeenCalledWith(
      'preview-batch',
      {
        action: 'apply',
        sha256: 'frozen-digest',
        confirm_user_id: 4,
        confirm_net_delta: 70715,
        reason: '',
      },
      'verified-proof'
    )
  )
})

test('changing the selection invalidates its preview', async () => {
  mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  fireEvent.change(screen.getByLabelText('Models to correct'), {
    target: { value: 'different-model' },
  })
  expect(
    screen.getByRole('button', { name: 'Apply adjustment' })
  ).toBeDisabled()
  expect(
    screen.getByText('Selection changed. Preview again.')
  ).toBeInTheDocument()
})

test('blocked evidence disables execution even with account confirmation', async () => {
  vi.mocked(previewCorrection).mockResolvedValue({ ...batch, can_apply: false })
  mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  expect(
    screen.getByRole('button', { name: 'Apply adjustment' })
  ).toBeDisabled()
  expect(actOnCorrection).not.toHaveBeenCalled()
})

test('reversing an applied batch requires a reason and confirms the opposite amount', async () => {
  vi.mocked(previewCorrection).mockResolvedValue({
    ...batch,
    status: 'applied',
  })
  vi.mocked(actOnCorrection).mockResolvedValue({ ...batch, status: 'reversed' })
  mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  expect(
    screen.getByRole('button', { name: 'Reverse adjustment' })
  ).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Reversal reason'), {
    target: { value: 'Undo reviewed correction' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Reverse adjustment' }))
  await waitFor(() =>
    expect(actOnCorrection).toHaveBeenCalledWith(
      'preview-batch',
      {
        action: 'reverse',
        sha256: 'frozen-digest',
        confirm_user_id: 4,
        confirm_net_delta: -70715,
        reason: 'Undo reviewed correction',
      },
      'verified-proof'
    )
  )
})

test('leaving the account during verification prevents the saved action from executing', async () => {
  let callback: ((proof: string) => Promise<unknown>) | undefined
  verificationStart.mockImplementation(
    (call: (proof: string) => Promise<unknown>) => {
      callback = call
      return Promise.resolve(true)
    }
  )
  const panel = mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Apply adjustment' }))
  expect(callback).toBeDefined()
  panel.unmount()
  if (!callback) throw new Error('Verification callback missing')
  await expect(callback('verified-proof')).rejects.toThrow(
    'Selection changed. Preview again.'
  )
  expect(actOnCorrection).not.toHaveBeenCalled()
})

test('loading a saved preview restores its selection without a false changed-selection warning', async () => {
  const saved = {
    ...batch,
    id: 'saved-preview',
    start_at: 1788192000,
    end_at: 1790784000,
  }
  vi.mocked(listCorrections).mockResolvedValue([saved])
  vi.mocked(getCorrection).mockResolvedValue(saved)
  mountPanel()
  fireEvent.click(await screen.findByRole('button', { name: 'View' }))
  await screen.findByText('frozen-digest')
  expect(screen.getByLabelText('Start date')).toHaveValue('2026-09-01')
  expect(screen.getByLabelText('End date')).toHaveValue('2026-09-30')
  expect(screen.getByLabelText('Models to correct')).toHaveValue('wan-prime')
  expect(
    screen.queryByText('Selection changed. Preview again.')
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  expect(screen.getByRole('button', { name: 'Apply adjustment' })).toBeEnabled()
})

test('trimming model names and reasons does not invalidate the preview', async () => {
  mountPanel()
  await screen.findByRole('option', { name: 'wan-prime / 77.00%' })
  fireEvent.change(screen.getByLabelText('Target billing group'), {
    target: { value: 'wan-prime' },
  })
  fireEvent.change(screen.getByLabelText('Models to correct'), {
    target: { value: '  wan-prime  \n' },
  })
  fireEvent.change(screen.getByLabelText('Adjustment reason'), {
    target: { value: '  Wrong group configuration  ' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Preview adjustment' }))
  await screen.findByText('frozen-digest')
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  expect(
    screen.queryByText('Selection changed. Preview again.')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Apply adjustment' })).toBeEnabled()
  expect(previewCorrection).toHaveBeenCalledWith(
    expect.objectContaining({
      models: ['wan-prime'],
      reason: 'Wrong group configuration',
    })
  )
})

test('finance viewers can open adjustment details without controls that move money', async () => {
  vi.mocked(listCorrections).mockResolvedValue([
    { ...batch, status: 'applied' },
  ])
  vi.mocked(getCorrection).mockResolvedValue({ ...batch, status: 'applied' })
  mountPanel(false)
  fireEvent.click(await screen.findByRole('button', { name: 'View' }))
  await screen.findByText('frozen-digest')
  expect(
    screen.queryByRole('button', { name: 'Preview adjustment' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByLabelText('Type the account ID to confirm')
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Reverse adjustment' })
  ).not.toBeInTheDocument()
  expect(correctionGroups).not.toHaveBeenCalled()
  expect(actOnCorrection).not.toHaveBeenCalled()
})

test('adjustment history uses separate table columns for long IDs, status, amounts and actions', async () => {
  const longId = `correction-${'a'.repeat(64)}`
  vi.mocked(listCorrections).mockResolvedValue([
    { ...batch, id: longId },
    { ...batch, id: 'applied-batch', status: 'applied', net_delta: -70715 },
  ])
  mountPanel()
  const table = await screen.findByRole('table', { name: 'Adjustment history' })
  await within(table).findByText(longId)
  expect(
    within(table).getByRole('columnheader', { name: 'Batch ID' })
  ).toBeVisible()
  expect(
    within(table).getByRole('columnheader', { name: 'Status' })
  ).toBeVisible()
  expect(
    within(table).getByRole('columnheader', {
      name: 'Net debit (negative means credit)',
    })
  ).toBeVisible()
  expect(within(table).getAllByRole('button', { name: 'View' })).toHaveLength(2)
  expect(within(table).getByText(longId)).toHaveClass(
    'break-all',
    'whitespace-normal'
  )
  expect(table.parentElement).toHaveClass('overflow-x-auto')
})

test('empty adjustment history displays an explicit empty state', async () => {
  mountPanel(false)
  expect(await screen.findByText('No data')).toBeVisible()
  expect(screen.queryByRole('button', { name: 'View' })).not.toBeInTheDocument()
})

test('failed adjustment history can be refreshed', async () => {
  vi.mocked(listCorrections).mockRejectedValueOnce(new Error('Unavailable'))
  mountPanel(false)
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Failed to load data'
  )
  fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
  expect(await screen.findByText('No data')).toBeVisible()
})

test('pending adjustment history shows loading until an empty response arrives', async () => {
  let finish: ((items: CorrectionBatch[]) => void) | undefined
  vi.mocked(listCorrections).mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve
      })
  )
  mountPanel(false)
  expect(screen.getByText('Loading...')).toBeVisible()
  expect(screen.queryByText('No data')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled()
  await act(async () => finish?.([]))
  expect(await screen.findByText('No data')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Refresh' })).toBeEnabled()
})

test('model pricing adjustment previews current pricing without changing the billing group', async () => {
  mountPanel()
  fireEvent.change(screen.getByLabelText('Adjustment type'), {
    target: { value: 'model_pricing' },
  })
  expect(
    screen.queryByLabelText('Target billing group')
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Models to correct'), {
    target: { value: 'doubao-seedance-2-5' },
  })
  fireEvent.change(screen.getByLabelText('Adjustment reason'), {
    target: { value: 'Recalculate resolution pricing' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Preview adjustment' }))
  await screen.findByText('frozen-digest')
  expect(previewCorrection).toHaveBeenCalledWith(
    expect.objectContaining({
      mode: 'model_pricing',
      target_group: '',
      models: ['doubao-seedance-2-5'],
    })
  )
})

test('switching adjustment type invalidates a previous billing group preview', async () => {
  mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  fireEvent.change(screen.getByLabelText('Adjustment type'), {
    target: { value: 'model_pricing' },
  })
  expect(
    screen.getByRole('button', { name: 'Apply adjustment' })
  ).toBeDisabled()
  expect(screen.getByText('Selection changed. Preview again.')).toBeVisible()
})

test('saved model pricing preview restores its mode and shows the existing billing group', async () => {
  const saved = {
    ...batch,
    mode: 'model_pricing' as const,
    target_group: '',
    target_rate: '',
    start_at: 1788192000,
    end_at: 1790784000,
  }
  vi.mocked(listCorrections).mockResolvedValue([saved])
  vi.mocked(getCorrection).mockResolvedValue(saved)
  mountPanel()
  fireEvent.click(await screen.findByRole('button', { name: 'View' }))
  await screen.findByText('frozen-digest')
  expect(screen.getByLabelText('Adjustment type')).toHaveValue('model_pricing')
  expect(
    screen.queryByLabelText('Target billing group')
  ).not.toBeInTheDocument()
  expect(
    screen.queryByText('Selection changed. Preview again.')
  ).not.toBeInTheDocument()
})

test('fully refunded adjustments with a zero net debit can be confirmed', async () => {
  vi.mocked(previewCorrection).mockResolvedValue({
    ...batch,
    mode: 'group_rate',
    net_delta: 0,
    charge_delta: 20,
    refund_delta: 20,
    can_apply: true,
  })
  mountPanel()
  await fillPreview()
  fireEvent.change(screen.getByLabelText('Type the account ID to confirm'), {
    target: { value: '4' },
  })
  expect(screen.getByRole('button', { name: 'Apply adjustment' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Apply adjustment' }))
  await waitFor(() =>
    expect(actOnCorrection).toHaveBeenCalledWith(
      'preview-batch',
      expect.objectContaining({ confirm_net_delta: 0 }),
      'verified-proof'
    )
  )
})

test('an effective zero amount is displayed as zero while original evidence remains visible', async () => {
  const historical = {
    source_entry_id: 10,
    model_name: 'previously-free',
    posted_at: 100,
    original_group: 'old',
    original_rate: '1',
    original_quota: 500000,
    corrected_quota: 0,
    delta: 0,
    blocked: '',
  }
  vi.mocked(listCorrections).mockResolvedValue([
    { ...batch, mode: 'model_pricing' },
  ])
  vi.mocked(getCorrection).mockResolvedValue({
    ...batch,
    mode: 'model_pricing',
    rows: [historical],
  })
  mountPanel()
  fireEvent.click(await screen.findByRole('button', { name: 'View' }))
  await screen.findByText('frozen-digest')
  const row = screen.getByText('previously-free').closest('tr')
  if (!row) throw new Error('Expected the historical evidence table row')
  const cells = within(row).getAllByRole('cell')
  expect(cells[3]).toHaveTextContent('$1')
  expect(cells[4]).toHaveTextContent('$0')
})
