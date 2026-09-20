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
import { describe, expect, test } from 'vitest'

import { costAccountingBackfillSchema } from '../cost-accounting-schema'

describe('cost accounting operation schema', () => {
  test('accepts an empty discount for zero-profit accounting', () => {
    const result = costAccountingBackfillSchema.safeParse({
      start_time: '2026-09-19T00:00',
      end_time: '2026-09-19T23:59',
      channel_id: 8,
      cost_discount: '',
      limit: 1000,
    })

    expect(result.success).toBe(true)
  })
})
