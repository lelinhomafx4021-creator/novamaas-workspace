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
import { expect, test } from 'vitest'

import { DEFAULT_VIDEO_FORM } from '../constants'
import {
  buildVideoRequestPayload,
  parseVideoPayloadToForm,
} from '../video-json'

test('native JSON preserves multiple input forms, order, unknown content and explicit false/zero when edited', () => {
  const payload = {
    model: 'model',
    content: [
      {
        type: 'image_url',
        role: 'reference_image',
        image_url: { url: 'https://cdn.example/a.png' },
        unknown: false,
      },
      { type: 'text', text: 'animate', unknown: 0 },
      {
        type: 'image_url',
        role: 'reference_image',
        image_url: { url: 'data:image/png;base64,b25l' },
      },
      {
        type: 'image_url',
        role: 'reference_image',
        image_url: { url: 'asset://asset-one' },
      },
      { type: 'provider_specific', enabled: false, count: 0 },
    ],
    duration: 0,
    watermark: false,
    generate_audio: false,
    seed: 0,
    unknown: { flag: false, count: 0 },
  }
  const parsed = parseVideoPayloadToForm(JSON.stringify(payload))
  expect(parsed.success).toBe(true)
  expect(
    buildVideoRequestPayload(parsed.model || '', {
      ...DEFAULT_VIDEO_FORM,
      ...parsed.videoPatch,
    })
  ).toEqual(payload)
})
