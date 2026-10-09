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
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import {
  getBillingAccount,
  getBillingDay,
  searchBillingAccounts,
  saveBillingAccount,
} from '../api'
import { Billing } from '../index'

vi.mock('../api', async (original) => ({
  ...(await original<typeof import('../api')>()),
  getBillingAccount: vi.fn(),
  getBillingDay: vi.fn(),
  searchBillingAccounts: vi.fn(),
  saveBillingAccount: vi.fn(),
}))

afterEach(() => {
  useAuthStore.getState().auth.reset()
  vi.clearAllMocks()
})

test('formal accounting accounts display their company title in the account selector', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  const customer = {
    id: 4,
    username: 'formal_customer',
    display_name: 'Customer',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    accounting_start_at: 100,
    company_title: 'Customer Ltd',
  }
  vi.mocked(searchBillingAccounts).mockResolvedValue({
    items: [customer],
  })
  vi.mocked(getBillingDay).mockRejectedValue(new Error('No usage fixture'))
  vi.mocked(getBillingAccount).mockImplementation(async (id) => ({
    user_id: id,
    accounting_start_at: id === 4 ? 100 : 0,
    company_title: id === 4 ? 'Customer Ltd' : '',
    tax_id: '',
    profile_version: 1,
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Billing />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('combobox', { name: 'Account' }))
  const option = await screen.findByRole('option', {
    name: 'formal_customer (#4) (Formal accounting - Customer Ltd)',
  })
  await user.click(option)
  expect(screen.getByRole('combobox', { name: 'Account' })).toHaveTextContent(
    'formal_customer (#4) (Formal accounting - Customer Ltd)'
  )
})

test('the current administrator shows its formal accounting title even outside search results', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  vi.mocked(searchBillingAccounts).mockResolvedValue({ items: [] })
  vi.mocked(getBillingDay).mockRejectedValue(new Error('No usage fixture'))
  vi.mocked(getBillingAccount).mockResolvedValue({
    user_id: 1,
    accounting_start_at: 100,
    company_title: 'Admin Company',
    tax_id: 'TAX',
    profile_version: 1,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Billing />
    </QueryClientProvider>
  )
  await waitFor(() =>
    expect(screen.getByRole('combobox', { name: 'Account' })).toHaveTextContent(
      'admin (#1) (Formal accounting - Admin Company)'
    )
  )
})

test('saving a selected account title refreshes its formal label after search results exclude it', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  let title = 'Original Company'
  const customer = {
    id: 4,
    username: 'customer',
    accounting_start_at: 100,
    company_title: title,
  }
  vi.mocked(searchBillingAccounts).mockResolvedValue({ items: [customer] })
  vi.mocked(getBillingDay).mockRejectedValue(new Error('No usage fixture'))
  vi.mocked(getBillingAccount).mockImplementation(async (id) => ({
    user_id: id,
    accounting_start_at: id === 4 ? 100 : 0,
    company_title: id === 4 ? title : '',
    tax_id: 'TAX',
    profile_version: 1,
  }))
  vi.mocked(saveBillingAccount).mockImplementation(async (id, input) => {
    title = input.company_title ?? title
    return {
      user_id: id,
      accounting_start_at: 100,
      company_title: title,
      tax_id: 'TAX',
      profile_version: 2,
    }
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Billing />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('combobox', { name: 'Account' }))
  await user.click(
    await screen.findByRole('option', {
      name: 'customer (#4) (Formal accounting - Original Company)',
    })
  )
  vi.mocked(searchBillingAccounts).mockResolvedValue({ items: [] })
  fireEvent.change(screen.getByLabelText('Search accounts'), {
    target: { value: 'absent' },
  })
  await waitFor(() =>
    expect(searchBillingAccounts).toHaveBeenCalledWith('absent')
  )
  await user.click(screen.getByRole('tab', { name: 'Billing identity' }))
  const field = await screen.findByLabelText('Company title')
  await user.clear(field)
  await user.type(field, 'Updated Company')
  await user.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() =>
    expect(screen.getByRole('combobox', { name: 'Account' })).toHaveTextContent(
      'customer (#4) (Formal accounting - Updated Company)'
    )
  )
})

test('a selected customer keeps its username and ownership after account search results change', async () => {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'admin', role: 100 })
  const customer = {
    id: 4,
    username: 'example_customer',
    display_name: 'Customer',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    accounting_start_at: 0,
    company_title: 'Customer',
  }
  vi.mocked(searchBillingAccounts).mockResolvedValue({
    items: [customer],
  })
  vi.mocked(getBillingDay).mockRejectedValue(new Error('No usage fixture'))
  vi.mocked(getBillingAccount).mockResolvedValue({
    user_id: 4,
    accounting_start_at: 0,
    company_title: 'Customer',
    tax_id: 'TAX',
    profile_version: 1,
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Billing />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.click(screen.getByRole('combobox', { name: 'Account' }))
  await user.click(
    await screen.findByRole('option', { name: 'example_customer (#4)' })
  )
  expect(
    screen.getByText('Selected billing customer: example_customer (#4)')
  ).toBeVisible()
  vi.mocked(searchBillingAccounts).mockResolvedValue({
    items: [{ ...customer, id: 5, username: 'another_customer' }],
  })
  fireEvent.change(screen.getByLabelText('Search accounts'), {
    target: { value: 'another' },
  })
  await waitFor(() =>
    expect(searchBillingAccounts).toHaveBeenCalledWith('another')
  )
  await user.click(screen.getByRole('combobox', { name: 'Account' }))
  expect(
    await screen.findByRole('option', { name: 'another_customer (#5)' })
  ).toBeVisible()
  await user.keyboard('{Escape}')
  expect(screen.getByRole('combobox', { name: 'Account' })).toHaveTextContent(
    'example_customer (#4)'
  )
  await user.click(screen.getByRole('tab', { name: 'Billing identity' }))
  await screen.findByLabelText('Company title')
  expect(getBillingAccount).toHaveBeenCalledWith(4)
  expect(
    screen.getByText('Selected billing customer: example_customer (#4)')
  ).toBeVisible()
})
