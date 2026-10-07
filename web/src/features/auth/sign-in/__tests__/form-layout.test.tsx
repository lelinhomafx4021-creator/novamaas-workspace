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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import type { SystemStatus } from '@/features/auth/types'
import { api } from '@/lib/api'

import { UserAuthForm } from '../components/user-auth-form'

let queryClient: QueryClient

async function renderForm(status: Partial<SystemStatus>) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  queryClient.setQueryData(['status'], status)

  const rootRoute = createRootRoute({ component: UserAuthForm })
  const router = createRouter({
    routeTree: rootRoute,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })

  await router.load()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

afterEach(() => {
  queryClient?.clear()
  localStorage.clear()
})

test('legal consent appears before sign-in actions and unlocks them when accepted', async () => {
  await renderForm({
    password_login_enabled: true,
    user_agreement_enabled: true,
  })

  const user = userEvent.setup()
  const password = screen.getByLabelText('Password')
  const legalConsent = screen.getByRole('checkbox')
  const submit = screen.getByRole('button', { name: 'Sign in' })

  expect(password.compareDocumentPosition(legalConsent)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
  expect(legalConsent.compareDocumentPosition(submit)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
  expect(submit).toBeDisabled()

  await user.click(legalConsent)
  expect(submit).toBeEnabled()
})

test('alternative providers follow the password action under one separator', async () => {
  await renderForm({
    github_oauth: true,
    password_login_enabled: true,
  })

  const submit = screen.getByRole('button', { name: 'Sign in' })
  const github = screen.getByRole('button', { name: 'Continue with GitHub' })

  expect(submit.compareDocumentPosition(github)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
  expect(screen.getAllByText('Or continue with')).toHaveLength(1)
})

test('password sign-in accepts a username, email address, or phone number', async () => {
  await renderForm({ password_login_enabled: true })

  expect(
    screen.getByRole('textbox', {
      name: 'Username, Email or Phone Number',
    })
  ).toHaveAttribute('placeholder', 'Enter your username, email or phone number')
})

test('SMS tab shows phone input inline and remembers the selected login method', async () => {
  localStorage.clear()
  await renderForm({ password_login_enabled: true, sms_login: true })
  await userEvent
    .setup()
    .click(screen.getByRole('tab', { name: 'SMS sign-in' }))
  expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()
  expect(
    screen.getByRole('textbox', { name: 'Phone Number' })
  ).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(localStorage.getItem('new-api:login-method:v1')).toBe('sms')
})

test('invalid saved login method falls back to account password', async () => {
  localStorage.setItem('new-api:login-method:v1', 'invalid')
  await renderForm({ password_login_enabled: true, sms_login: true })
  expect(
    screen.getByRole('tab', { name: 'Account and password' })
  ).toHaveAttribute('aria-selected', 'true')
})

test('opening sign-in again restores the saved SMS tab', async () => {
  localStorage.setItem('new-api:login-method:v1', 'sms')
  await renderForm({ password_login_enabled: true, sms_login: true })
  expect(screen.getByRole('tab', { name: 'SMS sign-in' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(
    screen.getByRole('textbox', { name: 'Phone Number' })
  ).toBeInTheDocument()
  expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()
})

test('keyboard navigation switches login tabs without opening a phone dialog', async () => {
  await renderForm({ password_login_enabled: true, sms_login: true })
  screen.getByRole('tab', { name: 'Account and password' }).focus()
  await userEvent.setup().keyboard('[ArrowRight][Enter]')
  expect(screen.getByRole('tab', { name: 'SMS sign-in' })).toHaveAttribute(
    'aria-selected',
    'true'
  )
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('WeChat tab opens the official authorization frame without a nested dialog', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      data: {
        authorize_url:
          'https://open.weixin.qq.com/connect/qrconnect?state=test',
      },
    },
  })
  await renderForm({ password_login_enabled: true, wechat_web_login: true })
  const user = userEvent.setup()
  await user.click(screen.getByRole('tab', { name: 'WeChat sign in' }))
  await user.click(screen.getByRole('button', { name: 'Continue with WeChat' }))
  expect(await screen.findByTitle('WeChat sign in')).toHaveAttribute(
    'src',
    'https://open.weixin.qq.com/connect/qrconnect?state=test'
  )
  expect(api.post).toHaveBeenCalledWith('/api/oauth/wechat-web/start')
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

test('a saved disabled SMS method falls back to account password', async () => {
  localStorage.setItem('new-api:login-method:v1', 'sms')
  await renderForm({ password_login_enabled: true, sms_login: false })
  expect(
    screen.getByRole('tab', { name: 'Account and password' })
  ).toHaveAttribute('aria-selected', 'true')
})
