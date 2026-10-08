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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import * as phoneAPI from '@/features/phone/api'
import { api } from '@/lib/api'

import { UsersMutateDrawer } from '../components/users-mutate-drawer'
import { UsersProvider } from '../components/users-provider'
import {
  transformFormDataToPayload,
  transformUserToFormDefaults,
  USER_FORM_DEFAULT_VALUES,
  userFormSchema,
} from '../lib/user-form'
import type { User } from '../types'

const apiMocks = vi.hoisted(() => ({
  adjustUserQuota: vi.fn(async () => ({ success: true })),
  createUser: vi.fn(async () => ({ success: true })),
  getGroups: vi.fn(async () => ({ success: true, data: ['default'] })),
  getPermissionCatalog: vi.fn(async () => ({ resources: [], roles: [] })),
  getUser: vi.fn(),
  updateUser: vi.fn(async () => ({ success: true })),
}))

vi.mock('../api', () => apiMocks)

let queryClient: QueryClient

beforeEach(() => {
  vi.spyOn(phoneAPI, 'getPhoneStatus').mockResolvedValue({
    phone: '',
    verified: null,
    sms_enabled: true,
  })
  vi.spyOn(phoneAPI, 'sendUserMutationSMS').mockResolvedValue({
    challenge_token: 'phone-challenge',
    expires_in: 300,
    retry_after: 60,
  })
})

afterEach(() => {
  vi.clearAllMocks()
  vi.restoreAllMocks()
  queryClient?.clear()
})

test('create-user form exposes a phone number field', async () => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <UsersMutateDrawer open onOpenChange={vi.fn()} />
      </UsersProvider>
    </QueryClientProvider>
  )

  expect(await screen.findByLabelText('Phone Number')).toHaveAttribute(
    'type',
    'tel'
  )
})

test('create-user payload trims and preserves the phone number', () => {
  const values = userFormSchema.parse({
    ...USER_FORM_DEFAULT_VALUES,
    username: 'phone-create-user',
    password: 'NewPassword123',
    phone: ' 13800138000 ',
  })

  expect(transformFormDataToPayload(values)).toMatchObject({
    username: 'phone-create-user',
    phone: '+8613800138000',
  })
})

test('edit-user form loads the phone number and can clear it', () => {
  const user: User = {
    id: 1,
    username: 'phone-edit-user',
    display_name: 'Phone Edit User',
    phone: '13800138000',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
  }
  const defaults = transformUserToFormDefaults(user)

  expect(defaults.phone).toBe('13800138000')
  expect(
    transformFormDataToPayload({ ...defaults, phone: '' }, user.id)
  ).toMatchObject({ id: user.id, phone: '' })
})

const existingUser: User = {
  id: 42,
  username: 'phone-edit-user',
  display_name: 'Before',
  phone: '+8613800138000',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  group: 'default',
  status: 1,
  role: 1,
}

async function showUserForm(currentRow?: User) {
  if (currentRow) {
    apiMocks.getUser.mockResolvedValue({ success: true, data: currentRow })
  }
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <UsersMutateDrawer
          open
          currentRow={currentRow}
          onOpenChange={vi.fn()}
        />
      </UsersProvider>
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(screen.getByLabelText('Username')).toHaveValue(
      currentRow?.username || ''
    )
  )
}

async function requestPhoneCode() {
  await userEvent.click(screen.getByRole('button', { name: 'Send SMS code' }))
  expect(
    screen.queryByRole('button', { name: 'Verify current phone' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByLabelText('Current account password')
  ).not.toBeInTheDocument()
  return screen.findByLabelText('SMS verification code')
}

test('create-user phone is verified at save and prefix stays outside the editable value', async () => {
  const user = userEvent.setup()
  await showUserForm()
  await user.type(screen.getByLabelText('Username'), 'phone-create-user')
  await user.type(screen.getByLabelText('Password'), 'NewPassword123')
  await user.type(screen.getByLabelText('Phone Number'), '13800138000')
  expect(screen.getByLabelText('Country calling code')).toHaveTextContent('+86')
  expect(screen.getByLabelText('Phone Number')).toHaveValue('13800138000')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(
    await screen.findByText(
      'Request a code for this phone number and enter it before saving.'
    )
  ).toBeVisible()
  expect(apiMocks.createUser).not.toHaveBeenCalled()
  const code = await requestPhoneCode()
  expect(phoneAPI.sendUserMutationSMS).toHaveBeenCalledWith({
    phone: '+8613800138000',
    user_id: undefined,
    username: 'phone-create-user',
  })
  await user.type(code, '123456')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() =>
    expect(apiMocks.createUser).toHaveBeenCalledWith(
      expect.objectContaining({
        phone: '+8613800138000',
        phone_challenge_token: 'phone-challenge',
        phone_verification_code: '123456',
      })
    )
  )
})

test('changing the destination or draft username invalidates the collected phone code', async () => {
  const user = userEvent.setup()
  await showUserForm()
  await user.type(screen.getByLabelText('Username'), 'phone-create-user')
  await user.type(screen.getByLabelText('Password'), 'NewPassword123')
  await user.type(screen.getByLabelText('Phone Number'), '13800138000')
  await user.type(await requestPhoneCode(), '123456')
  await user.type(screen.getByLabelText('Username'), '-new')
  expect(
    screen.queryByLabelText('SMS verification code')
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(apiMocks.createUser).not.toHaveBeenCalled()
  await user.clear(screen.getByLabelText('Phone Number'))
  await user.type(screen.getByLabelText('Phone Number'), '13900139000')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(apiMocks.createUser).not.toHaveBeenCalled()
})

test('an unchanged bound number saves other profile fields without sending SMS', async () => {
  vi.mocked(phoneAPI.getPhoneStatus).mockResolvedValue({
    phone: '+8613800138000',
    verified: {
      phone: '+8613800138000',
      source: 'aliyun_sms',
      verified_at: '2026-10-06T00:00:00Z',
    },
    sms_enabled: true,
  })
  const user = userEvent.setup()
  await showUserForm(existingUser)
  expect(screen.getByLabelText('Phone Number')).toHaveValue('13800138000')
  expect(await screen.findByText('Bound')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Send SMS code' })).toBeDisabled()
  await user.clear(screen.getByLabelText('Display Name'))
  await user.type(screen.getByLabelText('Display Name'), 'After')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  await waitFor(() =>
    expect(apiMocks.updateUser).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 42,
        phone: '+8613800138000',
        display_name: 'After',
      })
    )
  )
  expect(phoneAPI.sendUserMutationSMS).not.toHaveBeenCalled()
})

