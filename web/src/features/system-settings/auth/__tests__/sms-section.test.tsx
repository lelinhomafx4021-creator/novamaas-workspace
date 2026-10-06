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
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { SMSSection, type SMSConfiguration } from '../sms-section'

let queryClient: QueryClient
const defaults: SMSConfiguration = {
  settings: {
    enabled: false,
    sign_name: 'approved-signature',
    template_code: 'SMS_TEST',
    code_parameter: 'code',
    access_key_id_configured: true,
    access_key_secret_configured: true,
    security_token_configured: false,
  },
  effective: {
    enabled: false,
    ready: false,
    credentials_complete: true,
    sign_name: 'approved-signature',
    template_code: 'SMS_TEST',
    code_parameter: 'code',
    access_key_id_configured: true,
    access_key_secret_configured: true,
    security_token_configured: false,
  },
  environment_overrides: {},
}
function SMSSettingsPage() {
  const [actionsContainer, setActionsContainer] =
    useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActionsContainer} />
      <SettingsPageProvider actionsContainer={actionsContainer}>
        <SMSSection />
      </SettingsPageProvider>
    </>
  )
}
async function show(configuration = defaults) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: configuration },
  })
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: SMSSettingsPage }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByLabelText('SMS signature')
}
afterEach(() => {
  queryClient?.clear()
  vi.restoreAllMocks()
})

test('SMS configuration saves atomically and omits unchanged write-only credentials', async () => {
  const user = userEvent.setup()
  const put = vi.spyOn(api, 'put').mockResolvedValue({
    data: {
      success: true,
      data: {
        ...defaults,
        settings: { ...defaults.settings, enabled: true },
        effective: { ...defaults.effective, enabled: true, ready: true },
      },
    },
  })
  await show()
  expect(screen.getByLabelText('AccessKey Secret')).toHaveValue('')
  expect(screen.getAllByRole('button', { name: 'Save Changes' })).toHaveLength(
    1
  )
  await user.click(screen.getByRole('switch', { name: 'Enable SMS service' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/sms', {
      enabled: true,
      sign_name: 'approved-signature',
      template_code: 'SMS_TEST',
      code_parameter: 'code',
      clear_security_token: false,
    })
  )
  await waitFor(() =>
    expect(screen.getByRole('status')).toHaveTextContent('Ready')
  )
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeDisabled()
  expect(screen.getByText('Ready')).toHaveClass(
    'text-green-600',
    'dark:text-green-400'
  )
})

test('deployment overrides remain locked and are omitted when saving other fields', async () => {
  const user = userEvent.setup()
  const configuration = {
    ...defaults,
    environment_overrides: {
      enabled: 'SMS_ENABLED',
      access_key_secret: 'ALIYUN_SMS_ACCESS_KEY_SECRET',
    },
  }
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, data: configuration } })
  await show(configuration)
  expect(
    screen.getByRole('switch', { name: 'Enable SMS service' })
  ).toHaveAttribute('aria-disabled', 'true')
  expect(screen.getByLabelText('AccessKey Secret')).toBeDisabled()
  expect(screen.getByText('Configured by SMS_ENABLED')).toBeVisible()
  await user.clear(screen.getByLabelText('SMS signature'))
  await user.type(screen.getByLabelText('SMS signature'), 'new-signature')
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/sms', {
      sign_name: 'new-signature',
      template_code: 'SMS_TEST',
      code_parameter: 'code',
      clear_security_token: false,
    })
  )
})

test('failed enablement keeps entered credentials available for correction and shows the failure', async () => {
  const user = userEvent.setup()
  vi.spyOn(api, 'put').mockRejectedValue({
    response: { data: { code: 'SMS_CONFIG_INCOMPLETE' } },
  })
  const failure = vi.spyOn(toast, 'error')
  await show()
  await user.type(screen.getByLabelText('AccessKey Secret'), 'new-secret')
  await user.click(screen.getByRole('switch', { name: 'Enable SMS service' }))
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(failure).toHaveBeenCalledWith(
      'Complete the access keys, SMS signature and template before enabling SMS.'
    )
  )
  expect(screen.getByLabelText('AccessKey Secret')).toHaveValue('new-secret')
  expect(screen.getByRole('button', { name: 'Save Changes' })).toBeEnabled()
})

test('explicit STS removal takes precedence over a typed replacement token', async () => {
  const user = userEvent.setup()
  const configuration = {
    ...defaults,
    settings: { ...defaults.settings, security_token_configured: true },
    effective: { ...defaults.effective, security_token_configured: true },
  }
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true, data: defaults } })
  await show(configuration)
  await user.type(
    screen.getByLabelText('STS security token (optional)'),
    'replacement-token'
  )
  await user.click(
    screen.getByRole('switch', { name: 'Remove saved STS token' })
  )
  expect(screen.getByLabelText('STS security token (optional)')).toBeDisabled()
  await user.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith('/api/option/sms', {
      enabled: false,
      sign_name: 'approved-signature',
      template_code: 'SMS_TEST',
      code_parameter: 'code',
      clear_security_token: true,
    })
  )
  await waitFor(() =>
    expect(
      screen.queryByRole('switch', { name: 'Remove saved STS token' })
    ).not.toBeInTheDocument()
  )
})
