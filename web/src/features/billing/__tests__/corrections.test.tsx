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

function mountPanel() {
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
      <CorrectionPanel userId={4} actorId={1} />
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
  fireEvent.click(await screen.findByRole('button', { name: /saved-preview/ }))
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
