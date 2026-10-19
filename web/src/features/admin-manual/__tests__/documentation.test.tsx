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
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import { StrictMode } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { DirectionProvider } from '@/context/direction-provider'
import { NotFoundError } from '@/features/errors/not-found-error'
import { api } from '@/lib/api'
import { Route as ManualRoute } from '@/routes/admin-manual'
import { Route as DocsRoute } from '@/routes/docs'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

const documents = [
  {
    id: 'first',
    title: 'Private chapter one',
    content:
      '## Synthetic procedure\n\nPrivate test guidance.\n\n### Follow up\n\nTest verification.',
  },
  {
    id: 'second',
    title: 'Private chapter two',
    content: '## Synthetic reset\n\nSearchable reset example.',
  },
]
const originalAdapter = api.defaults.adapter
const originalConfig = useSystemConfigStore.getState()
let client: QueryClient
let enabled: boolean
let contentDenied: boolean
let contentRequests: number
let accessRequests: number
let contentLoad: Promise<void> | undefined

function signIn(role: number, id = 20) {
  useAuthStore.getState().auth.setBundle({
    access_token: `test-access-${id}`,
    access_expires_at: Math.floor(Date.now() / 1000) + 3600,
    token_type: 'Bearer',
    user: { id, username: `reader-${id}`, role },
    session: {
      sid: `test-session-${id}`,
      current: true,
      login_method: 'password',
      ip: '',
      user_agent: '',
      created_at: 1,
      last_active_at: 1,
      expires_at: 9999999999,
    },
  })
}

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(HTMLElement.prototype, 'scrollIntoView').mockImplementation(() => {})
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState(originalConfig)
  useSystemConfigStore.getState().setLoading(false)
  enabled = true
  contentDenied = false
  contentRequests = 0
  accessRequests = 0
  contentLoad = undefined
  api.defaults.adapter = async (config) => {
    let data: unknown = ''
    const user = useAuthStore.getState().auth.user
    const authorized =
      enabled &&
      (user?.role ?? 0) >= 10 &&
      useAuthStore.getState().auth.accessToken !== 'expired-manual-access'
    if (config.url === '/api/admin-manual/access') accessRequests++
    if (config.url === '/api/admin-manual') contentRequests++
    if (
      (config.url === '/api/admin-manual/access' && !authorized) ||
      (config.url === '/api/admin-manual' && (!authorized || contentDenied))
    ) {
      throw new AxiosError('Not Found', undefined, config, undefined, {
        status: 404,
        statusText: 'Not Found',
        headers: {},
        config,
        data: { error: { type: 'invalid_request_error' } },
      })
    }
    if (config.url === '/api/admin-manual/access') data = { available: true }
    if (config.url === '/api/admin-manual') {
      await contentLoad
      data = documents
    }
    if (config.url === '/api/docs') {
      data = [
        {
          id: 'quickstart',
          title: 'Public quick start',
          content: '## Public request\n\nPublic test guidance.',
        },
      ]
    }
    if (config.url === '/api/status') {
      data = { HeaderNavModules: '{"docs":true}', announcements_enabled: false }
    }
    return {
      data: { success: true, data },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
})

afterEach(async () => {
  cleanup()
  client?.clear()
  await api.get('/api/status', { disableDuplicate: true })
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState(originalConfig)
  localStorage.clear()
})

async function renderManual(path = '/admin-manual') {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  client.setQueryData(['status'], {
    HeaderNavModules: '{"docs":true}',
    announcements_enabled: false,
  })
  client.setQueryData(['notice'], { success: true, data: '' })
  const root = createRootRouteWithContext<{ queryClient: QueryClient }>()({
    component: Outlet,
    notFoundComponent: NotFoundError,
  })
  const manual = ManualRoute.update({
    id: '/admin-manual/',
    path: '/admin-manual/',
    getParentRoute: () => root,
  } as Parameters<typeof ManualRoute.update>[0])
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => <h1>Home</h1>,
  })
  const docs = DocsRoute.update({
    id: '/docs/',
    path: '/docs/',
    getParentRoute: () => root,
  } as Parameters<typeof DocsRoute.update>[0])
  const router = createRouter({
    routeTree: root.addChildren([manual, docs, home]),
    context: { queryClient: client },
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()
  render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <DirectionProvider>
          <RouterProvider router={router} />
        </DirectionProvider>
      </QueryClientProvider>
    </StrictMode>
  )
  return router
}

