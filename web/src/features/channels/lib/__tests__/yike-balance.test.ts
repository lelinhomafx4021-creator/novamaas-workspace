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
import { describe, expect, it } from 'vitest'

import { CHANNEL_TYPE_YIKE } from '../../constants'
import { formatYikeCredits, isYikeChannel } from '../yike-balance'

describe('Yike credit balance', () => {
  it('formats native credits without a currency symbol', () => {
    expect(formatYikeCredits(104, '积分', 'zh-CN')).toBe('104 积分')
  })

  it('falls back for missing or invalid balances', () => {
    expect(formatYikeCredits(null, 'Credits', 'en-US')).toBe('-')
    expect(formatYikeCredits(undefined, 'Credits', 'en-US')).toBe('-')
    expect(formatYikeCredits(Number.NaN, 'Credits', 'en-US')).toBe('-')
  })

  it('recognizes only the Yike channel type', () => {
    expect(isYikeChannel(CHANNEL_TYPE_YIKE)).toBe(true)
    expect(isYikeChannel(1)).toBe(false)
  })
})
