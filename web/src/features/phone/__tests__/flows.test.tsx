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
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { OtpForm } from '@/features/auth/otp/components/otp-form'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import * as phoneAPI from '../api'
import { PhoneBindingCard } from '../components/phone-binding-card'
import { SmsLoginForm } from '../components/sms-login-form'
import { WeChatBindingCard } from '../components/wechat-binding-card'

vi.mock('../api', async (importOriginal) => {
  const original = await importOriginal<typeof import('../api')>()
  return {
    ...original,
    getPhoneStatus: vi.fn(),
    verifyAccountSecurity: vi.fn(),
    sendBindingSMS: vi.fn(),
    confirmPhoneBinding: vi.fn(),
    sendLoginSMS: vi.fn(),
    loginWithSMS: vi.fn(),
  }
})

let queryClient: QueryClient

beforeEach(() => {
  useAuthStore.getState().auth.reset()
  vi.mocked(phoneAPI.getPhoneStatus).mockResolvedValue({
    phone: '13800138000',
    verified: null,
    sms_enabled: true,
  })
  vi.mocked(phoneAPI.verifyAccountSecurity).mockResolvedValue(
    'current-session-proof'
  )
  vi.mocked(phoneAPI.sendBindingSMS).mockResolvedValue({
    challenge_token: 'binding-challenge',
    expires_in: 300,
    retry_after: 60,
  })
  vi.mocked(phoneAPI.sendLoginSMS).mockResolvedValue({
    challenge_token: 'login-challenge',
    expires_in: 300,
    retry_after: 60,
  })
  vi.mocked(phoneAPI.confirmPhoneBinding).mockResolvedValue(undefined)
})

afterEach(() => {
  queryClient?.clear()
  vi.clearAllMocks()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('administrator disconnects a scoped WeChat identity using their own security proof', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValue({ data: { data: [] } })
    .mockResolvedValueOnce({
      data: {
        data: [
          {
            app_id: 'wx-app',
            nickname: 'WeChat Name',
            has_avatar: false,
            openid: 'scoped-subject',
            bound_at: '2026-10-06T00:00:00Z',
          },
        ],
      },
    })
  const request = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  const changed = vi.fn()
  await show(() => <WeChatBindingCard userId={42} onChanged={changed} />)
  expect(await screen.findByText('WeChat Name')).toBeInTheDocument()
  await userEvent.click(
    screen.getByRole('button', { name: 'Manage WeChat binding' })
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Disconnect WeChat' })
  )
  expect(
    screen.queryByRole('button', { name: 'Verify current phone' })
  ).not.toBeInTheDocument()
  await userEvent.type(
    screen.getByLabelText('Current account password'),
    'admin-password'
  )
  await userEvent.click(screen.getByRole('button', { name: /^Verify$/ }))
  await waitFor(() =>
    expect(request).toHaveBeenCalledWith(
      '/api/user/42/wechat-miniapp/unbind',
      { app_id: 'wx-app' },
      { headers: { 'X-Security-Proof': 'current-session-proof' } }
    )
  )
  await waitFor(() => expect(changed).toHaveBeenCalledOnce())
  await waitFor(() =>
    expect(screen.queryByText('WeChat Name')).not.toBeInTheDocument()
  )
  expect(screen.getByText('Not bound')).toBeInTheDocument()
})

test('last-login protection keeps a WeChat binding visible and explains recovery', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      data: [
        {
          app_id: 'wx-app',
          nickname: 'WeChat Name',
          has_avatar: false,
          bound_at: '2026-10-06T00:00:00Z',
        },
      ],
    },
  })
  vi.spyOn(api, 'post').mockRejectedValue({
    response: { data: { code: 'MINI_AUTH_LAST_CREDENTIAL' } },
  })
  await show(() => <WeChatBindingCard userId={42} />)
  await userEvent.click(
    await screen.findByRole('button', { name: 'Manage WeChat binding' })
  )
  await userEvent.click(
    screen.getByRole('button', { name: 'Disconnect WeChat' })
  )
  await userEvent.type(
    screen.getByLabelText('Current account password'),
    'admin-password'
  )
  await userEvent.click(screen.getByRole('button', { name: /^Verify$/ }))
  expect(
    await screen.findByText(
      'Add another sign-in method before disconnecting WeChat.'
    )
  ).toBeInTheDocument()
  expect(screen.getAllByText('WeChat Name').length).toBeGreaterThan(0)
})

