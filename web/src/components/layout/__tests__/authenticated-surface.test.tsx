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
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { AuthenticatedLayout } from '../components/authenticated-layout'

const initialConfig = useSystemConfigStore.getState()
let queryClient: QueryClient

beforeEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  useAuthStore.getState().auth.setUser({ id: 1, username: 'preview', role: 1 })
  useSystemConfigStore.setState(initialConfig)
  useSystemConfigStore.getState().setConfig({
    systemName: 'Test MaaS',
    logo: '/logo.svg?v=4',
  })
  useSystemConfigStore.getState().setLoading(false)
})

afterEach(() => {
  queryClient?.clear()
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState(initialConfig)
  localStorage.clear()
})

describe('Authenticated application surface', () => {
  test('full-screen applications fill the window and returning to the dashboard restores its navigation', async () => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(['status'], { announcements_enabled: false })
    queryClient.setQueryData(['notice'], { success: true, data: '' })
    const root = createRootRoute({ component: AuthenticatedLayout })
    const studio = createRoute({
      getParentRoute: () => root,
      path: '/image-studio',
      staticData: { fullScreen: true },
      component: () => <main>Image editor</main>,
    })
    const dashboard = createRoute({
      getParentRoute: () => root,
      path: '/dashboard',
      component: () => <main>Dashboard content</main>,
    })
    const router = createRouter({
      routeTree: root.addChildren([studio, dashboard]),
      history: createMemoryHistory({ initialEntries: ['/image-studio'] }),
    })
    await router.load()
    const view = render(
      <QueryClientProvider client={queryClient}>
        <DirectionProvider>
          <RouterProvider router={router} />
        </DirectionProvider>
      </QueryClientProvider>
    )

    const editor = await screen.findByText('Image editor')
    expect(view.container.querySelector('[data-slot="app-header"]')).toBeNull()
    expect(view.container.querySelector('[data-slot="sidebar"]')).toBeNull()
    expect(editor.closest('.h-svh')).toHaveClass('w-full', 'overflow-hidden')
    await act(() => router.navigate({ to: '/dashboard' }))
    const content = await screen.findByText('Dashboard content')
    expect(content.closest('[data-slot="sidebar-inset"]')).not.toBeNull()
    expect(
      view.container.querySelector('[data-slot="app-header"]')
    ).not.toBeNull()
  })

  test('dashboard content uses the MaaS shell with a glass toolbar', async () => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    })
    queryClient.setQueryData(['status'], { announcements_enabled: false })
    queryClient.setQueryData(['notice'], { success: true, data: '' })
    const root = createRootRoute({
      component: () => (
        <AuthenticatedLayout>
          <main>Dashboard content</main>
        </AuthenticatedLayout>
      ),
    })
    const router = createRouter({
      routeTree: root,
      history: createMemoryHistory({ initialEntries: ['/dashboard'] }),
    })
    await router.load()

    render(
      <QueryClientProvider client={queryClient}>
        <DirectionProvider>
          <RouterProvider router={router} />
        </DirectionProvider>
      </QueryClientProvider>
    )

    const content = await screen.findByText('Dashboard content')
    const surface = content.closest('[data-slot="sidebar-wrapper"]')
    const toolbar = surface?.querySelector('[data-slot="app-header"]')
    const inset = content.closest('[data-slot="sidebar-inset"]')
    const logo = toolbar?.querySelector('img[alt="Logo"]')

    expect(surface).toHaveClass('maas-app-shell')
    expect(toolbar).toHaveAttribute('data-surface', 'glass')
    expect(logo?.parentElement).toHaveClass('size-8')
    expect(inset).toContainElement(content)
  })
})
