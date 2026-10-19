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

test('form sends enabled parameters and omits disabled defaults', () => {
  expect(
    buildVideoRequestPayload('model', {
      ...DEFAULT_VIDEO_FORM,
      prompt: 'animate',
      hasResolution: true,
      resolution: '1080p',
      hasRatio: true,
      ratio: '16:9',
      hasDuration: true,
      duration: 5,
      hasWatermark: true,
      watermark: false,
      hasSeed: true,
      seed: '998877',
      hasGenerateAudio: true,
      generateAudio: true,
    })
  ).toEqual({
    model: 'model',
    content: [{ type: 'text', text: 'animate' }],
    resolution: '1080p',
    ratio: '16:9',
    duration: 5,
    watermark: false,
    seed: 998877,
    generate_audio: true,
  })
})

test('first and last frame images retain their roles and order', () => {
  const materials = [
    {
      type: 'image_url',
      image_url: { url: 'https://example.com/first.png' },
      role: 'first_frame',
    },
    {
      type: 'image_url',
      image_url: { url: 'https://example.com/last.png' },
      role: 'last_frame',
    },
  ]
  expect(
    buildVideoRequestPayload('model', {
      ...DEFAULT_VIDEO_FORM,
      prompt: 'animate',
      materials,
    })?.content
  ).toEqual([{ type: 'text', text: 'animate' }, ...materials])
})

test('empty form does not invent a request and invalid JSON is rejected', () => {
  expect(buildVideoRequestPayload('', DEFAULT_VIDEO_FORM)).toBeNull()
  expect(buildVideoRequestPayload('model', DEFAULT_VIDEO_FORM)).toBeNull()
  expect(parseVideoPayloadToForm('not JSON').success).toBe(false)
})
