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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { WebhookDialog } from '../components/webhook-dialog'
import type { WebhookEndpoint } from '../types'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

const enabledCapabilities = {
  asset_library_enabled: true,
  media_tasks_enabled: true,
  manual_enabled: false,
}
let queryClient: QueryClient
const endpoint: WebhookEndpoint = {
  id: 'we_example',
  object: 'webhook_endpoint',
  name: 'production callback',
  url: 'https://customer.example.com/webhooks/events',
  event_types: ['asset.active', 'asset.failed'],
  status: 'enabled',
  created_at: 1,
  updated_at: 1,
}

function renderDialog(scope: 'assets' | 'tasks' = 'assets') {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <WebhookDialog scope={scope} open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
}

function fillEndpoint() {
  fireEvent.change(screen.getByLabelText('Endpoint name'), {
    target: { value: endpoint.name },
  })
  fireEvent.change(screen.getByLabelText('Callback URL'), {
    target: { value: endpoint.url },
  })
}

test('manual downloads remain available without offering configuration when all categories are disabled', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? {
              asset_library_enabled: false,
              media_tasks_enabled: false,
              manual_enabled: true,
            }
          : [],
    },
  }))
  renderDialog('tasks')
  expect(
    await screen.findByRole('button', {
      name: 'Download media task webhook manual',
    })
  ).toBeVisible()
  expect(await screen.findByText('No webhook endpoints yet')).toBeVisible()
  expect(screen.getByRole('status')).toHaveTextContent(
    'Webhooks are disabled by the administrator.'
  )
  expect(
    screen.queryByRole('button', { name: 'Add endpoint' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByText('Add your callback address above to receive events.')
  ).not.toBeInTheDocument()
})

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data: url === '/api/webhooks/capabilities' ? enabledCapabilities : [],
    },
  }))
  vi.mocked(api.post).mockResolvedValue({
    data: { success: true, data: endpoint },
  })
  vi.mocked(api.put).mockResolvedValue({
    data: { success: true, data: endpoint },
  })
})

test.each(['assets', 'tasks'] as const)(
  'the %s webhook page shows the manual download only when the persisted policy allows it',
  async (scope) => {
    vi.mocked(api.get).mockImplementation(async (url) => ({
      data: {
        success: true,
        data:
          url === '/api/webhooks/capabilities'
            ? { ...enabledCapabilities, manual_enabled: true }
            : [],
      },
    }))
    renderDialog(scope)
    expect(
      await screen.findByRole('button', {
        name:
          scope === 'assets'
            ? 'Download asset webhook manual'
            : 'Download media task webhook manual',
      })
    ).toBeVisible()
  }
)

test('a disabled manual policy hides the download while endpoint management stays available', async () => {
  renderDialog('tasks')
  await screen.findByText('No webhook endpoints yet')
  await waitFor(() =>
    expect(queryClient.getQueryData(['webhook-capabilities'])).toEqual({
      ...enabledCapabilities,
    })
  )
  expect(
    screen.queryByRole('button', {
      name: 'Download media task webhook manual',
    })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Add endpoint' })).toBeVisible()
})

test('a policy lookup failure prevents configuration and manual download', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data:
      url === '/api/webhooks/capabilities'
        ? { success: false, message: 'policy unavailable' }
        : { success: true, data: [] },
  }))
  renderDialog()
  await screen.findByText('No webhook endpoints yet')
  await waitFor(() =>
    expect(queryClient.getQueryState(['webhook-capabilities'])?.status).toBe(
      'error'
    )
  )
  expect(
    screen.queryByRole('button', { name: 'Download asset webhook manual' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Add endpoint' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('alert')).toHaveTextContent('policy unavailable')
})
afterEach(() => queryClient?.clear())

test('disabled categories prevent configuration even when the dialog is already open', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? {
              asset_library_enabled: false,
              media_tasks_enabled: false,
              manual_enabled: false,
            }
          : [],
    },
  }))
  renderDialog('tasks')
  expect(
    await screen.findByText('Webhooks are disabled by the administrator.')
  ).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Add endpoint' })
  ).not.toBeInTheDocument()
  expect(api.post).not.toHaveBeenCalled()
})

test('media configuration contains no asset choices when asset webhooks are disabled', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? { ...enabledCapabilities, asset_library_enabled: false }
          : [],
    },
  }))
  renderDialog('tasks')
  await screen.findByRole('button', { name: 'Add endpoint' })
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(screen.queryByText('Events')).not.toBeInTheDocument()
  fillEndpoint()
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/webhook-endpoints',
      {
        name: endpoint.name,
        url: endpoint.url,
        event_types: ['task.status_changed'],
      },
      expect.any(Object)
    )
  )
})

