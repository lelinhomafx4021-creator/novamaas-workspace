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
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, expect, test } from 'vitest'

import { useSidebarData } from '@/hooks/use-sidebar-data'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

const originalAdapter = api.defaults.adapter
let client: QueryClient
let available: boolean
let requests: number

beforeEach(() => {
  useAuthStore.getState().auth.reset()
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  available = false
  requests = 0
  api.defaults.adapter = async (config) => {
    requests++
    return {
      data: { success: true, data: { available } },
      status: 200,
      statusText: 'OK',
      config,
      headers: {},
    }
  }
})

afterEach(() => {
  cleanup()
  client.clear()
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.reset()
})

function Wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

for (const role of [0, 1, 10, 100]) {
  for (const enabled of [false, true]) {
    test(`sidebar and command-search source omit manual unless role ${role} and enabled=${enabled} permit it`, async () => {
      available = enabled
      if (role) {
        useAuthStore.setState((state) => ({
          auth: {
            ...state.auth,
            user: { id: 10, username: 'reader', role },
            accessToken: 'test-access',
            accessExpiresAt: 9999999999,
          },
        }))
      }
      const { result } = renderHook(() => useSidebarData(), {
        wrapper: Wrapper,
      })
      if (role >= 10) await waitFor(() => expect(client.isFetching()).toBe(0))
      const items = result.current.navGroups.flatMap((group) => group.items)
      expect(items.some((item) => item.url === '/admin-manual')).toBe(
        enabled && role >= 10
      )
      if (role < 10) expect(requests).toBe(0)
    })
  }
}

test('changing account cannot reuse a previous administrator capability', async () => {
  available = true
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 10, username: 'admin', role: 10 },
      accessToken: 'test-admin',
      accessExpiresAt: 9999999999,
    },
  }))
  const { result } = renderHook(() => useSidebarData(), { wrapper: Wrapper })
  await waitFor(() =>
    expect(
      result.current.navGroups
        .flatMap((group) => group.items)
        .some((item) => item.url === '/admin-manual')
    ).toBe(true)
  )
  act(() =>
    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        user: { id: 11, username: 'member', role: 1 },
        accessToken: 'test-member',
      },
    }))
  )
  expect(
    result.current.navGroups
      .flatMap((group) => group.items)
      .some((item) => item.url === '/admin-manual')
  ).toBe(false)
})
