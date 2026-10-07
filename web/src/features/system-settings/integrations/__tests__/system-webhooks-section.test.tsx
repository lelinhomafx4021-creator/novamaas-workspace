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

import type { SystemWebhookSettings } from '@/features/webhooks/types'
import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { SystemWebhooksSection } from '../system-webhooks-section'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))
let client: QueryClient
let actionsContainer: HTMLDivElement
let storedSettings: SystemWebhookSettings
const initial: SystemWebhookSettings = {
  asset_library: { enabled: false, url: '' },
  media_tasks: { enabled: false, url: '' },
  manual_enabled: false,
}
function renderSection() {
  actionsContainer = document.createElement('div')
  actionsContainer.setAttribute('data-testid', 'page-actions')
  document.body.append(actionsContainer)
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <SystemWebhooksSection />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}
beforeEach(() => {
  vi.resetAllMocks()
  storedSettings = initial
  vi.mocked(api.get).mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/webhooks/capabilities'
          ? {
              asset_library_enabled: storedSettings.asset_library.enabled,
              media_tasks_enabled: storedSettings.media_tasks.enabled,
              manual_enabled: storedSettings.manual_enabled,
            }
          : storedSettings,
    },
  }))
  vi.mocked(api.put).mockImplementation(async (_url, input) => {
    storedSettings = input as SystemWebhookSettings
    return { data: { success: true, data: storedSettings } }
  })
  vi.mocked(api.post).mockResolvedValue({
    data: {
      success: true,
      data: {
        reachable: true,
        http_status: 204,
        duration_ms: 12,
        event_id: 'evt_probe',
        webhook_id: 'wh_probe',
      },
    },
  })
})
afterEach(() => {
  client?.clear()
  actionsContainer?.remove()
})

test('disabled categories keep their address and test controls unavailable', async () => {
  renderSection()
  expect(
    await screen.findByLabelText('Callback address for Media task webhook')
  ).toBeDisabled()
  expect(
    screen.getByLabelText('Callback address for Asset library webhook')
  ).toBeDisabled()
  expect(
    screen.getByRole('button', { name: 'Test Media task webhook' })
  ).toBeDisabled()
  expect(
    screen.getByRole('button', { name: 'Test Asset library webhook' })
  ).toBeDisabled()
  expect(api.post).not.toHaveBeenCalled()
})

test('a category can be enabled for personal webhooks without an operator address', async () => {
  renderSection()
  fireEvent.click(
    await screen.findByRole('switch', { name: 'Enable Media task webhook' })
  )
  expect(
    screen.getByLabelText('Callback address for Media task webhook')
  ).toBeEnabled()
  expect(
    screen.getByRole('button', { name: 'Test Media task webhook' })
  ).toBeDisabled()
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/webhooks/system',
      { ...initial, media_tasks: { enabled: true, url: '' } },
      expect.any(Object)
    )
  )
  await waitFor(() =>
    expect(client.getQueryData(['webhook-capabilities'])).toEqual({
      asset_library_enabled: false,
      media_tasks_enabled: true,
      manual_enabled: false,
    })
  )
})

test('category cards fill the available page width in a responsive grid', async () => {
  renderSection()
  const categories = await screen.findByRole('group', {
    name: 'Webhook categories',
  })
  const section = categories.closest('section')
  expect(section).toHaveClass('w-full', 'min-w-0')
  expect(section).not.toHaveClass('max-w-5xl')
  expect(categories).toHaveClass('grid', 'xl:grid-cols-2')
  expect(
    screen.getByRole('group', { name: 'Asset library webhook' })
  ).toBeVisible()
  expect(
    screen.getByRole('group', { name: 'Media task webhook' })
  ).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Test Media task webhook' })
  ).toHaveTextContent('Test connection')
})

test('places the save action in the settings page header and keeps the form below it', async () => {
  renderSection()
  const save = await screen.findByRole('button', { name: 'Save Changes' })
  expect(actionsContainer).toContainElement(save)
  expect(
    screen.getByRole('group', { name: 'Webhook categories' })
  ).not.toContainElement(save)
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
})