test('closing the category while the dialog is open removes the configuration form', async () => {
  renderDialog('tasks')
  await screen.findByRole('button', { name: 'Add endpoint' })
  queryClient.setQueryData(['webhook-capabilities'], {
    ...enabledCapabilities,
    media_tasks_enabled: false,
  })
  expect(
    await screen.findByText('Webhooks are disabled by the administrator.')
  ).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Add endpoint' })
  ).not.toBeInTheDocument()
})

test('editing a legacy mixed endpoint removes only a disabled category with a warning', async () => {
  const mixed = {
    ...endpoint,
    event_types: ['asset.failed', 'task.status_changed'],
  }
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? { ...enabledCapabilities, asset_library_enabled: false }
          : [mixed],
    },
  }))
  renderDialog('tasks')
  const edit = await screen.findByRole('button', {
    name: `Edit webhook ${endpoint.name}`,
  })
  expect(
    screen.getByRole('button', { name: `Send test event to ${endpoint.name}` })
  ).toBeDisabled()
  fireEvent.click(edit)
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(
    screen.getByText(
      'Subscriptions to disabled categories are removed when you save.'
    )
  ).toBeVisible()
  expect(api.put).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/webhook-endpoints/we_example',
      {
        name: endpoint.name,
        url: endpoint.url,
        event_types: ['task.status_changed'],
      },
      expect.any(Object)
    )
  )
})

test('asset entry creates an HTTPS callback with asset events selected', async () => {
  renderDialog()
  expect(await screen.findByText('No webhook endpoints yet')).toBeVisible()
  expect(
    screen.queryByRole('checkbox', { name: 'Media task status changes' })
  ).not.toBeInTheDocument()
  fillEndpoint()
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/webhook-endpoints',
      {
        name: endpoint.name,
        url: endpoint.url,
        event_types: ['asset.active', 'asset.failed'],
      },
      expect.any(Object)
    )
  )
  await waitFor(() =>
    expect(screen.getByLabelText('Endpoint name')).toHaveValue('')
  )
  expect(screen.getByRole('button', { name: 'Close' })).toBeVisible()
})

test('each dialog lists only its own category and preserves enabled legacy subscriptions on edit', async () => {
  const mixed = {
    ...endpoint,
    event_types: ['asset.failed', 'task.status_changed'],
  }
  const taskOnly = {
    ...endpoint,
    id: 'we_task',
    name: 'task callback',
    event_types: ['task.status_changed'],
  }
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? enabledCapabilities
          : [mixed, taskOnly],
    },
  }))
  renderDialog('assets')
  await screen.findByText(endpoint.name)
  expect(screen.queryByText('task callback')).not.toBeInTheDocument()
  expect(
    screen.queryByText('Media task status changes')
  ).not.toBeInTheDocument()
  fireEvent.click(
    screen.getByRole('button', { name: `Edit webhook ${endpoint.name}` })
  )
  expect(
    screen.getByText(
      'This endpoint also delivers the other category. Name or URL changes affect both.'
    )
  ).toBeVisible()
  expect(
    screen.queryByRole('checkbox', { name: 'Media task status changes' })
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/webhook-endpoints/we_example',
      {
        name: endpoint.name,
        url: endpoint.url,
        event_types: ['asset.failed', 'task.status_changed'],
      },
      expect.any(Object)
    )
  )
})

test('a legacy mixed endpoint cannot lose all asset events from the asset dialog', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? enabledCapabilities
          : [
              {
                ...endpoint,
                event_types: ['asset.failed', 'task.status_changed'],
              },
            ],
    },
  }))
  renderDialog('assets')
  await screen.findByText(endpoint.name)
  fireEvent.click(
    screen.getByRole('button', { name: `Edit webhook ${endpoint.name}` })
  )
  fireEvent.click(screen.getByRole('checkbox', { name: 'Asset fails review' }))
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  expect(await screen.findByText('Select at least one event')).toBeVisible()
  expect(api.put).not.toHaveBeenCalled()
})

test('deleting a legacy mixed endpoint explains that both categories stop', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? enabledCapabilities
          : [
              {
                ...endpoint,
                event_types: ['asset.failed', 'task.status_changed'],
              },
            ],
    },
  }))
  renderDialog('tasks')
  await screen.findByText(endpoint.name)
  fireEvent.click(
    screen.getByRole('button', { name: `Delete webhook ${endpoint.name}` })
  )
  expect(
    screen.getByText(
      'This endpoint also subscribes to the other category. Deleting it stops both.'
    )
  ).toBeVisible()
})

