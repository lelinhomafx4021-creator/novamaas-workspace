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

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

function validForm(costDiscount: string) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'cost-accounted-channel',
    key: 'test-key',
    models: 'gpt-5',
    group: ['default'],
    cost_discount: costDiscount,
  }
}

describe('channel upstream cost discount', () => {
  test.each(['0', '0.8', '0.876543', '1', '1.000000'])(
    'accepts exact supported decimal %s',
    (value) => {
      expect(channelFormSchema.safeParse(validForm(value)).success).toBe(true)
    }
  )

  test.each(['-0.1', '0.1234567', '1.000001', 'NaN'])(
    'rejects invalid discount %s',
    (value) => {
      expect(channelFormSchema.safeParse(validForm(value)).success).toBe(false)
    }
  )

  test('defaults to no discount configuration', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.cost_discount).toBe('')
  })

  test('serializes an entered discount and allows an authorized user to clear it', () => {
    const create = transformFormDataToCreatePayload(validForm('0.765432'))
    expect(create.channel.cost_discount).toBe('0.765432')

    const update = transformFormDataToUpdatePayload(validForm(''), 7, true)
    expect(update.cost_discount).toBe('')

    const redactedUpdate = transformFormDataToUpdatePayload(
      validForm(''),
      7,
      false
    )
    expect(redactedUpdate).not.toHaveProperty('cost_discount')
  })
})
