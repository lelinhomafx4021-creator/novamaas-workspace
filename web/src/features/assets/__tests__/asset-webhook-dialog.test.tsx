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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { createAssetWebhookEndpoint, listAssetWebhookEndpoints } from '../api'
import { AssetWebhookDialog } from '../components/asset-webhook-dialog'

vi.mock('../api', () => ({
  createAssetWebhookEndpoint: vi.fn(),
  deleteAssetWebhookEndpoint: vi.fn(),
  listAssetWebhookEndpoints: vi.fn(),
  testAssetWebhookEndpoint: vi.fn(),
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
      <AssetWebhookDialog open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(listAssetWebhookEndpoints).mockResolvedValue({
    success: true,
    data: [],
  })
})

afterEach(() => queryClient?.clear())

test('creates an HTTPS callback without requiring a signing secret', async () => {
  vi.mocked(createAssetWebhookEndpoint).mockResolvedValue({
    success: true,
    data: {
      id: 'we_example',
      object: 'webhook_endpoint',
      name: 'production callback',
      url: 'https://customer.example.com/webhooks/assets',
      event_types: ['asset.active', 'asset.failed'],
      status: 'enabled',
      created_at: 1,
      updated_at: 1,
    },
  })
  renderDialog()

  expect(await screen.findByText('No webhook endpoints yet')).toBeVisible()
  fireEvent.change(screen.getByLabelText('Endpoint name'), {
    target: { value: 'production callback' },
  })
  fireEvent.change(screen.getByLabelText('Callback URL'), {
    target: { value: 'https://customer.example.com/webhooks/assets' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))

  await waitFor(() =>
    expect(createAssetWebhookEndpoint).toHaveBeenCalledWith({
      name: 'production callback',
      url: 'https://customer.example.com/webhooks/assets',
      event_types: ['asset.active', 'asset.failed'],
    })
  )
  await waitFor(() =>
    expect(screen.getByLabelText('Endpoint name')).toHaveValue('')
  )
  expect(
    screen.queryByText('Save your webhook signing secret now')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Close' })).toBeVisible()
})
