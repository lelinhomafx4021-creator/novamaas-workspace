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

import { assessCache, assessStress, getStandard } from './baselines'
import {
  buildHtmlReport,
  buildMarkdownReport,
  formatTokenCompact,
  isInformationalRow,
  type ReportInput,
} from './report'
import type { CacheMetrics, CheckResult, StressMetrics } from './types'

function makeTestInput(): ReportInput {
  const basicChecks: CheckResult[] = [
    { id: 'ping', title: 'Connectivity', status: 'pass' },
    { id: 'stream', title: 'Streaming', status: 'pass' },
    { id: 'json_mode', title: 'JSON mode', status: 'pass' },
  ]
  const stressMetrics: StressMetrics = {
    total: 100,
    succeeded: 100,
    failed: 0,
    error_rate: 0,
    elapsed_ms: 40383,
    tokens_per_sec: 935.0,
    prompt_tokens: 137500,
    completion_tokens: 37760,
    ttft_avg_ms: 3208,
    ttft_p50_ms: 2431,
    ttft_p90_ms: 3236,
    ttft_n: 100,
    tpot_avg_ms: 15.6,
    tpot_p50_ms: 14.4,
    tpot_p90_ms: 22.7,
    tpot_n: 100,
    rpm: 148.6,
    tpm: 260394,
  }
  const cacheMetrics: CacheMetrics = {
    warm_prompt_tokens: 3000,
    avg_hit_rate: 0.989,
    min_hit_rate: 0.95,
    last_cached_tokens: 2967,
    last_prompt_tokens: 3000,
    wait_seconds: 30,
    rounds: 5,
    has_cached_tokens: true,
    hit_count: 5,
    avg_depth_rate: 0.989,
    mode: 'static',
  }
  const std = getStandard('default')

  return {
    baseUrl: 'https://maas.ai.shilijia.xyz/v1',
    model: 'glm-5.2',
    standardLabel: 'Default standard',
    basicChecks,
    cacheChecks: [{ id: 'cache_probe', title: 'Cache probe', status: 'pass' }],
    summaries: {
      basic: 'Passed 9, skipped 1, failed 0',
      cache: 'Cache test completed successfully',
      stress: 'Stress test completed: 100 requests',
    },
    stressAssessment: assessStress(stressMetrics, std),
    cacheAssessment: assessCache(cacheMetrics, std),
    errorMessage: '',
    statusLabel: (status) => status,
    stressConfig: {
      concurrency: 50,
      rounds: 2,
      stream: true,
      breakCache: false,
      maxTokens: 1024,
    },
    stressMetrics,
    cacheMetrics,
    t: (key) => key,
  }
}

describe('supplier-test report', () => {
  test('formatTokenCompact formats large and small numbers', () => {
    expect(formatTokenCompact(0)).toBe('0')
    expect(formatTokenCompact(800)).toBe('800')
    expect(formatTokenCompact(1500)).toBe('1.5k')
    expect(formatTokenCompact(37760)).toBe('37.8k')
    expect(formatTokenCompact(137500)).toBe('137.5k')
    expect(formatTokenCompact(2500000)).toBe('2.5M')
  })

  test('buildHtmlReport suppresses browser header/footers with zero page margin', () => {
    const input = makeTestInput()
    const html = buildHtmlReport(input)
    expect(html).toContain('@page {')
    expect(html).toContain('margin: 0;')
    expect(html).toContain('print-color-adjust: exact')
  })

  test('buildHtmlReport includes concurrency config and execution telemetry', () => {
    const input = makeTestInput()
    const html = buildHtmlReport(input)

    // Concurrency configuration
    expect(html).toContain('50 × 2 (100)')
    expect(html).toContain('max_tokens = 1024')
    expect(html).toContain('Stream')

    // Execution metrics
    expect(html).toContain('40.38 s')
    expect(html).toContain('935.0 tok/s')
    expect(html).toContain('137.5k / 37.8k')

    // Sections and overall verdicts
    expect(html).toContain('Concurrency and stress test')
    expect(html).toContain('Prompt cache test')
    expect(html).toContain('Stress test assessment')
    expect(html).toContain('Prompt cache assessment')

    // Benchmark tables must NOT contain informational rows like duration or tokens
    const benchmarkRows = input.stressAssessment?.rows.filter(
      (r) => !isInformationalRow(r)
    )
    expect(benchmarkRows).toBeDefined()
    expect(benchmarkRows?.length).toBeGreaterThan(0)
    for (const r of benchmarkRows ?? []) {
      expect(html).toContain(r.label)
    }
  })

  test('buildMarkdownReport formats structured sections without Cannot compare in tables', () => {
    const input = makeTestInput()
    const md = buildMarkdownReport(input)

    expect(md).toContain('# Supplier Test Report')
    expect(md).toContain('## 1. Connectivity and protocol')
    expect(md).toContain('## 2. Concurrency and stress test')
    expect(md).toContain('## 3. Prompt cache test')
    expect(md).toContain('50 × 2 (100)')
    expect(md).toContain('40.38 s')
    expect(md).toContain('935.0 tok/s')

    // Informational rows (throughput, duration, tokens) are in summary bullets, not benchmark table
    expect(md).not.toContain('| Throughput |')
    expect(md).not.toContain('| Total duration |')
  })
})
