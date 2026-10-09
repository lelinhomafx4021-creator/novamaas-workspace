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
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { fireEvent, render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { usageLogSchema, type UsageLog } from '../../../data/schema'
import type { LogOtherData } from '../../../types'
import { UsageLogsProvider } from '../../usage-logs-provider'
import { useCommonLogsColumns } from '../common-logs-columns'

// The external icon bundle imports JSON using browser-specific module handling.
vi.mock('@lobehub/icons', () => ({}))

const tiers = [
  {
    model: 'doubao-seedance-2-0',
    resolution: '480p',
    base: 46,
    price: 46,
    video: false,
  },
  {
    model: 'doubao-seedance-2-0',
    resolution: '720p',
    base: 46,
    price: 28,
    video: true,
  },
  {
    model: 'doubao-seedance-2-0',
    resolution: '1080p',
    base: 46,
    price: 51,
    video: false,
  },
  {
    model: 'doubao-seedance-2-0-260128',
    resolution: '1080p',
    base: 46,
    price: 31,
    video: true,
  },
  {
    model: 'doubao-seedance-2-0',
    resolution: '4k',
    base: 46,
    price: 26,
    video: false,
  },
  {
    model: 'doubao-seedance-2-0',
    resolution: '4k',
    base: 46,
    price: 16,
    video: true,
  },
  {
    model: 'doubao-seedance-2-5',
    resolution: '480p',
    base: 70,
    price: 70,
    video: false,
  },
  {
    model: 'doubao-seedance-2-5',
    resolution: '720p',
    base: 70,
    price: 42,
    video: true,
  },
  {
    model: 'doubao-seedance-2-5',
    resolution: '1080p',
    base: 70,
    price: 77,
    video: false,
  },
  {
    model: 'doubao-seedance-2-5-260628',
    resolution: '1080p',
    base: 70,
    price: 46,
    video: true,
  },
]

function PricingRow(props: { log: UsageLog }) {
  const columns = useCommonLogsColumns(false).filter(
    (column) => 'accessorKey' in column && column.accessorKey === 'content'
  )
  const table = useReactTable({
    data: [props.log],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  return (
    <table>
      <tbody>
        {table.getRowModel().rows.map((row) => (
          <tr key={row.id}>
            {row.getVisibleCells().map((cell) => (
              <td key={cell.id}>
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function renderPricing(other: LogOtherData, model = 'doubao-seedance-2-5') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const log = usageLogSchema.parse({
    id: 1,
    user_id: 4,
    created_at: 1,
    type: 2,
    model_name: model,
    content: 'generate',
    quota: 100,
    other: JSON.stringify(other),
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <UsageLogsProvider>
        <PricingRow log={log} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

describe('video pricing in usage log previews and customer details', () => {
  const originalCurrency = {
    ...useSystemConfigStore.getState().config.currency,
  }
  beforeEach(async () => {
    await i18next.changeLanguage('en')
    useSystemConfigStore.getState().setConfig({
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      },
    })
  })
  afterEach(() => {
    useSystemConfigStore.getState().setConfig({ currency: originalCurrency })
  })

  test.each(tiers)(
    '$model $resolution video=$video shows its saved live tier price in both views',
    (tier) => {
      renderPricing(
        {
          is_task: true,
          model_price: -1,
          model_ratio: tier.base / 14,
          group_ratio: 0.86,
          other_ratios: { video_input: tier.price / tier.base },
        },
        tier.model
      )

      const preview = screen.getByRole('button', {
        name: `Standard · ¥${tier.price}/M`,
      })
      fireEvent.click(preview)
      const dialog = screen.getByRole('dialog')
      expect(within(dialog).getByText(`¥${tier.price}/M`)).toBeVisible()
      expect(within(dialog).getByText('Video pricing multiplier')).toBeVisible()
    }
  )

  test('a historical settlement without is_task or resolution uses the precise top-level multiplier', () => {
    renderPricing({ task_id: 'task_old', model_ratio: 5, video_input: 46 / 70 })
    fireEvent.click(screen.getByRole('button', { name: 'Standard · ¥46/M' }))
    expect(within(screen.getByRole('dialog')).getByText('¥46/M')).toBeVisible()
  })

  test('a correction back to the base tier overrides the old top-level multiplier with an empty map', () => {
    renderPricing({
      task_id: 'task_old',
      billing_correction_applied: true,
      model_ratio: 5,
      resolution: '720p',
      video_input: 1.1,
      other_ratios: {},
    })
    fireEvent.click(screen.getByRole('button', { name: 'Standard · ¥70/M' }))
    expect(within(screen.getByRole('dialog')).getByText('¥70/M')).toBeVisible()
  })

  test('a corrected 1080p tier uses the structured multiplier once instead of the old video discount', () => {
    renderPricing({
      task_id: 'task_old',
      billing_correction_applied: true,
      model_ratio: 5,
      resolution: '1080p',
      has_video: false,
      video_input: 0.6,
      other_ratios: { video_input: 1.1 },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Standard · ¥77/M' }))
    expect(within(screen.getByRole('dialog')).getByText('¥77/M')).toBeVisible()
  })

  test('a log without a precise saved multiplier keeps its historical base price', () => {
    renderPricing({ is_task: true, model_ratio: 5 })
    expect(
      screen.getByRole('button', { name: 'Standard · ¥70/M' })
    ).toBeVisible()
  })
})
