import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { AccountSummaryPanel } from '../account-summary-panel'

let client: QueryClient
function showPanel() {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  useAuthStore.getState().auth.setUser({
    id: 7,
    username: 'platform-user',
    role: 1,
    display_name: 'Platform Roy',
    group: 'premium',
    email: 'roy@example.com',
  })
  render(
    <QueryClientProvider client={client}>
      <AccountSummaryPanel />
    </QueryClientProvider>
  )
}
afterEach(() => {
  client?.clear()
  useAuthStore.getState().auth.setUser(null)
})

test('shows platform account, verified phone and WeChat nickname without edit controls', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      data: url.endsWith('/phone')
        ? { phone: 'unverified', verified: { phone: '+8613800000000' } }
        : [{ app_id: 'wx-mini', nickname: 'WeChat Roy' }],
    },
  }))
  showPanel()
  expect(screen.getByText('@platform-user')).toBeInTheDocument()
  expect(await screen.findByText('+86 138 0000 0000')).toBeInTheDocument()
  expect(await screen.findByText('WeChat Roy')).toBeInTheDocument()
  expect(screen.getByText('premium')).toBeInTheDocument()
  expect(screen.getByText('Platform Roy')).toBeInTheDocument()
  expect(screen.getByText('roy@example.com')).toBeInTheDocument()
  expect(screen.getByText('7')).toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})

test('reports a load failure without claiming that the account is unbound', async () => {
  vi.spyOn(api, 'get').mockRejectedValue(new Error('Network unavailable'))
  showPanel()
  await waitFor(() =>
    expect(
      screen.getAllByText('Unable to load account information')
    ).toHaveLength(2)
  )
  expect(screen.queryByText('Not bound')).not.toBeInTheDocument()
})

test('shows unbound status when phone and WeChat bindings are absent', async () => {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: { data: url.endsWith('/phone') ? { phone: '', verified: null } : [] },
  }))
  showPanel()
  await waitFor(() => expect(screen.getAllByText('Not bound')).toHaveLength(2))
})

test('downloads a stored WeChat avatar with authentication and releases its display URL', async () => {
  const create = vi.fn(() => 'blob:wechat-avatar')
  const revoke = vi.fn()
  vi.spyOn(URL, 'createObjectURL').mockImplementation(create)
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(revoke)
  const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: url.includes('/avatar?')
      ? new Blob(['jpeg'], { type: 'image/jpeg' })
      : {
          data: url.endsWith('/phone')
            ? { phone: '', verified: null }
            : [{ app_id: 'wx-mini', nickname: 'WeChat Roy', has_avatar: true }],
        },
  }))
  showPanel()
  await waitFor(() => expect(create).toHaveBeenCalledOnce())
  expect(get).toHaveBeenCalledWith(
    '/api/user/self/wechat-miniapp/avatar?app_id=wx-mini',
    { responseType: 'blob' }
  )
  useAuthStore.getState().auth.setUser(null)
  await waitFor(() => expect(revoke).toHaveBeenCalledWith('blob:wechat-avatar'))
})
