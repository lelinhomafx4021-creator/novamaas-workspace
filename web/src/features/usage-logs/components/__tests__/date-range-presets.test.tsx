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

import { CompactDateTimeRangePicker } from '../compact-date-time-range-picker'

describe('usage log date range presets', () => {
  beforeAll(async () => {
    i18next.addResourceBundle('en', 'translation', {
      'Date Range': 'Date Range',
      Yesterday: 'Yesterday',
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
})