test('editing a bound number requires a new code and keeps a rejected code visible for correction', async () => {
  vi.mocked(phoneAPI.getPhoneStatus).mockResolvedValue({
    phone: '+8613800138000',
    verified: {
      phone: '+8613800138000',
      source: 'aliyun_sms',
      verified_at: '2026-10-06T00:00:00Z',
    },
    sms_enabled: true,
  })
  apiMocks.updateUser.mockRejectedValueOnce({
    response: { data: { code: 'PHONE_VERIFICATION_INVALID' } },
  })
  const user = userEvent.setup()
  await showUserForm(existingUser)
  await user.clear(screen.getByLabelText('Phone Number'))
  await user.type(screen.getByLabelText('Phone Number'), '13900139000')
  expect(screen.getByText('Phone verification pending')).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(apiMocks.updateUser).not.toHaveBeenCalled()
  await user.type(await requestPhoneCode(), '123456')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(
    await screen.findByText(
      'Phone verification failed. Check the code or request a new one.'
    )
  ).toBeVisible()
  expect(screen.getByLabelText('Phone Number')).toHaveValue('13900139000')
  expect(screen.getByLabelText('SMS verification code')).toHaveValue('123456')
  expect(apiMocks.updateUser).toHaveBeenCalledWith(
    expect.objectContaining({
      id: 42,
      phone: '+8613900139000',
      phone_challenge_token: 'phone-challenge',
      phone_verification_code: '123456',
    })
  )
})

test('a late SMS response cannot verify a renamed user draft', async () => {
  let deliver: ((challenge: phoneAPI.SmsChallenge) => void) | undefined
  vi.mocked(phoneAPI.sendUserMutationSMS).mockReturnValue(
    new Promise((resolve) => {
      deliver = resolve
    })
  )
  const user = userEvent.setup()
  await showUserForm()
  await user.type(screen.getByLabelText('Username'), 'phone-create-user')
  await user.type(screen.getByLabelText('Password'), 'NewPassword123')
  await user.type(screen.getByLabelText('Phone Number'), '13800138000')
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  await waitFor(() =>
    expect(phoneAPI.sendUserMutationSMS).toHaveBeenCalledOnce()
  )
  await user.type(screen.getByLabelText('Username'), '-new')
  expect(deliver).toBeDefined()
  deliver?.({
    challenge_token: 'late-challenge',
    expires_in: 300,
    retry_after: 60,
  })
  await screen.findByRole('button', { name: 'Resend in 60s' })
  expect(
    screen.queryByLabelText('SMS verification code')
  ).not.toBeInTheDocument()
  await user.click(screen.getByRole('button', { name: 'Save changes' }))
  expect(apiMocks.createUser).not.toHaveBeenCalled()
})

test('edit-user drawer shows mini program binding independently of the legacy WeChat ID', async () => {
  apiMocks.getUser.mockResolvedValue({ success: true, data: existingUser })
  const request = vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: [
        {
          app_id: 'wx-app',
          nickname: 'Bound WeChat Name',
          has_avatar: false,
          openid: 'mini-openid',
          unionid: 'mini-unionid',
          bound_at: '2026-10-06T00:00:00Z',
          last_login_at: '2026-10-06T01:00:00Z',
        },
      ],
    },
  })
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <UsersMutateDrawer
          open
          currentRow={existingUser}
          onOpenChange={vi.fn()}
        />
      </UsersProvider>
    </QueryClientProvider>
  )
  expect(
    (await screen.findAllByText('Bound WeChat Name')).length
  ).toBeGreaterThan(0)
  expect(request).toHaveBeenCalledWith('/api/user/42/wechat-miniapp')
  expect(screen.getByText('OpenID: mini-openid')).toBeInTheDocument()
  await userEvent.click(
    screen.getByRole('button', { name: 'Manage WeChat binding' })
  )
  expect(
    await screen.findByRole('button', { name: 'Disconnect WeChat' })
  ).toBeInTheDocument()
})

test('a failed SMS request shows an error and allows retry without account verification', async () => {
  vi.mocked(phoneAPI.sendUserMutationSMS).mockRejectedValueOnce({
    response: { data: { code: 'PHONE_SMS_UNAVAILABLE' } },
  })
  const user = userEvent.setup()
  await showUserForm()
  await user.type(screen.getByLabelText('Username'), 'phone-create-user')
  await user.type(screen.getByLabelText('Phone Number'), '13800138000')
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'SMS service is unavailable. Use another sign-in method.'
  )
  expect(
    screen.queryByLabelText('Current account password')
  ).not.toBeInTheDocument()
  expect(
    screen.queryByLabelText('SMS verification code')
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Send SMS code' })).toBeEnabled()
  await user.click(screen.getByRole('button', { name: 'Send SMS code' }))
  expect(await screen.findByLabelText('SMS verification code')).toBeVisible()
  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
