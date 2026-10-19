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
import { AxiosError } from 'axios'
import { useState } from 'react'
import { afterEach, expect, test } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { AdminManualSection } from '../admin-manual-section'

const originalAdapter = api.defaults.adapter
let client: QueryClient

afterEach(() => {
  client?.clear()
  api.defaults.adapter = originalAdapter
})

function SettingsFixture() {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActions} />
      <SettingsPageProvider actionsContainer={actions}>
        <AdminManualSection defaultEnabled={false} />
      </SettingsPageProvider>
    </>
  )
}

test('root control saves the selected policy and invalidates capability without exposing it in status', async () => {
  let stored: unknown
  const urls: string[] = []
  client = new QueryClient()
  client.setQueryData(['admin-manual', 'access', 10, 'sid', 100], false)
  client.setQueryData(
    ['admin-manual', 'documents', 10, 'sid', 100],
    [{ content: 'Synthetic old private content' }]
  )
  api.defaults.adapter = async (config) => {
    urls.push(config.url ?? '')
    stored = JSON.parse(config.data as string)
    return {
      data: { success: true },
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
    }
  }
  render(
    <QueryClientProvider client={client}>
      <SettingsFixture />
    </QueryClientProvider>
  )
  const save = await screen.findByRole('button', { name: 'Save Changes' })
  expect(save).toBeDisabled()
  fireEvent.click(
    screen.getByRole('switch', { name: 'Enable platform integration guide' })
  )
  expect(save).toBeEnabled()
  fireEvent.click(save)
  await waitFor(() =>
    expect(stored).toEqual({ key: 'AdminManualEnabled', value: 'true' })
  )
  await waitFor(() =>
    expect(
      client.getQueriesData({ queryKey: ['admin-manual', 'documents'] })
    ).toEqual([])
  )
  expect(
    client.getQueryState(['admin-manual', 'access', 10, 'sid', 100])
      ?.isInvalidated
  ).toBe(true)
  expect(urls).toEqual(['/api/option/'])
})

test('failed persistence keeps the switch editable for retry and does not grant capability', async () => {
  client = new QueryClient()
  client.setQueryData(['admin-manual', 'access', 10, 'sid', 100], false)
  api.defaults.adapter = async (config) => ({
    data: { success: false, message: 'Test save failed' },
    status: 200,
    statusText: 'OK',
    config,
    headers: {},
  })
  render(
    <QueryClientProvider client={client}>
      <SettingsFixture />
    </QueryClientProvider>
  )
  fireEvent.click(
    screen.getByRole('switch', { name: 'Enable platform integration guide' })
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
  )
  expect(client.getQueryData(['admin-manual', 'access', 10, 'sid', 100])).toBe(
    false
  )
})

test('transport failure keeps policy off and permits retry without an unhandled rejection', async () => {
  client = new QueryClient()
  client.setQueryData(['admin-manual', 'access', 10, 'sid', 100], false)
  let requests = 0
  api.defaults.adapter = async (config) => {
    requests++
    throw new AxiosError('Isolated network failure', undefined, config)
  }
  render(
    <QueryClientProvider client={client}>
      <SettingsFixture />
    </QueryClientProvider>
  )
  fireEvent.click(
    screen.getByRole('switch', { name: 'Enable platform integration guide' })
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(requests).toBe(1))
  expect(
    await screen.findByRole('button', { name: 'Save Changes' })
  ).toBeEnabled()
  expect(client.getQueryData(['admin-manual', 'access', 10, 'sid', 100])).toBe(
    false
  )
})
