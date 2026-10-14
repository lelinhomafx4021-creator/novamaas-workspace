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
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  getRequestSnapshotStatus,
  startRequestSnapshotCleanup,
  startRequestSnapshotMigration,
} from '../api'
import { RequestSnapshotMaintenance } from '../request-snapshot-maintenance'
import type { RequestSnapshotJob } from '../types'
import { snapshotStatus } from './snapshot-fixtures'

vi.mock('../api', () => ({
  getRequestSnapshotStatus: vi.fn(),
  startRequestSnapshotMigration: vi.fn(),
  startRequestSnapshotCleanup: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient
const runningJob: RequestSnapshotJob = {
  status: 'running',
  mode: 'migrate',
  cursor: 5,
  processed: 3,
  failed: 1,
}

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
    success: true,
    data: snapshotStatus(),
  })
  vi.mocked(startRequestSnapshotMigration).mockResolvedValue({
    success: true,
    data: runningJob,
  })
  vi.mocked(startRequestSnapshotCleanup).mockResolvedValue({
    success: true,
    data: { ...runningJob, mode: 'cleanup' },
  })
})
afterEach(() => queryClient?.clear())

function renderMaintenance() {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <RequestSnapshotMaintenance migrationEnabled />
    </QueryClientProvider>
  )
}

describe('snapshot queue and maintenance', () => {
  test('shows the current node queue separately from shared storage settings', async () => {
    renderMaintenance()
    expect(await screen.findByText('dev-node-1')).toBeVisible()
    expect(screen.getByText('/private/audit-spool')).toBeVisible()
    expect(screen.getByText('5.00')).toBeVisible()
    expect(
      screen.getByText(/SNAPSHOT_SPOOL_DIR and SNAPSHOT_WORKERS/)
    ).toBeVisible()
  })

  test('starts migration only after confirmation and refreshes asynchronous completion', async () => {
    renderMaintenance()
    const button = await screen.findByRole('button', {
      name: 'Migrate historical snapshots',
    })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(
      'Database bodies remain until you run verified cleanup.'
    )
    expect(startRequestSnapshotMigration).not.toHaveBeenCalled()
    fireEvent.click(screen.getByRole('button', { name: 'Start migration' }))
    expect(await screen.findByText('Running')).toBeVisible()
    expect(button).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove verified database bodies' })
    ).toBeDisabled()
    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus({
        migration: { ...runningJob, status: 'completed' },
      }),
    })
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(await screen.findByText('Completed')).toBeVisible()
    expect(button).toBeEnabled()
    expect(startRequestSnapshotMigration).toHaveBeenCalledOnce()
  })

  test('requires explicit verified cleanup confirmation and preserves cancellation', async () => {
    renderMaintenance()
    const button = await screen.findByRole('button', {
      name: 'Remove verified database bodies',
    })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(
      'Snapshots that could not be migrated remain in the database.'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(startRequestSnapshotCleanup).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    fireEvent.click(button)
    fireEvent.click(
      await screen.findByRole('button', { name: 'Start verified cleanup' })
    )
    expect(await screen.findByText('Running')).toBeVisible()
    expect(
      screen.getByRole('status', { name: 'Snapshot maintenance progress' })
    ).toHaveTextContent('Database body cleanup')
    expect(startRequestSnapshotCleanup).toHaveBeenCalledOnce()
  })

  test('restores a running job and prevents concurrent migration or cleanup', async () => {
    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus({ migration: runningJob }),
    })
    renderMaintenance()
    expect(await screen.findByText('Running')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove verified database bodies' })
    ).toBeDisabled()
    expect(startRequestSnapshotMigration).not.toHaveBeenCalled()
    expect(startRequestSnapshotCleanup).not.toHaveBeenCalled()
  })

  test('shows failed job and upload reasons without hiding retained queue data', async () => {
    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus({
        last_error: 'Upload timed out',
        migration: {
          ...runningJob,
          status: 'failed',
          error: 'Verification failed for row 5',
        },
      }),
    })
    renderMaintenance()
    expect(
      await screen.findByText('Verification failed for row 5')
    ).toBeVisible()
    expect(screen.getByText('Upload timed out')).toBeVisible()
    expect(screen.getByText('dev-node-1')).toBeVisible()
    expect(
      screen.getByRole('status', { name: 'Snapshot maintenance progress' })
    ).toHaveTextContent('Failed')
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeEnabled()
  })

  test('disables maintenance when status cannot be loaded and allows a refresh', async () => {
    vi.mocked(getRequestSnapshotStatus).mockRejectedValue(
      new Error('Node queue is unavailable')
    )
    renderMaintenance()
    expect(await screen.findByText('Node queue is unavailable')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Remove verified database bodies' })
    ).toBeDisabled()
    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus(),
    })
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(await screen.findByText('dev-node-1')).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeEnabled()
  })

  test('reports a rejected maintenance start and permits another attempt', async () => {
    vi.mocked(startRequestSnapshotMigration).mockResolvedValue({
      success: false,
      message: 'Migration lease is held by another node',
      data: runningJob,
    })
    renderMaintenance()
    const button = await screen.findByRole('button', {
      name: 'Migrate historical snapshots',
    })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)
    fireEvent.click(
      await screen.findByRole('button', { name: 'Start migration' })
    )
    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Migration lease is held by another node'
      )
    )
    expect(button).toBeEnabled()
    expect(screen.getByText('Idle')).toBeVisible()
  })

  test('preserves a running checkpoint when refresh fails and replaces it on recovery', async () => {
    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus({
        migration: {
          ...runningJob,
          phase: 'legacy_tasks',
          task_cursor: 1600,
          task_upper_id: 3000,
          scanned: 1600,
        },
      }),
    })
    renderMaintenance()
    expect(await screen.findByText('1600 / 3000')).toBeVisible()
    vi.mocked(getRequestSnapshotStatus).mockRejectedValue(
      new Error('Status request timed out')
    )
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(await screen.findByText('Status request timed out')).toBeVisible()
    expect(
      screen.getByText(
        'Status refresh failed. Showing the last received status.'
      )
    ).toBeVisible()
    expect(screen.getByText('1600 / 3000')).toBeVisible()
    expect(screen.getByText('Historical tasks')).toBeVisible()

    vi.mocked(getRequestSnapshotStatus).mockResolvedValue({
      success: true,
      data: snapshotStatus({
        migration: {
          ...runningJob,
          status: 'completed',
          phase: 'archive_table',
          cursor: 50,
          archive_upper_id: 50,
        },
      }),
    })
    fireEvent.click(screen.getByRole('button', { name: 'Refresh' }))
    expect(await screen.findByText('Completed')).toBeVisible()
    expect(screen.getByText('50 / 50')).toBeVisible()
    expect(
      screen.queryByText('Status request timed out')
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Migrate historical snapshots' })
    ).toBeEnabled()
    expect(startRequestSnapshotMigration).not.toHaveBeenCalled()
  })
})
