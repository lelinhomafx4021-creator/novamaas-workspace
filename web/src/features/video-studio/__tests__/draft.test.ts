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
  parametersForModel,
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
  resolutions: ['480P', '720P'],
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
  resolutions: ['768P', '1080P'],
  audio: false,
  seed: false,
  defaults: { duration: 5, resolution: '768P', ratio: '16:9' },
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

  test('blocks duplicate frames and unsupported model parameters before submit', () => {
    const issues = draftIssues(
      draftWith({
        references: [
          { source: 'storage', id: 'ref_1', role: 'first_frame', label: 'a' },
          { source: 'storage', id: 'ref_2', role: 'first_frame', label: 'b' },
        ],
        parameters: {
          duration: 2.5,
          resolution: '1080P',
          ratio: '4:3',
          seed: 2 ** 31,
        },
      }),
      wan3
    )
    expect(issues).toEqual([
      'Only one first frame and one last frame are allowed',
      'Seed is out of range for this model',
      'Duration is out of range for this model',
      'Resolution is not available for this model',
      'Aspect ratio is not available for this model',
    ])
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
    expect(request.token_id).toBeUndefined()
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

describe('parametersForModel', () => {
  test('keeps values the new model supports and resets the rest to its defaults', () => {
    const next = parametersForModel(
      {
        duration: 12,
        resolution: '720P',
        ratio: '16:9',
        generate_audio: true,
        seed: 7,
      },
      minimax
    )
    // 720P does not exist for H3; audio and seed are not offered.
    expect(next).toEqual({ duration: 12, resolution: '768P', ratio: '16:9' })
  })

  test('keeps an explicit audio=false and seed=0 when the model supports them', () => {
    const next = parametersForModel(
      {
        duration: 5,
        resolution: '480P',
        ratio: '16:9',
        generate_audio: false,
        seed: 0,
      },
      wan3
    )
    expect(next).toMatchObject({
      resolution: '480P',
      generate_audio: false,
      seed: 0,
    })
  })
})

describe('parseDraft', () => {
  test('fills fields that older drafts did not save', () => {
    const draft = parseDraft(
      JSON.stringify({
        version: 1,
        nodes: { n1: { model: 'wan3.0-video', group: 'vip', prompt: 'x' } },
      })
    )
    expect(draft.nodes.n1).toMatchObject({
      model: 'wan3.0-video',
      prompt: 'x',
      token_id: 0,
      text_token_id: 0,
      text_model: '',
      references: [],
    })
  })
})