test('strictly mounted authorized readers switch between separate manuals inside the documentation layout', async () => {
  signIn(10)
  const router = await renderManual('/docs')
  await screen.findByRole('heading', { name: 'Public quick start' })
  expect(contentRequests).toBe(0)
  const user = userEvent.setup()
  const switcher = await screen.findByRole('navigation', { name: 'Docs' })
  expect(
    within(switcher).getByRole('link', { name: 'API documentation' })
  ).toHaveAttribute('aria-current', 'page')
  const manual = within(switcher).getByRole('link', {
    name: 'Platform integration guide',
  })
  let release!: () => void
  contentLoad = new Promise<void>((resolve) => {
    release = resolve
  })
  manual.focus()
  await user.keyboard('{Enter}')
  await screen.findByRole('status', { name: 'Loading' })
  expect(
    within(screen.getByRole('navigation', { name: 'Docs' })).getByRole('link', {
      name: 'Platform integration guide',
    })
  ).toHaveAttribute('aria-current', 'page')
  await act(async () => {
    release()
  })
  await screen.findByRole('heading', { name: 'Private chapter one' })
  expect(router.state.location.pathname).toBe('/admin-manual')
  const header = screen
    .getAllByRole('banner')
    .find((element) => within(element).queryByRole('link', { name: 'Console' }))
  expect(header).toBeInTheDocument()
  if (!header) return
  expect(within(header).getByRole('link', { name: 'Docs' })).toHaveAttribute(
    'aria-current',
    'page'
  )
  expect(
    screen.queryByRole('button', { name: 'Toggle Sidebar' })
  ).not.toBeInTheDocument()
  const privateSwitcher = screen.getByRole('navigation', { name: 'Docs' })
  expect(
    within(privateSwitcher).getByRole('link', {
      name: 'Platform integration guide',
    })
  ).toHaveAttribute('aria-current', 'page')
  await user.click(screen.getByRole('button', { name: 'Documentation menu' }))
  await user.type(
    screen.getByRole('searchbox', { name: 'Search documentation' }),
    'Public request'
  )
  expect(screen.getByText('No documentation found')).toBeInTheDocument()
  await user.click(
    within(privateSwitcher).getByRole('link', { name: 'API documentation' })
  )
  await screen.findByRole('heading', { name: 'Public quick start' })
  expect(router.state.location.pathname).toBe('/docs')
  expect(
    screen.getByRole('searchbox', { name: 'Search documentation' })
  ).toHaveValue('')
  expect(screen.queryByText('Private test guidance.')).not.toBeInTheDocument()
  expect(
    client.getQueriesData({ queryKey: ['admin-manual', 'documents'] })
  ).toEqual([])
})

for (const role of [0, 1, 10, 100]) {
  for (const open of [false, true]) {
    test(`direct page access with role ${role} and enabled=${open} follows the permission matrix`, async () => {
      enabled = open
      if (role) signIn(role)
      await renderManual()
      if (open && role >= 10) {
        expect(
          await screen.findByRole('heading', { name: 'Private chapter one' })
        ).toBeInTheDocument()
        expect(contentRequests).toBeGreaterThan(0)
      } else {
        expect(
          await screen.findByRole('heading', { name: '404' })
        ).toBeInTheDocument()
        expect(
          screen.queryByText('Private test guidance.')
        ).not.toBeInTheDocument()
        expect(contentRequests).toBe(0)
        if (role < 10) expect(accessRequests).toBe(0)
      }
    })
  }
}

test('manual search, mobile menu, chapter navigation and anchors stay on the protected page', async () => {
  signIn(10)
  const router = await renderManual()
  await screen.findByRole('heading', { name: 'Private chapter one' })
  const user = userEvent.setup()
  const menu = screen.getByRole('button', { name: 'Documentation menu' })
  expect(menu).toHaveAttribute('aria-expanded', 'false')
  menu.focus()
  await user.keyboard('{Enter}')
  expect(menu).toHaveAttribute('aria-expanded', 'true')
  await user.type(
    screen.getByRole('searchbox', { name: 'Search documentation' }),
    'reset'
  )
  const navigation = screen.getByRole('navigation', {
    name: 'Platform integration guide',
  })
  expect(
    within(navigation).queryByRole('link', { name: 'Private chapter one' })
  ).not.toBeInTheDocument()
  await user.click(
    within(navigation).getByRole('link', { name: /^Private chapter two/ })
  )
  await screen.findByRole('heading', { name: 'Private chapter two' })
  expect(router.state.location.pathname).toBe('/admin-manual')
  expect(router.state.location.search).toMatchObject({ article: 'second' })
  expect(menu).toHaveAttribute('aria-expanded', 'false')
  await user.click(
    within(screen.getByRole('navigation', { name: 'On this page' })).getByRole(
      'link',
      { name: 'Synthetic reset' }
    )
  )
  expect(router.state.location.pathname).toBe('/admin-manual')
  expect(router.state.location.hash).toBe('synthetic-reset')
  await user.click(
    screen.getByRole('link', { name: /Previous\s*Private chapter one/ })
  )
  await screen.findByRole('heading', { name: 'Private chapter one' })
})

