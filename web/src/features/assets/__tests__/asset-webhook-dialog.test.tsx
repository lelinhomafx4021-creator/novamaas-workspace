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

import {
  createAssetWebhookEndpoint,
  listAssetWebhookEndpoints,
  rotateAssetWebhookEndpointSecret,
} from '../api'
import { AssetWebhookDialog } from '../components/asset-webhook-dialog'

vi.mock('../api', () => ({
  createAssetWebhookEndpoint: vi.fn(),
  deleteAssetWebhookEndpoint: vi.fn(),
  listAssetWebhookEndpoints: vi.fn(),
  rotateAssetWebhookEndpointSecret: vi.fn(),
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

test('creates an HTTPS callback and reveals its signing secret only once', async () => {
  vi.mocked(createAssetWebhookEndpoint).mockResolvedValue({
    success: true,
    data: {
      id: 'we_example',
      object: 'webhook_endpoint',
      name: 'production callback',
      url: 'https://customer.example.com/webhooks/assets',
      event_types: ['asset.active', 'asset.failed'],
      signing_secret: 'whsec_once_only',
      signing_secret_hint: '****only',
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
  expect(await screen.findByText('whsec_once_only')).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Close' })
  ).not.toBeInTheDocument()

  fireEvent.click(
    screen.getByRole('button', { name: 'I have saved the signing secret' })
  )
  expect(screen.queryByText('whsec_once_only')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Close' })).toBeVisible()
})

test('confirms before rotating a signing secret', async () => {
  vi.mocked(listAssetWebhookEndpoints).mockResolvedValue({
    success: true,
    data: [
      {
        id: 'we_example',
        object: 'webhook_endpoint',
        name: 'production callback',
        url: 'https://customer.example.com/webhooks/assets',
        event_types: ['asset.failed'],
        signing_secret_hint: '****old1',
        status: 'enabled',
        created_at: 1,
        updated_at: 1,
      },
    ],
  })
  vi.mocked(rotateAssetWebhookEndpointSecret).mockResolvedValue({
    success: true,
    data: {
      id: 'we_example',
      object: 'webhook_endpoint',
      name: 'production callback',
      url: 'https://customer.example.com/webhooks/assets',
      event_types: ['asset.failed'],
      signing_secret: 'whsec_rotated_once',
      signing_secret_hint: '****once',
      status: 'enabled',
      created_at: 1,
      updated_at: 2,
    },
  })
  renderDialog()

  fireEvent.click(
    await screen.findByRole('button', {
      name: 'Rotate signing secret for production callback',
    })
  )

  expect(rotateAssetWebhookEndpointSecret).not.toHaveBeenCalled()
  expect(screen.getByText('Rotate signing secret?')).toBeVisible()
  fireEvent.click(screen.getByRole('button', { name: 'Rotate secret' }))

  await waitFor(() =>
    expect(rotateAssetWebhookEndpointSecret).toHaveBeenCalledWith('we_example')
  )
  expect(await screen.findByText('whsec_rotated_once')).toBeVisible()
})
