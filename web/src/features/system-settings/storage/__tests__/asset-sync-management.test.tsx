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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { listAssetSyncJobs, retryAssetSyncJob } from '@/features/assets/api'

import { AssetSyncManagement } from '../asset-sync-management'

vi.mock('@/features/assets/api', () => ({
  listAssetSyncJobs: vi.fn(),
  retryAssetSyncJob: vi.fn(),
}))
vi.mock('../api', () => ({
  listAssetChannelConfigs: vi.fn(async () => ({
    success: true,
    data: [
      {
        channel_id: 7,
        channel_name: 'YooFang production',
        enabled: true,
      },
    ],
  })),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(listAssetSyncJobs).mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 91,
          asset_id: 'asset-20260922120000-failed',
          asset_name: 'Campaign hero',
          asset_type: 'image',
          group_id: 'group-1',
          group_name: 'Campaign',
          owner_user_id: 42,
          owner_name: 'Asset Owner',
          channel_id: 7,
          channel_name: 'YooFang production',
          operation: 'sync',
          status: 'failed',
          progress: 50,
          attempts: 3,
          last_error: 'upstream unavailable',
          last_synced_at: 1,
          updated_at: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      summary: { total: 1, pending: 0, processing: 0, active: 0, failed: 1 },
    },
  })
  vi.mocked(retryAssetSyncJob).mockResolvedValue({
    success: true,
    data: undefined,
  })
})

afterEach(() => queryClient?.clear())

function renderManagement() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <AssetSyncManagement />
    </QueryClientProvider>
  )
}

describe('asset synchronization management', () => {
  test('keeps failed synchronization handling on the administrator page', async () => {
    renderManagement()

    expect(await screen.findByText('Campaign hero')).toBeVisible()
    expect(screen.getByText('upstream unavailable')).toBeVisible()
    expect(screen.getByText('Channel ID · 7')).toBeVisible()
    expect(screen.getByText('User ID · 42')).toBeVisible()
    expect(screen.getByRole('table')).toHaveClass('table-fixed')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() => expect(retryAssetSyncJob).toHaveBeenCalledWith(91))
    expect(
      screen.queryByRole('button', { name: 'Sync all' })
    ).not.toBeInTheDocument()
  })

  test('presents synchronization details as a compact vertically centered table', async () => {
    renderManagement()

    const assetName = await screen.findByText('Campaign hero')
    const assetCell = assetName.closest('td')

    expect(screen.getAllByRole('columnheader')).toHaveLength(5)
    expect(
      screen.queryByRole('columnheader', { name: 'Actions' })
    ).not.toBeInTheDocument()
    expect(assetCell).toHaveClass('align-middle')
    expect(assetCell?.querySelector('svg')).toBeNull()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeVisible()
    for (const cell of screen.getAllByRole('cell')) {
      expect(cell).toHaveClass('align-middle')
    }
  })
})