test('unknown chapters and a revoked content request show the ordinary missing page', async () => {
  signIn(100)
  await renderManual('/admin-manual?article=missing')
  expect(
    await screen.findByRole('heading', { name: '404' })
  ).toBeInTheDocument()
  cleanup()
  client.clear()
  contentDenied = true
  await renderManual()
  expect(
    await screen.findByRole('heading', { name: '404' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Private test guidance.')).not.toBeInTheDocument()
})

test('policy closure on revalidation removes previously displayed text and query data', async () => {
  signIn(10)
  await renderManual()
  await screen.findByText('Private test guidance.')
  enabled = false
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['admin-manual', 'access'] })
  })
  expect(
    await screen.findByRole('heading', { name: '404' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Private test guidance.')).not.toBeInTheDocument()
  await waitFor(() =>
    expect(
      client.getQueriesData({ queryKey: ['admin-manual', 'documents'] })
    ).toEqual([])
  )
})

test('logout or a role downgrade hides old content without waiting for another response', async () => {
  signIn(10)
  await renderManual()
  await screen.findByText('Private test guidance.')
  act(() =>
    useAuthStore
      .getState()
      .auth.setUser({ id: 20, username: 'reader-20', role: 1 })
  )
  expect(
    await screen.findByRole('heading', { name: '404' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Private test guidance.')).not.toBeInTheDocument()
  act(() => useAuthStore.getState().auth.reset())
  expect(screen.getByRole('heading', { name: '404' })).toBeInTheDocument()
})

test('a failed background content read never continues to display cached text', async () => {
  signIn(10)
  await renderManual()
  await screen.findByText('Private test guidance.')
  contentDenied = true
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['admin-manual', 'documents'] })
  })
  expect(
    await screen.findByRole('heading', { name: '404' })
  ).toBeInTheDocument()
  expect(screen.queryByText('Private test guidance.')).not.toBeInTheDocument()
})

test('scrolling the documentation window updates the manual directory', async () => {
  signIn(10)
  await renderManual()
  await screen.findByRole('heading', { name: 'Private chapter one' })
  const directory = screen.getByRole('navigation', { name: 'On this page' })
  const procedure = within(directory).getByRole('link', {
    name: 'Synthetic procedure',
  })
  const followUp = within(directory).getByRole('link', { name: 'Follow up' })
  expect(procedure).toHaveAttribute('aria-current', 'page')
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      if (this.id === 'synthetic-procedure') {
        return new DOMRect(0, -200, 600, 32)
      }
      if (this.id === 'follow-up') return new DOMRect(0, 100, 600, 32)
      return new DOMRect(0, 900, 600, 100)
    }
  )
  fireEvent.scroll(window)
  await waitFor(() => expect(followUp).toHaveAttribute('aria-current', 'page'))
  expect(procedure).not.toHaveAttribute('aria-current')
})

test('expired access credentials refresh before the hidden endpoint can turn them into a missing page', async () => {
  signIn(10)
  const auth = useAuthStore.getState().auth
  const refreshed = {
    user: auth.user,
    session: auth.session,
    access_token: 'refreshed-manual-access',
    token_type: 'Bearer',
    access_expires_at: Math.floor(Date.now() / 1000) + 3600,
  }
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      accessToken: 'expired-manual-access',
      accessExpiresAt: 1,
    },
  }))
  const request = axios.Axios.prototype.request
  const refresh = vi
    .spyOn(axios.Axios.prototype, 'request')
    .mockImplementation(function (
      this: InstanceType<typeof axios.Axios>,
      config: AxiosRequestConfig
    ) {
      if (config.url === '/api/user/auth/refresh') {
        return Promise.resolve({
          data: { success: true, data: refreshed },
          status: 200,
        })
      }
      return request.call(this, config)
    } as typeof request)
  await renderManual()
  await waitFor(() =>
    expect(screen.getByText('Private test guidance.')).toBeInTheDocument()
  )
  expect(refresh).toHaveBeenCalledWith(
    expect.objectContaining({
      url: '/api/user/auth/refresh',
      headers: { 'X-Auth-Session': 'test-session-20' },
    })
  )
})
