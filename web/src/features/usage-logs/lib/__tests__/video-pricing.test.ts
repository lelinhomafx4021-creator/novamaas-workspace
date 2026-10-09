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

import { getVideoPricingMultiplier } from '../format'

describe('saved task video pricing multipliers', () => {
  test.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY])(
    'invalid saved multiplier %s falls back to the unit tier',
    (video_input) => {
      expect(
        getVideoPricingMultiplier({
          is_task: true,
          other_ratios: { video_input },
        })
      ).toBe(1)
    }
  )

  test('a structured map without video pricing ignores the old flattened video multiplier', () => {
    expect(
      getVideoPricingMultiplier({
        is_task: true,
        other_ratios: { seconds: 5 },
        video_input: 0.6,
      })
    ).toBe(1)
  })

  test('unrelated token logs do not apply a video pricing multiplier', () => {
    expect(
      getVideoPricingMultiplier({
        model_ratio: 5,
        other_ratios: { video_input: 1.1 },
      })
    ).toBe(1)
  })

  test('a task uses its precise structured multiplier even when no resolution label was saved', () => {
    expect(
      getVideoPricingMultiplier({
        is_task: true,
        other_ratios: { video_input: 46 / 70 },
      })
    ).toBe(46 / 70)
  })
})
