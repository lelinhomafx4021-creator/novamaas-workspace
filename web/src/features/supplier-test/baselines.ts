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

import type { CacheMetrics, StressMetrics } from './types'

export type Verdict = 'ok' | 'slow' | 'na'

export const VERDICT_LABEL: Record<Verdict, string> = {
  ok: 'Normal',
  slow: 'Slow',
  na: 'Cannot compare',
}

export type MetricRow = {
  id: string
  label: string
  measured: string
  measuredValues?: Record<string, string | number>
  threshold: string
  verdict: Verdict
}

export type Assessment = {
  rows: MetricRow[]
  overall: Verdict
}

const NO_SAMPLE = 'No sample (enable stream)'
const TTFT_SHORT = 'Normal ≤ 8s; slower above that'
const TTFT_LONG = 'Normal ≤ 15s; slower above that'
const TTFT_P90 = 'Normal ≤ avg × 3; slower above that'
const TPOT_AVG = 'Normal ≤ 80ms; slower above that'
const TPOT_P90 = 'Normal ≤ 150ms; slower above that'
const HIT_RATE = 'Normal ≥ 50%; slower below that'
const TTL_RULE = 'Wait ≥ 30s and hit rate still ≥ 50%'
const RATE_ESTIMATE = 'Short-run estimate, not a vendor limit'
const ERROR_RATE = 'Normal < 10%; slower at 10%+'
const SUCCESS_RULE = 'Most requests succeed'

export function worstVerdict(verdicts: Verdict[]): Verdict {
  if (verdicts.includes('slow')) return 'slow'
  if (verdicts.includes('ok')) return 'ok'
  return 'na'
}

export function displayMeasured(
  row: MetricRow,
  t: (key: string, options?: Record<string, string | number>) => string
): string {
  if (row.measuredValues || row.measured.includes('{{')) {
    return t(row.measured, row.measuredValues)
  }
  if (/[A-Za-z]/.test(row.measured) && !/^\d/.test(row.measured)) {
    return t(row.measured)
  }
  return row.measured
}

export function overallLabel(verdict: Verdict): string {
  if (verdict === 'ok') return 'Overall: normal'
  if (verdict === 'slow') return 'Overall: slow'
  return 'Overall: cannot compare'
}

export function formatMs(value: number | undefined): string {
  if (value === undefined || !Number.isFinite(value) || value <= 0) return ''
  if (value < 10) return `${value.toFixed(1)} ms`
  return `${Math.round(value)} ms`
}

export function formatPercent(value: number | undefined): string {
  if (value === undefined || !Number.isFinite(value)) return ''
  return `${(value * 100).toFixed(1)}%`
}

export function formatCount(value: number | undefined, unit: string): string {
  if (value === undefined || !Number.isFinite(value) || value <= 0) return ''
  if (value >= 10000) return `${(value / 1000).toFixed(1)}k ${unit}`
  if (Number.isInteger(value)) return `${value} ${unit}`
  return `${value.toFixed(1)} ${unit}`
}

function sampleOr(value: string, fallback = NO_SAMPLE): string {
  return value || fallback
}

function ttftBand(ms: number, longInput: boolean): Verdict {
  if (!(ms > 0)) return 'na'
  const limit = longInput ? 15000 : 8000
  if (ms <= limit) return 'ok'
  return 'slow'
}

function tpotAvgBand(ms: number): Verdict {
  if (!(ms > 0)) return 'na'
  if (ms <= 80) return 'ok'
  return 'slow'
}

function tpotP90Band(ms: number): Verdict {
  if (!(ms > 0)) return 'na'
  if (ms <= 150) return 'ok'
  return 'slow'
}

function hitBand(rate: number): Verdict {
  if (!(rate >= 0)) return 'na'
  if (rate >= 0.5) return 'ok'
  return 'slow'
}

