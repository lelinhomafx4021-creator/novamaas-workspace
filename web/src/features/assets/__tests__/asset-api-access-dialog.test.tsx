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

import {
  createAssetAccessKey,
  deleteAssetAccessKey,
  listAssetAccessKeys,
} from '../api'
import { AssetApiAccessDialog } from '../components/asset-api-access-dialog'

vi.mock('../api', () => ({
  createAssetAccessKey: vi.fn(),
  deleteAssetAccessKey: vi.fn(),
  listAssetAccessKeys: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient

function renderDialog() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <AssetApiAccessDialog open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(listAssetAccessKeys).mockResolvedValue({ success: true, data: [] })
})

afterEach(() => queryClient?.clear())

describe('asset library downstream API access', () => {
  test('shows connection details and reveals a newly created secret only once', async () => {
    vi.mocked(createAssetAccessKey).mockResolvedValue({
      success: true,
      data: {
        id: 7,
        name: 'production uploader',
        access_key_id: 'AKNMEXAMPLE',
        secret_access_key: 'secret-once-only',
        secret_hint: 'only',
        status: 'enabled',
        last_used_at: 0,
        created_at: 1,
      },
    })
    renderDialog()

    expect(await screen.findByText('Asset Library API')).toBeVisible()
    expect(screen.getByText(`${window.location.origin}/api/v3/`)).toBeVisible()
    expect(screen.getByText('cn-beijing')).toBeVisible()
    expect(screen.getByText('ark')).toBeVisible()

    fireEvent.change(screen.getByLabelText('Key name'), {
      target: { value: 'production uploader' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Create access key' }))

    await waitFor(() =>
      expect(createAssetAccessKey).toHaveBeenCalledWith('production uploader')
    )
    expect(await screen.findByText('AKNMEXAMPLE')).toBeVisible()
    expect(screen.getByText('secret-once-only')).toBeVisible()
    expect(screen.getByText('Access Key ID (Access Key)')).toBeVisible()
    expect(screen.getByText('Secret Access Key (Secret Key)')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Close' })
    ).not.toBeInTheDocument()

    fireEvent.click(
      screen.getByRole('button', { name: 'I have saved the credentials' })
    )
    expect(screen.queryByText('secret-once-only')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Close' })).toBeVisible()
  })

  test('requires confirmation before revoking an access key', async () => {
    vi.mocked(listAssetAccessKeys).mockResolvedValue({
      success: true,
      data: [
        {
          id: 9,
          name: 'batch uploader',
          access_key_id: 'AKNMBATCH',
          secret_hint: '1234',
          status: 'enabled',
          last_used_at: 0,
          created_at: 1,
        },
      ],
    })
    vi.mocked(deleteAssetAccessKey).mockResolvedValue({
      success: true,
      data: undefined,
    })
    renderDialog()

    fireEvent.click(
      await screen.findByRole('button', { name: 'Revoke batch uploader' })
    )
    expect(screen.getByText('Revoke access key?')).toBeVisible()
    fireEvent.click(screen.getByRole('button', { name: 'Revoke' }))

    await waitFor(() => expect(deleteAssetAccessKey).toHaveBeenCalledWith(9))
  })
})