test('task entry only subscribes to media status events even when both categories are enabled', async () => {
  renderDialog('tasks')
  await screen.findByText('No webhook endpoints yet')
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(screen.queryByText('Asset becomes active')).not.toBeInTheDocument()
  fillEndpoint()
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/webhook-endpoints',
      {
        name: endpoint.name,
        url: endpoint.url,
        event_types: ['task.status_changed'],
      },
      expect.any(Object)
    )
  )
})

test('deselecting every event shows an accessible validation error and prevents submission', async () => {
  renderDialog('assets')
  await screen.findByText('No webhook endpoints yet')
  fillEndpoint()
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Asset becomes active' })
  )
  fireEvent.click(screen.getByRole('checkbox', { name: 'Asset fails review' }))
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  expect(await screen.findByText('Select at least one event')).toBeVisible()
  expect(
    screen.getByRole('checkbox', { name: 'Asset becomes active' })
  ).toHaveAttribute('aria-invalid', 'true')
  expect(api.post).not.toHaveBeenCalled()
})

test('an HTTP address shows an accessible error and prevents submission', async () => {
  renderDialog('tasks')
  await screen.findByText('No webhook endpoints yet')
  fillEndpoint()
  fireEvent.change(screen.getByLabelText('Callback URL'), {
    target: { value: 'http://customer.example/webhook' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  expect(
    await screen.findByText('Enter a valid HTTPS callback URL')
  ).toBeVisible()
  expect(screen.getByLabelText('Callback URL')).toHaveAttribute(
    'aria-invalid',
    'true'
  )
  expect(api.post).not.toHaveBeenCalled()
})

test('the account limit counts other-category endpoints without displaying them', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? enabledCapabilities
          : Array.from({ length: 5 }, (_, index) => ({
              ...endpoint,
              id: `we_${index}`,
              name: `callback ${index}`,
            })),
    },
  }))
  renderDialog('tasks')
  await waitFor(() =>
    expect(screen.getByText('No webhook endpoints yet')).toBeVisible()
  )
  expect(screen.getByRole('button', { name: 'Add endpoint' })).toBeDisabled()
  expect(screen.getByLabelText('Endpoint name')).toBeDisabled()
  expect(screen.queryByText('callback 0')).not.toBeInTheDocument()
  expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
  expect(
    screen.queryByText('Add your callback address above to receive events.')
  ).not.toBeInTheDocument()
  expect(
    screen.getByText(
      'The account limit is full. Delete an endpoint in the other category to add one here.'
    )
  ).toBeVisible()
})

test('a rejected save reports the server error and retains input for retry', async () => {
  vi.mocked(api.post).mockResolvedValue({
    data: { success: false, message: 'endpoint rejected' },
  })
  renderDialog('tasks')
  await screen.findByText('No webhook endpoints yet')
  fillEndpoint()
  fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }))
  await waitFor(() =>
    expect(toast.error).toHaveBeenCalledWith('endpoint rejected')
  )
  expect(screen.getByLabelText('Endpoint name')).toHaveValue(endpoint.name)
  expect(screen.getByLabelText('Callback URL')).toHaveValue(endpoint.url)
  expect(screen.getByRole('button', { name: 'Add endpoint' })).toBeEnabled()
})

test('test delivery uses the same shared endpoint API', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities' ? enabledCapabilities : [endpoint],
    },
  }))
  vi.mocked(api.post).mockResolvedValue({
    data: {
      success: true,
      data: {
        reachable: true,
        http_status: 204,
        duration_ms: 18,
        event_id: 'evt_test',
        webhook_id: 'wh_test',
      },
    },
  })
  renderDialog('assets')
  await screen.findByText(endpoint.name)
  fireEvent.click(
    screen.getByRole('button', { name: `Send test event to ${endpoint.name}` })
  )
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/webhook-endpoints/we_example/test',
      undefined,
      expect.any(Object)
    )
  )
  expect(await screen.findByText('Webhook test succeeded')).toBeVisible()
  expect(screen.getByText('HTTP 204 · 18 ms')).toBeVisible()
})

test('a non-2xx callback is reported as failed even when the test API succeeds', async () => {
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities' ? enabledCapabilities : [endpoint],
    },
  }))
  vi.mocked(api.post).mockResolvedValue({
    data: {
      success: true,
      data: {
        reachable: false,
        http_status: 503,
        duration_ms: 8,
        event_id: 'evt_test',
        webhook_id: 'wh_test',
      },
    },
  })
  renderDialog()
  await screen.findByText(endpoint.name)
  fireEvent.click(
    screen.getByRole('button', { name: `Send test event to ${endpoint.name}` })
  )
  expect(await screen.findByText('Webhook test failed')).toBeVisible()
  expect(screen.getByText('HTTP 503 · 8 ms')).toBeVisible()
})
