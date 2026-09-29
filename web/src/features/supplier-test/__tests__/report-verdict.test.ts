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

import { assessStress, getStandard } from '../baselines'
import {
  buildHtmlReport,
  buildMarkdownReport,
  type ReportInput,
} from '../report'
import type { StressMetrics } from '../types'

function reportInput(overrides: Partial<ReportInput> = {}): ReportInput {
  return {
    baseUrl: 'https://supplier.example/v1',
    model: 'example-model',
    basicChecks: [],
    cacheChecks: [],
    summaries: { basic: '', cache: '', stress: '' },
    stressAssessment: null,
    cacheAssessment: null,
    errorMessage: '',
    standardLabel: 'Default standard',
    statusLabel: (status) => {
      if (status === 'pass') return 'Passed'
      if (status === 'fail') return 'Failed'
      if (status === 'skip') return 'Skipped'
      if (status === 'running') return 'Running'
      return 'Idle'
    },
    t: (key, options) => {
      let value = key
      for (const [name, replacement] of Object.entries(options ?? {})) {
        value = value.replaceAll(`{{${name}}}`, String(replacement))
      }
      return value
    },
    ...overrides,
  }
}

describe('supplier report overall verdict', () => {
  test('keeps a passing result when assessed checks pass and other checks are skipped', () => {
    const html = buildHtmlReport(
      reportInput({
        basicChecks: [
          { id: 'connectivity', title: 'Connectivity', status: 'pass' },
          { id: 'tool_call', title: 'Tool call', status: 'skip' },
        ],
        stressAssessment: { rows: [], overall: 'ok' },
        cacheAssessment: { rows: [], overall: 'na' },
      }),
      { format: 'pdf' }
    )

    expect(html).toContain(
      '<div class="document-summary-value result-pass">Passed</div>'
    )
    expect(html).not.toContain('Cannot compare')
  })

  test('fails the report when it has no assessed result', () => {
    const html = buildHtmlReport(
      reportInput({
        basicChecks: [
          { id: 'connectivity', title: 'Connectivity', status: 'skip' },
        ],
        cacheAssessment: { rows: [], overall: 'na' },
      }),
      { format: 'pdf' }
    )

    expect(html).toContain(
      '<div class="document-summary-value result-fail">Failed</div>'
    )
    expect(html).not.toContain('Cannot compare')
  })

  test('uses the selected standard for both report thresholds and verdicts', () => {
    const metrics: StressMetrics = {
      total: 10,
      succeeded: 10,
      failed: 0,
      error_rate: 0,
      elapsed_ms: 1000,
      tokens_per_sec: 100,
      prompt_tokens: 1000,
      completion_tokens: 100,
      ttft_avg_ms: 4500,
      ttft_p50_ms: 4200,
      ttft_p90_ms: 7000,
      ttft_n: 10,
      tpot_avg_ms: 72,
      tpot_p50_ms: 68,
      tpot_p90_ms: 140,
      tpot_n: 10,
      rpm: 600,
      tpm: 60000,
    }
    const defaultHtml = buildHtmlReport(
      reportInput({
        stressAssessment: assessStress(metrics, getStandard('default')),
        stressMetrics: metrics,
      }),
      { format: 'pdf' }
    )
    const tightHtml = buildHtmlReport(
      reportInput({
        standardLabel: 'Tight standard',
        stressAssessment: assessStress(metrics, getStandard('tight')),
        stressMetrics: metrics,
      }),
      { format: 'pdf' }
    )

    expect(defaultHtml).toContain(
      '<div class="document-summary-value result-pass">Passed</div>'
    )
    expect(defaultHtml).toContain('Normal ≤ 8s; slower above that')
    expect(tightHtml).toContain(
      '<div class="document-summary-value result-fail">Failed</div>'
    )
    expect(tightHtml).toContain('Normal ≤ 3s; slower above that')
  })

  test('emphasizes pass and fail statuses and labels a skipped check reason', () => {
    const input = reportInput({
      basicChecks: [
        {
          id: 'connectivity',
          title: 'Connectivity',
          status: 'pass',
          message: 'Connected',
        },
        {
          id: 'usage',
          title: 'Usage fields',
          status: 'fail',
          message: 'Missing usage',
        },
        {
          id: 'request_id',
          title: 'Request id',
          status: 'skip',
          message: 'No request id returned',
        },
      ],
    })

    const html = buildHtmlReport(input, { format: 'pdf' })

    expect(html).toContain('class="check-status check-status-pass">Passed')
    expect(html).toContain('class="check-status check-status-fail">Failed')
    expect(html).toContain('class="check-status check-status-skip">Skipped')
    expect(html).toContain(
      '<span class="check-reason"><strong>Reason:</strong> No request id returned</span>'
    )

    const markdown = buildMarkdownReport(input)
    expect(markdown).toContain('Connectivity: **Passed** — Connected')
    expect(markdown).toContain(
      'Request id: **Skipped** — **Reason:** No request id returned'
    )
  })

  test('shows the supplier access URL and tested model in the PDF report', () => {
    const html = buildHtmlReport(
      reportInput({
        baseUrl: 'https://supplier.example/v1',
        model: 'supplier-model-v1',
      }),
      { format: 'pdf' }
    )

    expect(html).toContain('https://supplier.example/v1')
    expect(html).toContain('supplier-model-v1')
  })
})
