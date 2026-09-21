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

import type { Channel } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  MOONSHOT_FACADE_MODE_EMULATE,
  MOONSHOT_FACADE_MODE_KIMI_PASSTHROUGH,
  buildSettingJSON,
  transformChannelToFormDefaults,
} from '../channel-form'

function openAIChannel(setting: string): Channel {
  return {
    id: 1,
    type: 1,
    key: '',
    status: 1,
    name: 'kimi-openai',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance: 0,
    balance_updated_time: 0,
    models: 'kimi-k3',
    group: 'default',
    used_quota: 0,
    other: '',
    other_info: '',
    remark: '',
    max_input_tokens: 0,
    channel_info: {
      is_multi_key: false,
      multi_key_size: 0,
      multi_key_polling_index: 0,
      multi_key_mode: 'random',
    },
    setting,
    settings: '{}',
  }
}

describe('Moonshot facade mode', () => {
  test('defaults legacy OpenAI channels to compatibility emulation', () => {
    const defaults = transformChannelToFormDefaults(openAIChannel('{}'))

    expect(defaults.moonshot_facade_mode).toBe(MOONSHOT_FACADE_MODE_EMULATE)
  })

  test('loads and persists Kimi-compatible passthrough', () => {
    const defaults = transformChannelToFormDefaults(
      openAIChannel('{"moonshot_facade_mode":"kimi_passthrough"}')
    )
    const setting = JSON.parse(buildSettingJSON(defaults))

    expect(defaults.moonshot_facade_mode).toBe(
      MOONSHOT_FACADE_MODE_KIMI_PASSTHROUGH
    )
    expect(setting.moonshot_facade_mode).toBe(
      MOONSHOT_FACADE_MODE_KIMI_PASSTHROUGH
    )
  })

  test('omits the emulation default from saved settings', () => {
    const setting = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        moonshot_facade_mode: MOONSHOT_FACADE_MODE_EMULATE,
      })
    )

    expect(setting).not.toHaveProperty('moonshot_facade_mode')
  })
})