test('saves independent system category switches and addresses and updates the cached settings', async () => {
  renderSection()
  const taskSwitch = await screen.findByRole('switch', {
    name: 'Enable Media task webhook',
  })
  expect(taskSwitch).not.toBeChecked()
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  fireEvent.click(taskSwitch)
  fireEvent.change(
    screen.getByLabelText('Callback address for Media task webhook'),
    { target: { value: 'https://operator.example/tasks' } }
  )
  fireEvent.click(
    screen.getByRole('switch', { name: 'Enable Asset library webhook' })
  )
  fireEvent.change(
    screen.getByLabelText('Callback address for Asset library webhook'),
    { target: { value: 'https://operator.example/assets' } }
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  const expected = {
    asset_library: { enabled: true, url: 'https://operator.example/assets' },
    media_tasks: { enabled: true, url: 'https://operator.example/tasks' },
    manual_enabled: false,
  }
  await waitFor(() =>
    expect(api.put).toHaveBeenCalledWith(
      '/api/webhooks/system',
      expected,
      expect.any(Object)
    )
  )
  await waitFor(() =>
    expect(toast.success).toHaveBeenCalledWith('System webhook settings saved')
  )
  expect(client.getQueryData(['system-webhooks'])).toEqual(expected)
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
})

test('a nonempty operator address must be valid HTTPS before settings are saved', async () => {
  renderSection()
  fireEvent.click(
    await screen.findByRole('switch', { name: 'Enable Media task webhook' })
  )
  fireEvent.change(
    screen.getByLabelText('Callback address for Media task webhook'),
    { target: { value: 'http://operator.example/tasks' } }
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(
    await screen.findByText('Enter a valid HTTPS callback URL')
  ).toBeVisible()
  expect(
    screen.getByLabelText('Callback address for Media task webhook')
  ).toHaveAttribute('aria-invalid', 'true')
  expect(api.put).not.toHaveBeenCalled()
})

test('tests an unsaved enabled-category URL immediately and clears obsolete results on address changes', async () => {
  renderSection()
  fireEvent.click(
    await screen.findByRole('switch', { name: 'Enable Media task webhook' })
  )
  const input = screen.getByLabelText('Callback address for Media task webhook')
  fireEvent.change(input, {
    target: { value: 'https://operator.example/unsaved' },
  })
  fireEvent.click(
    screen.getByRole('button', { name: 'Test Media task webhook' })
  )
  await waitFor(() =>
    expect(api.post).toHaveBeenCalledWith(
      '/api/webhooks/system/media_tasks/test',
      { url: 'https://operator.example/unsaved' },
      expect.any(Object)
    )
  )
  expect(await screen.findByText('Webhook test succeeded')).toBeVisible()
  expect(screen.getByText('HTTP 204 · 12 ms')).toBeVisible()
  expect(api.put).not.toHaveBeenCalled()
  fireEvent.change(input, {
    target: { value: 'https://operator.example/other' },
  })
  expect(screen.queryByText('Webhook test succeeded')).not.toBeInTheDocument()
})

test('shows callback failure separately from a successful management request', async () => {
  vi.mocked(api.post).mockResolvedValue({
    data: {
      success: true,
      data: {
        reachable: false,
        http_status: 503,
        duration_ms: 7,
        event_id: 'evt_probe',
        webhook_id: 'wh_probe',
      },
    },
  })
  renderSection()
  fireEvent.click(
    await screen.findByRole('switch', { name: 'Enable Asset library webhook' })
  )
  fireEvent.change(
    screen.getByLabelText('Callback address for Asset library webhook'),
    { target: { value: 'https://operator.example/assets' } }
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Test Asset library webhook' })
  )
  expect(await screen.findByText('Webhook test failed')).toBeVisible()
  expect(screen.getByText('HTTP 503 · 7 ms')).toBeVisible()
  expect(
    screen.getByLabelText('Callback address for Asset library webhook')
  ).toHaveValue('https://operator.example/assets')
})

test('a configuration load failure cannot silently replace system settings with defaults', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: { success: false, message: 'configuration unavailable' },
  })
  renderSection()
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'configuration unavailable'
  )
  expect(
    screen.queryByRole('button', { name: 'Save Changes' })
  ).not.toBeInTheDocument()
  expect(api.put).not.toHaveBeenCalled()
})