export function assessStress(metrics: StressMetrics): Assessment {
  const promptTokens = metrics.prompt_tokens ?? 0
  const avgPrompt =
    metrics.succeeded > 0 && promptTokens > 0
      ? promptTokens / metrics.succeeded
      : 0
  const longInput = avgPrompt >= 8000
  const ttftThreshold = longInput ? TTFT_LONG : TTFT_SHORT

  let errorVerdict: Verdict = 'ok'
  if (metrics.error_rate >= 0.1) {
    errorVerdict = 'slow'
  }

  const rows: MetricRow[] = [
    {
      id: 'error_rate',
      label: 'Error rate',
      measured: formatPercent(metrics.error_rate) || '0.0%',
      threshold: ERROR_RATE,
      verdict: errorVerdict,
    },
    {
      id: 'success',
      label: 'Succeeded',
      measured: `${metrics.succeeded}/${metrics.total}`,
      threshold: SUCCESS_RULE,
      verdict: errorVerdict,
    },
    {
      id: 'duration',
      label: 'Total duration',
      measured:
        metrics.elapsed_ms >= 1000
          ? `${(metrics.elapsed_ms / 1000).toFixed(2)} s`
          : `${Math.round(metrics.elapsed_ms)} ms`,
      threshold: 'Wall time of this run',
      verdict: 'na',
    },
  ]

  if (metrics.ttft_avg_ms > 0) {
    let p90Verdict = ttftBand(metrics.ttft_p90_ms, longInput)
    if (metrics.ttft_avg_ms > 0 && metrics.ttft_p90_ms > metrics.ttft_avg_ms * 3) {
      p90Verdict = 'slow'
    }
    rows.push(
      {
        id: 'ttft_avg',
        label: 'TTFT avg',
        measured: formatMs(metrics.ttft_avg_ms),
        threshold: ttftThreshold,
        verdict: ttftBand(metrics.ttft_avg_ms, longInput),
      },
      {
        id: 'ttft_p50',
        label: 'TTFT P50',
        measured: formatMs(metrics.ttft_p50_ms),
        threshold: ttftThreshold,
        verdict: ttftBand(metrics.ttft_p50_ms, longInput),
      },
      {
        id: 'ttft_p90',
        label: 'TTFT P90',
        measured: formatMs(metrics.ttft_p90_ms),
        threshold: TTFT_P90,
        verdict: p90Verdict,
      }
    )
  } else {
    rows.push(
      {
        id: 'ttft_avg',
        label: 'TTFT avg',
        measured: NO_SAMPLE,
        threshold: ttftThreshold,
        verdict: 'na',
      },
      {
        id: 'ttft_p50',
        label: 'TTFT P50',
        measured: NO_SAMPLE,
        threshold: ttftThreshold,
        verdict: 'na',
      },
      {
        id: 'ttft_p90',
        label: 'TTFT P90',
        measured: NO_SAMPLE,
        threshold: TTFT_P90,
        verdict: 'na',
      }
    )
  }

  if (metrics.tpot_avg_ms > 0) {
    rows.push(
      {
        id: 'tpot_avg',
        label: 'TPOT avg',
        measured: formatMs(metrics.tpot_avg_ms),
        threshold: TPOT_AVG,
        verdict: tpotAvgBand(metrics.tpot_avg_ms),
      },
      {
        id: 'tpot_p50',
        label: 'TPOT P50',
        measured: formatMs(metrics.tpot_p50_ms),
        threshold: TPOT_AVG,
        verdict: tpotAvgBand(metrics.tpot_p50_ms),
      },
      {
        id: 'tpot_p90',
        label: 'TPOT P90',
        measured: formatMs(metrics.tpot_p90_ms),
        threshold: TPOT_P90,
        verdict: tpotP90Band(metrics.tpot_p90_ms),
      }
    )
  } else {
    rows.push(
      {
        id: 'tpot_avg',
        label: 'TPOT avg',
        measured: NO_SAMPLE,
        threshold: TPOT_AVG,
        verdict: 'na',
      },
      {
        id: 'tpot_p50',
        label: 'TPOT P50',
        measured: NO_SAMPLE,
        threshold: TPOT_AVG,
        verdict: 'na',
      },
      {
        id: 'tpot_p90',
        label: 'TPOT P90',
        measured: NO_SAMPLE,
        threshold: TPOT_P90,
        verdict: 'na',
      }
    )
  }

  rows.push(
    {
      id: 'tps',
      label: 'Throughput',
      measured: sampleOr(formatCount(metrics.tokens_per_sec, 'tok/s')),
      threshold: 'Observed completion tokens per second',
      verdict: 'na',
    },
    {
      id: 'rpm',
      label: 'RPM (estimate)',
      measured: sampleOr(formatCount(metrics.rpm, 'req/min')),
      threshold: RATE_ESTIMATE,
      verdict: 'na',
    },
    {
      id: 'tpm',
      label: 'TPM (estimate)',
      measured: sampleOr(formatCount(metrics.tpm, 'tok/min')),
      threshold: RATE_ESTIMATE,
      verdict: 'na',
    }
  )

  return { rows, overall: worstVerdict(rows.map((row) => row.verdict)) }
}

export function assessCache(metrics: CacheMetrics | null): Assessment {
  if (!metrics) {
    return { rows: [], overall: 'na' }
  }
  if (!metrics.has_cached_tokens) {
    return {
      rows: [
        {
          id: 'hit',
          label: 'Cache hit rate',
          measured: 'No cached_tokens field',
          threshold: HIT_RATE,
          verdict: 'na',
        },
        {
          id: 'ttl',
          label: 'Cache TTL',
          measured: 'Cannot compare',
          threshold: TTL_RULE,
          verdict: 'na',
        },
      ],
      overall: 'na',
    }
  }

  const hitVerdict = hitBand(metrics.avg_hit_rate)
  const hitText = formatPercent(metrics.avg_hit_rate)
  let ttlRow: MetricRow = {
    id: 'ttl',
    label: 'Cache TTL',
    measured: 'Waited {{seconds}}s (need ≥ 30s)',
    measuredValues: { seconds: metrics.wait_seconds },
    threshold: TTL_RULE,
    verdict: 'na',
  }
  if (metrics.wait_seconds >= 30) {
    ttlRow = {
      ...ttlRow,
      measured: 'Waited {{seconds}}s, hit {{rate}}',
      measuredValues: { seconds: metrics.wait_seconds, rate: hitText },
      verdict: hitVerdict,
    }
  }

  const rows: MetricRow[] = [
    {
      id: 'hit',
      label: 'Cache hit rate',
      measured: hitText,
      threshold: HIT_RATE,
      verdict: hitVerdict,
    },
    ttlRow,
  ]
  return { rows, overall: worstVerdict(rows.map((row) => row.verdict)) }
}
