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
  buildSubmitRequest,
  draftFromRun,
  draftIssues,
  newGenerationDraft,
  parseDraft,
} from '../lib/draft'
import type { Capability, GenerationDraft, Run } from '../types'

const wan3: Capability = {
  model: 'wan3.0-video',
  protocol: 'wan3',
  roles: ['first_frame', 'last_frame', 'reference_image'],
  max_references: 9,
  min_duration: 2,
  max_duration: 30,
  resolutions: ['480P', '720P', '1080P'],
  ratios: ['16:9'],
  audio: true,
  seed: true,
  asset_library: false,
  defaults: { duration: 5, resolution: '720P', ratio: '16:9' },
}

const minimax: Capability = {
  ...wan3,
  model: 'MiniMax-H3',
  protocol: 'openai_video',
  roles: [],
  max_references: 0,
  audio: false,
  seed: false,
}

function draftWith(patch: Partial<GenerationDraft>): GenerationDraft {
  return { ...newGenerationDraft(wan3), prompt: 'city at night', ...patch }
}

describe('draftIssues', () => {
  test('a complete draft has no issues', () => {
    expect(draftIssues(draftWith({}), wan3)).toEqual([])
  })

  test('switching to a text-only model keeps references but blocks submit', () => {
    const draft = draftWith({
      references: [
        { source: 'storage', id: 'ref_1', role: 'first_frame', label: 'a.png' },
      ],
      parameters: { duration: 5, generate_audio: false },
    })
    const issues = draftIssues(draft, minimax)
    expect(issues).toContain('Too many reference images for this model')
    expect(issues).toContain('This model does not support audio generation')
    expect(draft.references).toHaveLength(1)
  })

  test('frame rules match the server', () => {
    expect(
      draftIssues(
        draftWith({
          references: [
            { source: 'storage', id: 'ref_1', role: 'last_frame', label: 'a' },
          ],
        }),
        wan3
      )
    ).toContain('A last frame requires a first frame')
    expect(
      draftIssues(
        draftWith({
          references: [
            {
              source: 'seedance_asset',
              id: 'asset-1',
              role: 'first_frame',
              label: 'a',
            },
          ],
        }),
        wan3
      )
    ).toContain('Seedance assets only work with Seedance models')
  })
})

describe('submission and history', () => {
  test('submission keeps explicit false and zero and strips display labels', () => {
    const request = buildSubmitRequest(
      'vp_1',
      'gen_1',
      draftWith({
        references: [
          {
            source: 'storage',
            id: 'ref_1',
            role: 'first_frame',
            label: 'a.png',
          },
        ],
        parameters: { duration: 5, generate_audio: false, seed: 0 },
      }),
      'req-1'
    )
    expect(request.parameters).toEqual({
      duration: 5,
      generate_audio: false,
      seed: 0,
    })
    expect(request.references).toEqual([
      { source: 'storage', id: 'ref_1', role: 'first_frame' },
    ])
    expect(request.group).toBeUndefined()
  })

  test('generate again restores the submitted snapshot, not the current draft', () => {
    const current = draftWith({ prompt: 'edited later' })
    const run = {
      snapshot: JSON.stringify({
        model: 'wan3.0-video',
        prompt: 'original',
        references: [],
        parameters: { duration: 8 },
      }),
    } as Run
    const restored = draftFromRun(run, current)
    expect(restored.prompt).toBe('original')
    expect(restored.parameters).toEqual({ duration: 8 })
  })

  test('a corrupt stored draft opens as an empty project', () => {
    expect(parseDraft('not json')).toEqual({ version: 1, nodes: {} })
  })
})
