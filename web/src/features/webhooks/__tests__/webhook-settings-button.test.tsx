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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { WebhookSettingsButton } from '../components/webhook-settings-button'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))
let client: QueryClient

afterEach(() => {
  client?.clear()
  vi.resetAllMocks()
})

test.each(['assets', 'tasks'] as const)(
  'the %s entry follows its own category switch',
  async (scope) => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          asset_library_enabled: scope === 'assets',
          media_tasks_enabled: scope === 'tasks',
          manual_enabled: false,
        },
      },
    })
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const allowedClick = vi.fn()
    const blockedClick = vi.fn()
    render(
      <QueryClientProvider client={client}>
        <WebhookSettingsButton scope={scope} onClick={allowedClick} />
        <WebhookSettingsButton
          scope={scope === 'assets' ? 'tasks' : 'assets'}
          onClick={blockedClick}
        />
      </QueryClientProvider>
    )
    const button = await screen.findByRole('button', { name: 'Webhooks' })
    expect(screen.getAllByRole('button', { name: 'Webhooks' })).toHaveLength(1)
    fireEvent.click(button)
    expect(allowedClick).toHaveBeenCalledOnce()
    expect(blockedClick).not.toHaveBeenCalled()
  }
)

test('the entry remains available for manual downloads while categories are disabled', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: true,
      data: {
        asset_library_enabled: false,
        media_tasks_enabled: false,
        manual_enabled: true,
      },
    },
  })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClick = vi.fn()
  render(
    <QueryClientProvider client={client}>
      <WebhookSettingsButton scope='tasks' onClick={onClick} />
    </QueryClientProvider>
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Webhooks' }))
  expect(onClick).toHaveBeenCalledOnce()
})
