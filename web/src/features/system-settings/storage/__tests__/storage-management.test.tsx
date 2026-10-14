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

import { testStorageProfile } from '../api'
import { StorageManagement } from '../storage-management'

vi.mock('../api', () => ({
  archiveStorageProfile: vi.fn(),
  createStorageProfile: vi.fn(),
  getRelayMediaStoragePolicy: vi.fn(async () => ({
    success: true,
    data: {
      id: 0,
      key: 'relay_media_temp',
      name: 'Relay media temporary storage',
      purpose: 'relay_media_temp',
      storage_profile_id: 0,
      object_prefix: 'temporary/relay-media',
      signed_url_ttl_seconds: 259200,
      retention_seconds: 259200,
      max_file_bytes: 10485760,
      max_total_bytes: 20971520,
      max_files: 10,
      allowed_mime_types:
        'image/jpeg,image/png,image/webp,video/mp4,video/webm,video/quicktime',
      enabled: false,
      created_at: 0,
      updated_at: 0,
    },
  })),
  listStorageProfiles: vi.fn(async () => ({ success: true, data: [] })),
  getRequestSnapshotStoragePolicy: vi.fn(async () => ({
    success: true,
    data: {
      id: 0,
      key: 'request_snapshot',
      name: 'Request snapshot storage',
      purpose: 'request_snapshot',
      storage_profile_id: 0,
      object_prefix: 'audit/request-snapshots',
      signed_url_ttl_seconds: 0,
      retention_seconds: 0,
      max_file_bytes: 134217728,
      max_total_bytes: 2147483648,
      max_files: 2,
      allowed_mime_types: 'application/json',
      enabled: false,
      created_at: 0,
      updated_at: 0,
    },
  })),
  getRequestSnapshotStatus: vi.fn(async () => ({
    success: true,
    data: {
      node_name: 'dev-node',
      spool_directory: '/private/audit-spool',
      workers: 2,
      pending_files: 0,
      pending_bytes: 0,
      migration: {
        status: 'idle',
        mode: 'migrate',
        cursor: 0,
        processed: 0,
        failed: 0,
      },
    },
  })),
  updateRequestSnapshotStoragePolicy: vi.fn(),
  startRequestSnapshotMigration: vi.fn(),
  startRequestSnapshotCleanup: vi.fn(),
  testSavedStorageProfile: vi.fn(),
  testStorageProfile: vi.fn(),
  updateRelayMediaStoragePolicy: vi.fn(),
  updateStorageProfile: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient

beforeEach(() => vi.clearAllMocks())
afterEach(() => queryClient?.clear())

function renderStorageManagement() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <StorageManagement />
    </QueryClientProvider>
  )
}

describe('storage management', () => {
  test('opens the create profile dialog from the page action', async () => {
    renderStorageManagement()

    fireEvent.click(await screen.findByRole('button', { name: 'Add profile' }))

    expect(await screen.findByRole('dialog')).toBeVisible()
    expect(screen.getByText('Add storage profile')).toBeVisible()
  })

  test('shows the backend reason when testing a storage profile fails', async () => {
    vi.mocked(testStorageProfile).mockRejectedValue({
      isAxiosError: true,
      message: 'Request failed with status code 503',
      response: {
        data: {
          success: false,
          message: 'Set the storage credential encryption key on the server',
        },
      },
    })
    renderStorageManagement()

    fireEvent.click(await screen.findByRole('button', { name: 'Add profile' }))
    fireEvent.change(await screen.findByLabelText('Profile name'), {
      target: { value: 'Primary OSS' },
    })
    fireEvent.change(screen.getByLabelText('Bucket'), {
      target: { value: 'test-media-bucket' },
    })
    fireEvent.change(screen.getByLabelText('Access Key ID'), {
      target: { value: 'LTAI1234567890' },
    })
    fireEvent.change(screen.getByLabelText('Access Key Secret'), {
      target: { value: 'super-secret-value' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Test configuration' }))

    await waitFor(() => expect(testStorageProfile).toHaveBeenCalledOnce())
    expect(toast.error).toHaveBeenCalledWith(
      'Set the storage credential encryption key on the server'
    )
  })
})
