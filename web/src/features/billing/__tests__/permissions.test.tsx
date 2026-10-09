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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { getBillingAccount, getBillingDay, searchBillingAccounts } from '../api'
import { listCorrections } from '../correction-api'
import { Billing } from '../index'

vi.mock('../api', async (original) => ({
  ...(await original<typeof import('../api')>()),
  getBillingDay: vi.fn(),
  getBillingAccount: vi.fn(),
  searchBillingAccounts: vi.fn(),
}))
vi.mock('../correction-api', async (original) => ({
  ...(await original<typeof import('../correction-api')>()),
  listCorrections: vi.fn(),
}))

afterEach(() => {
  useAuthStore.getState().auth.reset()
  vi.resetAllMocks()
})

test.each([
  { role: 1, finance: false, switchAccount: false, adjustments: false },
  { role: 1, finance: true, switchAccount: false, adjustments: false },
  { role: 10, finance: false, switchAccount: false, adjustments: false },
  { role: 10, finance: true, switchAccount: true, adjustments: true },
  { role: 100, finance: false, switchAccount: true, adjustments: true },
])(
  'role $role and financial capability $finance control account switching and adjustment visibility',
  async (scenario) => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'viewer',
      role: scenario.role,
      permissions: {
        admin_permissions: { financial_accounting: { view: scenario.finance } },
      },
    })
    vi.mocked(getBillingDay).mockRejectedValue(new Error('No usage'))
    vi.mocked(getBillingAccount).mockResolvedValue({
      user_id: 1,
      company_title: '',
      tax_id: '',
      profile_version: 1,
      accounting_start_at: 0,
    })
    vi.mocked(searchBillingAccounts).mockResolvedValue({ items: [] })
    vi.mocked(listCorrections).mockResolvedValue([])
    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <Billing />
      </QueryClientProvider>
    )
    expect(Boolean(screen.queryByRole('combobox', { name: 'Account' }))).toBe(
      scenario.switchAccount
    )
    expect(
      Boolean(screen.queryByRole('tab', { name: 'Billing adjustments' }))
    ).toBe(scenario.adjustments)
    if (!scenario.switchAccount) {
      expect(searchBillingAccounts).not.toHaveBeenCalled()
    }
    if (scenario.adjustments) {
      fireEvent.click(screen.getByRole('tab', { name: 'Billing adjustments' }))
      expect(
        await screen.findByRole('table', { name: 'Adjustment history' })
      ).toBeVisible()
      expect(
        Boolean(screen.queryByRole('button', { name: 'Preview adjustment' }))
      ).toBe(scenario.role === 100)
    }
  }
)