test('downloads the branded PDF from the authenticated manual API', async () => {
  const blob = new Blob(['%PDF-1.7'], { type: 'application/pdf' })
  vi.mocked(api.get).mockImplementation(async (url) =>
    url === '/api/webhooks/manual.pdf'
      ? { data: blob }
      : {
          data: {
            success: true,
            data:
              url === '/api/webhooks/capabilities'
                ? {
                    asset_library_enabled: false,
                    media_tasks_enabled: false,
                    manual_enabled: true,
                  }
                : { ...initial, manual_enabled: true },
          },
        }
  )
  const create = vi.fn().mockReturnValue('blob:manual')
  const revoke = vi.fn()
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = create
      static revokeObjectURL = revoke
    }
  )
  const click = vi
    .spyOn(HTMLAnchorElement.prototype, 'click')
    .mockImplementation(() => {})
  try {
    renderSection()
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Download asset webhook manual',
      })
    )
    await waitFor(() =>
      expect(api.get).toHaveBeenCalledWith('/api/webhooks/manual.pdf', {
        responseType: 'blob',
        params: { scope: 'asset_library' },
      })
    )
    await waitFor(() => expect(create).toHaveBeenCalledWith(blob))
    expect(click).toHaveBeenCalledOnce()
    expect(revoke).toHaveBeenCalledWith('blob:manual')
  } finally {
    vi.unstubAllGlobals()
  }
})

test('manual download becomes available only after the administrator saves the switch and is hidden again when disabled', async () => {
  renderSection()
  const manualSwitch = await screen.findByRole('switch', {
    name: 'Allow users to download the Webhook API manual',
  })
  expect(manualSwitch).not.toBeChecked()
  expect(
    screen.queryByRole('button', { name: 'Download asset webhook manual' })
  ).not.toBeInTheDocument()
  fireEvent.click(manualSwitch)
  expect(
    screen.queryByRole('button', { name: 'Download asset webhook manual' })
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(
    await screen.findByRole('button', { name: 'Download asset webhook manual' })
  ).toBeVisible()
  expect(
    screen.getByRole('button', { name: 'Download media task webhook manual' })
  ).toBeVisible()
  expect(client.getQueryData(['webhook-capabilities'])).toEqual({
    asset_library_enabled: false,
    media_tasks_enabled: false,
    manual_enabled: true,
  })
  expect(api.put).toHaveBeenLastCalledWith(
    '/api/webhooks/system',
    { ...initial, manual_enabled: true },
    expect.any(Object)
  )
  fireEvent.click(manualSwitch)
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(
      screen.queryByRole('button', { name: 'Download asset webhook manual' })
    ).not.toBeInTheDocument()
  )
  expect(client.getQueryData(['webhook-capabilities'])).toEqual({
    asset_library_enabled: false,
    media_tasks_enabled: false,
    manual_enabled: false,
  })
  expect(api.get).not.toHaveBeenCalledWith(
    '/api/webhooks/manual.pdf',
    expect.anything()
  )
})

test('a rejected policy save keeps the download unavailable and retains the unsaved switch for retry', async () => {
  vi.mocked(api.put).mockResolvedValue({
    data: { success: false, message: 'policy save failed' },
  })
  renderSection()
  const manualSwitch = await screen.findByRole('switch', {
    name: 'Allow users to download the Webhook API manual',
  })
  fireEvent.click(manualSwitch)
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(toast.error).toHaveBeenCalledWith('policy save failed')
  )
  expect(manualSwitch).toBeChecked()
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  expect(
    screen.queryByRole('button', { name: 'Download asset webhook manual' })
  ).not.toBeInTheDocument()
  expect(storedSettings).toEqual(initial)
})
