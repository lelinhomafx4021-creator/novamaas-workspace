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
  type VideoReportInput,
} from '../report'
import { compactEvidenceJSON } from '../video-report'

const input = {
  t: (key: string) => key,
  runReport: {
    id: 'run-1',
    model: 'actual-model',
    base_url: 'https://gateway.example',
    task_id: 'task-1',
    query_id: 'query-1',
    status: 'completed',
    video_polling: { count: 8, elapsed_ms: 240 },
    verification_polling: { count: 2, elapsed_ms: 30 },
    started_at: 1,
    completed_at: 2,
    stages: [
      {
        id: 'video_submit',
        status: 'pass',
        purpose: 'Submit the video task',
        conclusion: 'Task accepted',
        method: 'POST',
        path: '/api/v3/contents/generations/tasks',
        request_body: '{"model":"actual-model","watermark":false,"seed":0}',
        response_body: '{"id":"task-1","extra":"accepted-proof"}',
        http_status: 200,
        elapsed_ms: 5,
      },
      {
        id: 'video_result',
        final: true,
        status: 'pass',
        purpose: 'Retrieve the final video result',
        conclusion: 'Video generated',
        method: 'GET',
        path: '/api/v3/contents/generations/tasks/task-1',
        request_body: '',
        response_body:
          '{"status":"succeeded","content":{"video_url":"https://cdn.example/result.mp4"},"extra":"final-proof"}',
        http_status: 200,
        elapsed_ms: 5,
      },
      {
        id: 'verification_submit',
        status: 'pass',
        purpose: 'Submit the original video for official verification',
        conclusion: 'Verification accepted',
        method: 'POST',
        path: '/?Action=CreateArkOfficialResultQuery&Version=2024-01-01',
        request_body:
          '{"Type":"ContentURL","ContentURL":"https://cdn.example/result.mp4"}',
        response_body: '{"Result":{"QueryID":"query-1"}}',
        http_status: 200,
        elapsed_ms: 5,
      },
      {
        id: 'verification_result',
        final: true,
        status: 'pass',
        purpose: 'Retrieve the final official verification result',
        conclusion: 'Official features detected',
        method: 'POST',
        path: '/?Action=GetArkOfficialResult&Version=2024-01-01',
        request_body: '{"QueryID":"query-1"}',
        response_body:
          '{"Result":{"Status":"Success","IsOfficial":"true","ModelName":"actual-model","extra":"verification-proof"}}',
        http_status: 200,
        elapsed_ms: 5,
      },
    ],
    assets: [],
    verification: {
      status: 'matched',
      is_official: true,
      model_name: 'actual-model',
      model_match: 'matched',
    },
  },
} satisfies VideoReportInput

test('all formats export one report with generation and verification together and one polling summary', () => {
  for (const build of [
    buildVideoMarkdownReport,
    buildVideoHtmlReport,
    (value: VideoReportInput) => buildVideoHtmlReport(value, { format: 'pdf' }),
  ]) {
    const report = build(input)
    for (const proof of [
      'Doubao Video Generation Test Report',
      'actual-model',
      'accepted-proof',
      'final-proof',
      'verification-proof',
      'CreateArkOfficialResultQuery',
      'GetArkOfficialResult',
      'task-1',
      'query-1',
      'No request body',
      'First queued response',
      'First running response',
    ]) {
      expect(report).toContain(proof)
    }
    expect(report.match(/Polling summary/g)).toHaveLength(1)
  }
})

test('an unobserved queue or running state is marked missing rather than invented', () => {
  const markdown = buildVideoMarkdownReport(input)
  expect(markdown).toContain('### First queued response\n\nNot recorded')
  expect(markdown).toContain('### First running response\n\nNot recorded')
})

test('long JSON and untrusted response text remain visible and escaped in PDF', () => {
  const report = structuredClone({
    ...input,
    t: undefined,
  }) as unknown as VideoReportInput
  report.t = (key) => key
  const run = report.runReport
  run.stages[0].response_body = `{"text":"<script>bad()</script>${'x'.repeat(10000)}tail-proof"}`
  const html = buildVideoHtmlReport(report, { format: 'pdf' })
  expect(html).toContain('tail-proof')
  expect(html).toContain('&lt;script&gt;')
  expect(html).not.toContain('<script>bad()')
})

test('compact evidence preserves large numbers, explicit values and whitespace inside JSON strings', () => {
  const raw =
    '{\n  "n":9007199254740993,\n  "text":"keep  two spaces",\n  "enabled": false, "zero": 0\n}'
  expect(compactEvidenceJSON(raw)).toBe(
    '{"n":9007199254740993,"text":"keep  two spaces","enabled":false,"zero":0}'
  )
  expect(compactEvidenceJSON('Upstream error:  service unavailable')).toBe(
    'Upstream error:  service unavailable'
  )
})
