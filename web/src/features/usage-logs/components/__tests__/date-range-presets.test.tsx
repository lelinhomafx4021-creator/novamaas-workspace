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
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test, vi } from 'vitest'

import { buildApiParams, buildBaseParams } from '../../lib/utils'
import { CompactDateTimeRangePicker } from '../compact-date-time-range-picker'

describe('usage log date range presets', () => {
  beforeAll(async () => {
    i18next.addResourceBundle('en', 'translation', {
      'Date Range': 'Date Range',
      Yesterday: 'Yesterday',
      'This month': 'This month',
      'Last month': 'Last month',
    })
    await i18next.changeLanguage('en')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('selecting yesterday emits the previous local natural day', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date(2026, 8, 19, 15, 30, 45, 123))
    const onChange = vi.fn()

    render(<CompactDateTimeRangePicker onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: /Date Range/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Yesterday' }))

    expect(onChange).toHaveBeenCalledWith({
      start: new Date(2026, 8, 18, 0, 0, 0, 0),
      end: new Date(2026, 8, 18, 23, 59, 59, 999),
    })
  })

  test.each([
    {
      name: 'October 2026 selects the complete September calendar month',
      now: new Date(2026, 9, 9, 15, 30, 45, 123),
      start: new Date(2026, 8, 1, 0, 0, 0, 0),
      end: new Date(2026, 8, 30, 23, 59, 59, 999),
    },
    {
      name: 'January selects December of the previous year',
      now: new Date(2026, 0, 15, 15, 30, 45, 123),
      start: new Date(2025, 11, 1, 0, 0, 0, 0),
      end: new Date(2025, 11, 31, 23, 59, 59, 999),
    },
    {
      name: 'March 31 in a leap year includes February 29',
      now: new Date(2024, 2, 31, 15, 30, 45, 123),
      start: new Date(2024, 1, 1, 0, 0, 0, 0),
      end: new Date(2024, 1, 29, 23, 59, 59, 999),
    },
  ])(
    'selecting last month in $name closes the picker and emits the local range',
    (fixture) => {
      vi.useFakeTimers()
      vi.setSystemTime(fixture.now)
      const onChange = vi.fn<(range: { start?: Date; end?: Date }) => void>()

      render(<CompactDateTimeRangePicker onChange={onChange} />)

      const trigger = screen.getByRole('button', { name: /Date Range/ })
      fireEvent.click(trigger)
      const thisMonth = screen.getByRole('button', { name: 'This month' })
      const lastMonth = screen.getByRole('button', { name: 'Last month' })
      const buttons = screen.getAllByRole('button')
      expect(buttons.indexOf(lastMonth)).toBe(buttons.indexOf(thisMonth) + 1)

      fireEvent.click(lastMonth)

      expect(onChange).toHaveBeenCalledExactlyOnceWith({
        start: fixture.start,
        end: fixture.end,
      })
      expect(trigger).toHaveAttribute('aria-expanded', 'false')

      const selectedRange = onChange.mock.calls[0][0]
      const searchParams = {
        startTime: selectedRange.start?.getTime(),
        endTime: selectedRange.end?.getTime(),
      }
      const expectedTimestamps = {
        start_timestamp: fixture.start.getTime() / 1000,
        end_timestamp: (fixture.end.getTime() - 999) / 1000,
      }
      expect(
        buildApiParams({ page: 1, pageSize: 20, searchParams, isAdmin: false })
      ).toMatchObject(expectedTimestamps)
      expect(
        buildBaseParams({ page: 1, pageSize: 20, searchParams })
      ).toMatchObject(expectedTimestamps)
    }
  )
})