async function show(component: () => React.ReactNode) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], { turnstile_check: false })
  const route = createRootRoute({ component })
  const router = createRouter({
    routeTree: route,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

test('binding requires current account proof and invalidates the code when the destination changes', async () => {
  const user = userEvent.setup()
  await show(() => <PhoneBindingCard />)
  await user.click(
    await screen.findByRole('button', { name: 'Verify phone number' })
  )
  expect(
    screen.queryByLabelText('New phone number (+86)')
  ).not.toBeInTheDocument()
  await user.type(
    screen.getByLabelText('Current account password'),
    'correct-password'
  )
  await user.click(screen.getByRole('button', { name: /^Verify$/ }))
  const phone = await screen.findByLabelText('New phone number (+86)')
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  await user.type(screen.getByLabelText('SMS verification code'), '123456')
  expect(
    screen.getByRole('button', { name: 'Confirm phone binding' })
  ).toBeEnabled()
  await user.clear(phone)
  await user.type(phone, '13900139000')
  expect(
    screen.getByRole('button', { name: 'Confirm phone binding' })
  ).toBeDisabled()
  expect(screen.getByLabelText('SMS verification code')).toHaveValue('')
  expect(phoneAPI.sendBindingSMS).toHaveBeenCalledWith(
    '13800138000',
    'current-session-proof',
    undefined
  )
})

test('successful administrator verification binds the selected user and refreshes the displayed status', async () => {
  const user = userEvent.setup()
  const changed = vi.fn()
  await show(() => <PhoneBindingCard userId={42} onChanged={changed} />)
  await user.click(
    await screen.findByRole('button', { name: 'Verify phone number' })
  )
  await user.type(
    screen.getByLabelText('Current account password'),
    'admin-password'
  )
  await user.click(screen.getByRole('button', { name: /^Verify$/ }))
  await screen.findByLabelText('New phone number (+86)')
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  await user.type(screen.getByLabelText('SMS verification code'), '123456')
  vi.mocked(phoneAPI.getPhoneStatus).mockResolvedValue({
    phone: '+8613800138000',
    verified: {
      phone: '+8613800138000',
      source: 'aliyun_sms',
      verified_at: '2026-10-06T00:00:00Z',
    },
    sms_enabled: true,
  })
  await user.click(
    screen.getByRole('button', { name: 'Confirm phone binding' })
  )
  await screen.findByRole('button', { name: 'Change phone number' })
  expect(phoneAPI.confirmPhoneBinding).toHaveBeenCalledWith(
    'binding-challenge',
    '123456',
    'current-session-proof',
    42
  )
  expect(changed).toHaveBeenCalledOnce()
})

test('SMS unavailable status keeps the binding action disabled', async () => {
  vi.mocked(phoneAPI.getPhoneStatus).mockResolvedValue({
    phone: '',
    verified: null,
    sms_enabled: false,
  })
  await show(() => <PhoneBindingCard />)
  expect(
    await screen.findByRole('button', { name: 'Verify phone number' })
  ).toBeDisabled()
  expect(screen.getByText(/SMS binding is not configured yet\./)).toBeVisible()
})

test('failed SMS delivery never enables sign-in', async () => {
  vi.mocked(phoneAPI.sendLoginSMS).mockRejectedValue({
    response: { data: { code: 'PHONE_SMS_UNAVAILABLE' } },
  })
  const user = userEvent.setup()
  await show(() => <SmsLoginForm />)
  await user.type(await screen.findByLabelText('Phone Number'), '13800138000')
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'SMS service is unavailable. Use another sign-in method.'
  )
  expect(
    screen.getByRole('button', { name: 'Verify and Sign In' })
  ).toBeDisabled()
})

test('MFA SMS uses the password login flow and can switch back to the authenticator', async () => {
  useAuthStore
    .getState()
    .auth.setPending2FAFlowToken('password-login-flow', true)
  const user = userEvent.setup()
  await show(() => <OtpForm />)
  await user.click(await screen.findByRole('button', { name: 'Use SMS code' }))
  expect(screen.queryByLabelText('Phone Number')).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  await waitFor(() =>
    expect(phoneAPI.sendLoginSMS).toHaveBeenCalledWith({
      phone: undefined,
      flow_token: 'password-login-flow',
      turnstile: '',
    })
  )
  expect(screen.getByRole('button', { name: 'Resend in 60s' })).toBeDisabled()
  await user.click(
    screen.getByRole('button', { name: 'Use authenticator code' })
  )
  expect(screen.getByRole('button', { name: 'Use backup code' })).toBeVisible()
})
