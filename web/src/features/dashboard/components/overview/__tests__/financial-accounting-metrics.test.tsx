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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { FinancialAccountingMetrics } from '../financial-accounting-metrics'

vi.mock('@/lib/format', () => ({
  formatQuota: (value: number) => `quota:${value}`,
}))

describe('financial accounting overview metrics', () => {
  beforeAll(async () => {
    i18next.addResourceBundle('en', 'translation', {
      "Today's turnover": "Today's turnover",
      "Today's profit": "Today's profit",
      "Yesterday's turnover": "Yesterday's turnover",
      "Yesterday's profit": "Yesterday's profit",
      'Pending accounting': 'Pending accounting',
      '{{count}} cost records are missing.':
        '{{count}} cost records are missing.',
    })
    await i18next.changeLanguage('en')
  })

  test('shows four natural-day metrics and withholds incomplete profit', () => {
    render(
      <FinancialAccountingMetrics
        overview={{
          today: {
            start_timestamp: 100,
            end_timestamp: 200,
            revenue_quota: 800,
            cost_quota: 600,
            profit_quota: 200,
            usage_records: 2,
            cost_records: 2,
            missing_cost_records: 0,
            accounting_complete: true,
          },
          yesterday: {
            start_timestamp: 1,
            end_timestamp: 99,
            revenue_quota: 900,
            cost_quota: 0,
            profit_quota: null,
            usage_records: 1,
            cost_records: 0,
            missing_cost_records: 1,
            accounting_complete: false,
          },
        }}
      />
    )

    expect(screen.getByText("Today's turnover")).toBeInTheDocument()
    expect(screen.getByText("Today's profit")).toBeInTheDocument()
    expect(screen.getByText("Yesterday's turnover")).toBeInTheDocument()
    expect(screen.getByText("Yesterday's profit")).toBeInTheDocument()
    expect(screen.getByText('quota:800')).toBeInTheDocument()
    expect(screen.getByText('quota:200')).toBeInTheDocument()
    expect(screen.getByText('quota:900')).toBeInTheDocument()
    expect(screen.getByText('Pending accounting')).toHaveAttribute(
      'title',
      '1 cost records are missing.'
    )
  })
})
