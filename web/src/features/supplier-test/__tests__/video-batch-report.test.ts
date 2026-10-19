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

import {
  buildVideoHtmlReport,
  buildVideoMarkdownReport,
  videoEvidenceText,
  videoResult,
} from '../report'
import type { VideoTestRecord } from '../types'

const records: VideoTestRecord[] = [
  'doubao-seedance-2-0-260128',
  'doubao-seedance-1-5-pro-251215',
].flatMap((model) =>
  ['720p', '1080p'].map((resolution) => {
    const id = `${model}-${resolution}`
    const exchange = {
      method: 'POST',
      url: 'https://supplier.example/api/v3/contents/generations/tasks',
      headers: 'Authorization: [REDACTED]',
      request: `{"model":"${model}","resolution":"${resolution}","seed":0,"watermark":false,"extra":9007199254740993}`,
      response: `{"id":"${id}"}`,
      status: 200,
      request_id: id,
      elapsed_ms: 10,
    }
    return {
      id,
      baseUrl: 'https://supplier.example',
      model,
      startedAt: 1,
      outcome: 'Succeeded',
      materials: [],
      metrics: {
        task_id: id,
        status: 'succeeded',
        elapsed_ms: 100,
        submit: exchange,
        raw_request_json: exchange.request,
        final: {
          ...exchange,
          method: 'GET',
          request: '',
          response: `{"status":"succeeded","video_url":"https://cdn.example/${id}.mp4"}`,
        },
        video_url: `https://cdn.example/${id}.mp4`,
      },
      verification: {
        attempted: true,
        queryId: `query-${id}`,
        count: 1,
        elapsed: 10,
        create: {
          ...exchange,
          request: `{"Type":"content_url","ContentURL":"https://cdn.example/${id}.mp4"}`,
          response: `{"Result":{"QueryID":"query-${id}"}}`,
        },
        final: {
          ...exchange,
          request: `{"QueryID":"query-${id}"}`,
          response: `{"Result":{"Status":"Success","IsOfficial":true,"ModelName":"${model}"}}`,
        },
      },
    }
  })
)

test.each([
  { v3: false, materialLibrary: false },
  { v3: true, materialLibrary: false },
  { v3: false, materialLibrary: true },
  { v3: true, materialLibrary: true },
])(
  'all exports preserve four model/resolution snapshots and manual values: %j',
  (checks) => {
    const input = { records, ...checks, t: (key: string) => key }
    for (const report of [
      buildVideoMarkdownReport(input),
      buildVideoHtmlReport(input),
      buildVideoHtmlReport(input, { format: 'pdf' }),
    ]) {
      expect(report).toContain(
        `Supplier has implemented the v3 API: ${checks.v3 ? 'Yes' : 'No'}`
      )
      expect(report).toContain(
        `Supplier has implemented a material library: ${checks.materialLibrary ? 'Yes' : 'No'}`
      )
      for (const record of records) {
        expect(report).toContain(record.metrics.task_id)
        expect(report).toContain(`query-${record.id}`)
      }
      expect(report).toContain('9007199254740993')
      expect(report).toContain('watermark')
      expect(report).toContain('Request headers')
      expect(report).toContain('Official verification submission')
      expect(report).not.toContain('max-height:')
    }
  }
)

test('partial response cannot become terminal proof, and aliases do not imply model matches', () => {
  const terminal = records[0].metrics.final
  if (!terminal) throw new Error('Missing terminal fixture')
  const record = {
    ...records[0],
    model: 'public-alias',
    outcome: 'Incomplete',
    metrics: {
      ...records[0].metrics,
      raw_request_json: '{"model":"public-alias"}',
      final: undefined,
      last_query: {
        ...terminal,
        response: '{"status":"running"}',
      },
    },
  }
  expect(videoResult(record).comparison).toBe('Cannot confirm')
  const markdown = buildVideoMarkdownReport({
    records: [record],
    t: (key) => key,
  })
  expect(markdown).toContain('Latest query (not final)')
  expect(markdown).toContain('No final response obtained')
})

test('redaction preserves large numeric tokens and binary attachments while masking nested credentials and link signatures', () => {
  const nested = JSON.stringify({
    api_key: 'private',
    url: 'https://cdn.example/a?signature=secret',
    image: 'data:image/png;base64,aGVsbG8=',
  })
  const raw = `{"number":9007199254740993,"watermark":false,"body":${JSON.stringify(nested)}}`
  const sanitized = videoEvidenceText(raw)
  expect(sanitized).not.toContain('private')
  expect(sanitized).not.toContain('signature=secret')
  expect(sanitized).toContain('9007199254740993')
  expect(sanitized).toContain('aGVsbG8=')
  expect(sanitized).toContain('false')
})

test.each([
  [
    false,
    'doubao-seedance-2-0-260128',
    'No features detected',
    'Cannot confirm',
  ],
  [null, 'doubao-seedance-2-0-260128', 'Cannot confirm', 'Cannot confirm'],
  [true, 'doubao-seedance-1-5-pro-251215', 'Matched', 'Mismatch'],
  [true, 'Seedance2.0', 'Matched', 'Cannot confirm'],
])(
  'official detection %s and model %s remain separate conclusions',
  (detected, model, origin, comparison) => {
    const record = records[0]
    const verification = record.verification
    if (!verification?.final) throw new Error('Missing final fixture')
    const result = videoResult({
      ...record,
      verification: {
        ...verification,
        final: {
          ...verification.final,
          response: JSON.stringify({
            Result: {
              Status: 'Success',
              IsOfficial: detected,
              ModelName: model,
            },
          }),
        },
      },
    })
    expect(result.generation).toBe('Succeeded')
    expect(result.origin).toBe(origin)
    expect(result.comparison).toBe(comparison)
  }
)
