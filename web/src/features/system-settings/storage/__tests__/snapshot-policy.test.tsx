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
  getRequestSnapshotStatus,
  getRequestSnapshotStoragePolicy,
  updateRequestSnapshotStoragePolicy,
} from '../api'
import { RequestSnapshotManagement } from '../request-snapshot-management'
import {
  snapshotPolicy,
  snapshotProfile,
  snapshotStatus,
} from './snapshot-fixtures'

vi.mock('../api', () => ({
  getRequestSnapshotStoragePolicy: vi.fn(),
  updateRequestSnapshotStoragePolicy: vi.fn(),
  getRequestSnapshotStatus: vi.fn(),
  startRequestSnapshotMigration: vi.fn(),
  startRequestSnapshotCleanup: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getRequestSnapshotStoragePolicy).mockResolvedValue({
    success: true,
    data: snapshotPolicy(),
  })
  vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
    success: true,
    data: snapshotStatus(),
  })
  vi.mocked(updateRequestSnapshotStoragePolicy).mockResolvedValue({
    success: true,
    data: snapshotPolicy(),
  })
})
afterEach(() => queryClient?.clear())

function renderManagement() {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RequestSnapshotManagement profiles={[snapshotProfile]} profilesReady />
    </QueryClientProvider>
  )
}

describe('request snapshot policy', () => {
  test('saves the selected profile, queue budget and permanent retention using API units', async () => {
    renderManagement()
    const form = within(
      await screen.findByRole('region', { name: 'Snapshot persistence' })
    )
    expect(form.getByLabelText('Maximum snapshot size (MiB)')).toHaveValue(128)
    expect(form.getByLabelText('Local queue budget (MiB)')).toHaveValue(2048)
    expect(form.getByLabelText('Snapshot retention (days)')).toHaveValue(0)
    expect(
      form.getByRole('option', { name: 'Private audit archive' })
    ).toHaveValue('7')
    expect(
      form.queryByLabelText('Signed URL lifetime (hours)')
    ).not.toBeInTheDocument()
    fireEvent.change(form.getByLabelText('Maximum snapshot size (MiB)'), {
      target: { value: '64' },
    })
    fireEvent.change(form.getByLabelText('Object prefix'), {
      target: { value: '/audit/custom/' },
    })
    fireEvent.click(form.getByRole('button', { name: 'Save snapshot policy' }))
    await waitFor(() =>
      expect(updateRequestSnapshotStoragePolicy).toHaveBeenCalledWith({
        enabled: true,
        storage_profile_id: 7,
        object_prefix: 'audit/custom',
        signed_url_ttl_seconds: 0,
        retention_seconds: 0,
        max_file_bytes: 64 * 1024 * 1024,
        max_total_bytes: 2 * 1024 * 1024 * 1024,
        max_files: 2,
        allowed_mime_types: 'application/json',
      })
    )
    expect(toast.success).toHaveBeenCalledWith('Storage policy saved')
  })

  test('allows saving a disabled policy without a storage profile and disables migration', async () => {
    vi.mocked(getRequestSnapshotStoragePolicy).mockResolvedValue({
      success: true,
      data: snapshotPolicy({ enabled: false, storage_profile_id: 0 }),
    })
    renderManagement()
    const form = within(
      await screen.findByRole('region', { name: 'Snapshot persistence' })
    )
    expect(
      form.getByRole('switch', { name: 'Enable snapshot persistence' })
    ).not.toBeChecked()
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeDisabled()
    fireEvent.click(form.getByRole('button', { name: 'Save snapshot policy' }))
    await waitFor(() =>
      expect(updateRequestSnapshotStoragePolicy).toHaveBeenCalledWith(
        expect.objectContaining({ enabled: false, storage_profile_id: 0 })
      )
    )
  })

  test('rejects enabling persistence without an active destination with an accessible field error', async () => {
    vi.mocked(getRequestSnapshotStoragePolicy).mockResolvedValue({
      success: true,
      data: snapshotPolicy({ enabled: false, storage_profile_id: 0 }),
    })
    renderManagement()
    const form = within(
      await screen.findByRole('region', { name: 'Snapshot persistence' })
    )
    fireEvent.click(
      form.getByRole('switch', { name: 'Enable snapshot persistence' })
    )
    fireEvent.click(form.getByRole('button', { name: 'Save snapshot policy' }))
    expect(await form.findByRole('alert')).toHaveTextContent(
      'Select an enabled storage profile'
    )
    expect(
      form.getByRole('combobox', { name: 'Storage profile' })
    ).toHaveAttribute('aria-invalid', 'true')
    expect(updateRequestSnapshotStoragePolicy).not.toHaveBeenCalled()
  })

  test('shows the server failure while retaining unsaved configuration for retry', async () => {
    vi.mocked(updateRequestSnapshotStoragePolicy).mockResolvedValue({
      success: false,
      message: 'Archive destination is unavailable',
      data: snapshotPolicy(),
    })
    renderManagement()
    const form = within(
      await screen.findByRole('region', { name: 'Snapshot persistence' })
    )
    fireEvent.change(form.getByLabelText('Object prefix'), {
      target: { value: 'audit/retry' },
    })
    fireEvent.click(form.getByRole('button', { name: 'Save snapshot policy' }))
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Archive destination is unavailable'
      )
    )
    expect(form.getByLabelText('Object prefix')).toHaveValue('audit/retry')
    expect(
      form.getByRole('button', { name: 'Save snapshot policy' })
    ).toBeEnabled()
  })

  test('disables configuration controls while a save is pending', async () => {
    let finishSave: (
      value: Awaited<ReturnType<typeof updateRequestSnapshotStoragePolicy>>
    ) => void = () => undefined
    vi.mocked(updateRequestSnapshotStoragePolicy).mockImplementation(
      () =>
        new Promise((resolve) => {
          finishSave = resolve
        })
    )
    renderManagement()
    const form = within(
      await screen.findByRole('region', { name: 'Snapshot persistence' })
    )
    fireEvent.click(form.getByRole('button', { name: 'Save snapshot policy' }))
    expect(
      await form.findByRole('button', { name: 'Saving...' })
    ).toBeDisabled()
    expect(
      form.getByRole('switch', { name: 'Enable snapshot persistence' })
    ).toHaveAttribute('aria-disabled', 'true')
    expect(form.getByLabelText('Object prefix')).toBeDisabled()
    finishSave({ success: true, data: snapshotPolicy() })
    await waitFor(() =>
      expect(
        form.getByRole('button', { name: 'Save snapshot policy' })
      ).toBeEnabled()
    )
  })
})
