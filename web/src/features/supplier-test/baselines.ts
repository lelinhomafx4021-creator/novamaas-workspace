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

export type MetricGroup = 'shallow' | 'perf'

export type MetricRow = {
  id: string
  label: string
  measured: string
  measuredValues?: Record<string, string | number>
  threshold: string
  thresholdValues?: Record<string, string | number>
  verdict: Verdict
  group?: MetricGroup
}

export type SupplierStandard = {
  id: string
  labelKey: string
  errorSlow: number
  ttftShortOkMs: number
  ttftLongOkMs: number
  ttftP90AvgTimes: number
  tpotAvgOkMs: number
  tpotP90OkMs: number
  cacheHitOk: number
  ttlWaitSeconds: number
  longInputTokens: number
}

export const SUPPLIER_STANDARDS: SupplierStandard[] = [
  {
    id: 'default',
    labelKey: 'Default standard',
    errorSlow: 0.1,
    ttftShortOkMs: 8000,
    ttftLongOkMs: 15000,
    ttftP90AvgTimes: 3,
    tpotAvgOkMs: 80,
    tpotP90OkMs: 150,
    cacheHitOk: 0.5,
    ttlWaitSeconds: 30,
    longInputTokens: 8000,
  },
  {
    id: 'tight',
    labelKey: 'Tight standard',
    errorSlow: 0.05,
    ttftShortOkMs: 3000,
    ttftLongOkMs: 8000,
    ttftP90AvgTimes: 2,
    tpotAvgOkMs: 50,
    tpotP90OkMs: 80,
    cacheHitOk: 0.8,
    ttlWaitSeconds: 30,
    longInputTokens: 8000,
  },
]

export const DEFAULT_STANDARD = SUPPLIER_STANDARDS[0]

const STANDARD_NUMBER_KEYS = [
  'errorSlow',
  'ttftShortOkMs',
  'ttftLongOkMs',
  'ttftP90AvgTimes',
  'tpotAvgOkMs',
  'tpotP90OkMs',
  'cacheHitOk',
  'ttlWaitSeconds',
  'longInputTokens',
] as const

function clampNumber(value: unknown, fallback: number, min: number, max: number): number {
  const next = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(next)) return fallback
  return Math.min(max, Math.max(min, next))
}

export function getStandard(id: string): SupplierStandard {
  return SUPPLIER_STANDARDS.find((item) => item.id === id) ?? DEFAULT_STANDARD
}

export function sanitizeStandard(
  raw?: Partial<SupplierStandard> | null
): SupplierStandard {
  const base = DEFAULT_STANDARD
  return {
    id: typeof raw?.id === 'string' && raw.id.trim() ? raw.id.trim() : 'custom',
    labelKey:
      typeof raw?.labelKey === 'string' && raw.labelKey.trim()
        ? raw.labelKey.trim()
        : 'Custom standard',
    errorSlow: clampNumber(raw?.errorSlow, base.errorSlow, 0, 1),
    ttftShortOkMs: clampNumber(raw?.ttftShortOkMs, base.ttftShortOkMs, 1, 120000),
    ttftLongOkMs: clampNumber(raw?.ttftLongOkMs, base.ttftLongOkMs, 1, 180000),
    ttftP90AvgTimes: clampNumber(
      raw?.ttftP90AvgTimes,
      base.ttftP90AvgTimes,
      1,
      10
    ),
    tpotAvgOkMs: clampNumber(raw?.tpotAvgOkMs, base.tpotAvgOkMs, 1, 5000),
    tpotP90OkMs: clampNumber(raw?.tpotP90OkMs, base.tpotP90OkMs, 1, 5000),
    cacheHitOk: clampNumber(raw?.cacheHitOk, base.cacheHitOk, 0, 1),
    ttlWaitSeconds: clampNumber(
      raw?.ttlWaitSeconds,
      base.ttlWaitSeconds,
      0,
      600
    ),
    longInputTokens: clampNumber(
      raw?.longInputTokens,
      base.longInputTokens,
      1,
      256000
    ),
  }
}

export function matchingStandardId(standard: SupplierStandard): string | null {
  const found = SUPPLIER_STANDARDS.find((preset) =>
    STANDARD_NUMBER_KEYS.every(
      (key) => Math.abs(preset[key] - standard[key]) < 1e-9
    )
  )
  return found?.id ?? null
}

export type Assessment = {
  rows: MetricRow[]
  overall: Verdict
}

const NO_SAMPLE = 'No sample (enable stream)'
const TTFT_RULE = 'Normal ≤ {{seconds}}s; slower above that'
const TTFT_P90 =
  'Normal ≤ {{seconds}}s and ≤ avg × {{times}}; slower above that'
const TPOT_RULE = 'Normal ≤ {{ms}}ms; slower above that'
const HIT_RATE = 'Normal ≥ {{percent}}%; slower below that'
const TTL_RULE = 'Wait ≥ {{seconds}}s and hit rate still ≥ {{percent}}%'
const RATE_ESTIMATE = 'Short-run estimate, not a vendor limit'
const ERROR_RATE = 'Normal < {{percent}}%; slower at that or above'
const SUCCESS_RULE = 'Most requests succeed'
const TOKEN_RULE = 'Used as the long-input TTFT cutoff; not a tokenizer audit'

export function worstVerdict(verdicts: Verdict[]): Verdict {
  if (verdicts.includes('slow')) return 'slow'
  if (verdicts.includes('ok')) return 'ok'
  return 'na'
}

export function assessmentGroup(
  assessment: Assessment,
  group: MetricGroup
): Assessment {
  const rows = assessment.rows.filter(
    (row) => (row.group ?? 'perf') === group
  )
  return { rows, overall: worstVerdict(rows.map((row) => row.verdict)) }
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

export function displayThreshold(
  row: MetricRow,
  t: (key: string, options?: Record<string, string | number>) => string
): string {
  return t(row.threshold, row.thresholdValues)
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

function ttftBand(ms: number, longInput: boolean, standard: SupplierStandard): Verdict {
  if (!(ms > 0)) return 'na'
  const limit = longInput ? standard.ttftLongOkMs : standard.ttftShortOkMs
  if (ms <= limit) return 'ok'
  return 'slow'
}

function tpotAvgBand(ms: number, standard: SupplierStandard): Verdict {
  if (!(ms > 0)) return 'na'
  if (ms <= standard.tpotAvgOkMs) return 'ok'
  return 'slow'
}

function tpotP90Band(ms: number, standard: SupplierStandard): Verdict {
  if (!(ms > 0)) return 'na'
  if (ms <= standard.tpotP90OkMs) return 'ok'
  return 'slow'
}

function hitBand(rate: number, standard: SupplierStandard): Verdict {
  if (!(rate >= 0)) return 'na'
  if (rate >= standard.cacheHitOk) return 'ok'
  return 'slow'
}

function percentValue(rate: number): number {
  return Math.round(rate * 1000) / 10
}

export function assessStress(
  metrics: StressMetrics,
  standard: SupplierStandard = DEFAULT_STANDARD
): Assessment {
  const promptTokens = metrics.prompt_tokens ?? 0
  const avgPrompt =
    metrics.succeeded > 0 && promptTokens > 0
      ? promptTokens / metrics.succeeded
      : 0
  const longInput = avgPrompt >= standard.longInputTokens
  const ttftLimit = longInput ? standard.ttftLongOkMs : standard.ttftShortOkMs
  const ttftThreshold = TTFT_RULE
  const ttftValues = { seconds: ttftLimit / 1000 }

  let errorVerdict: Verdict = 'ok'
  if (metrics.error_rate >= standard.errorSlow) {
    errorVerdict = 'slow'
  }

  const rows: MetricRow[] = [
    {
      id: 'error_rate',
      label: 'Error rate',
      measured: formatPercent(metrics.error_rate) || '0.0%',
      threshold: ERROR_RATE,
      thresholdValues: { percent: percentValue(standard.errorSlow) },
      verdict: errorVerdict,
      group: 'shallow',
    },
    {
      id: 'success',
      label: 'Succeeded',
      measured: `${metrics.succeeded}/${metrics.total}`,
      threshold: SUCCESS_RULE,
      verdict: errorVerdict,
      group: 'shallow',
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
      group: 'shallow',
    },
  ]

  if (metrics.ttft_avg_ms > 0) {
    let p90Verdict = ttftBand(metrics.ttft_p90_ms, longInput, standard)
    if (
      metrics.ttft_avg_ms > 0 &&
      metrics.ttft_p90_ms > metrics.ttft_avg_ms * standard.ttftP90AvgTimes
    ) {
      p90Verdict = 'slow'
    }
    rows.push(
      {
        id: 'ttft_avg',
        label: 'TTFT avg',
        measured: formatMs(metrics.ttft_avg_ms),
        threshold: ttftThreshold,
        thresholdValues: ttftValues,
        verdict: ttftBand(metrics.ttft_avg_ms, longInput, standard),
      },
      {
        id: 'ttft_p50',
        label: 'TTFT P50',
        measured: formatMs(metrics.ttft_p50_ms),
        threshold: ttftThreshold,
        thresholdValues: ttftValues,
        verdict: ttftBand(metrics.ttft_p50_ms, longInput, standard),
      },
      {
        id: 'ttft_p90',
        label: 'TTFT P90',
        measured: formatMs(metrics.ttft_p90_ms),
        threshold: TTFT_P90,
        thresholdValues: {
          seconds: ttftLimit / 1000,
          times: standard.ttftP90AvgTimes,
        },
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
        thresholdValues: ttftValues,
        verdict: 'na',
      },
      {
        id: 'ttft_p50',
        label: 'TTFT P50',
        measured: NO_SAMPLE,
        threshold: ttftThreshold,
        thresholdValues: ttftValues,
        verdict: 'na',
      },
      {
        id: 'ttft_p90',
        label: 'TTFT P90',
        measured: NO_SAMPLE,
        threshold: TTFT_P90,
        thresholdValues: {
          seconds: ttftLimit / 1000,
          times: standard.ttftP90AvgTimes,
        },
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
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotAvgOkMs },
        verdict: tpotAvgBand(metrics.tpot_avg_ms, standard),
      },
      {
        id: 'tpot_p50',
        label: 'TPOT P50',
        measured: formatMs(metrics.tpot_p50_ms),
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotAvgOkMs },
        verdict: tpotAvgBand(metrics.tpot_p50_ms, standard),
      },
      {
        id: 'tpot_p90',
        label: 'TPOT P90',
        measured: formatMs(metrics.tpot_p90_ms),
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotP90OkMs },
        verdict: tpotP90Band(metrics.tpot_p90_ms, standard),
      }
    )
  } else {
    rows.push(
      {
        id: 'tpot_avg',
        label: 'TPOT avg',
        measured: NO_SAMPLE,
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotAvgOkMs },
        verdict: 'na',
      },
      {
        id: 'tpot_p50',
        label: 'TPOT P50',
        measured: NO_SAMPLE,
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotAvgOkMs },
        verdict: 'na',
      },
      {
        id: 'tpot_p90',
        label: 'TPOT P90',
        measured: NO_SAMPLE,
        threshold: TPOT_RULE,
        thresholdValues: { ms: standard.tpotP90OkMs },
        verdict: 'na',
      }
    )
  }

  rows.push(
    {
      id: 'tokens',
      label: 'Prompt / completion tokens',
      measured: `${metrics.prompt_tokens} / ${metrics.completion_tokens}`,
      threshold: TOKEN_RULE,
      verdict: 'na',
      group: 'perf',
    },
    {
      id: 'tps',
      label: 'Throughput',
      measured: sampleOr(formatCount(metrics.tokens_per_sec, 'tok/s')),
      threshold: 'Observed completion tokens per second',
      verdict: 'na',
      group: 'perf',
    },
    {
      id: 'rpm',
      label: 'RPM (estimate)',
      measured: sampleOr(formatCount(metrics.rpm, 'req/min')),
      threshold: RATE_ESTIMATE,
      verdict: 'na',
      group: 'perf',
    },
    {
      id: 'tpm',
      label: 'TPM (estimate)',
      measured: sampleOr(formatCount(metrics.tpm, 'tok/min')),
      threshold: RATE_ESTIMATE,
      verdict: 'na',
      group: 'perf',
    }
  )

  return { rows, overall: worstVerdict(rows.map((row) => row.verdict)) }
}

export function assessCache(
  metrics: CacheMetrics | null,
  standard: SupplierStandard = DEFAULT_STANDARD
): Assessment {
  const hitValues = { percent: percentValue(standard.cacheHitOk) }
  const ttlValues = {
    seconds: standard.ttlWaitSeconds,
    percent: percentValue(standard.cacheHitOk),
  }
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
          thresholdValues: hitValues,
          verdict: 'na',
        },
        {
          id: 'ttl',
          label: 'Cache TTL',
          measured: 'Cannot compare',
          threshold: TTL_RULE,
          thresholdValues: ttlValues,
          verdict: 'na',
        },
      ],
      overall: 'na',
    }
  }

  const hitVerdict = hitBand(metrics.avg_hit_rate, standard)
  const hitText = formatPercent(metrics.avg_hit_rate)
  let ttlRow: MetricRow = {
    id: 'ttl',
    label: 'Cache TTL',
    measured: 'Waited {{seconds}}s (need ≥ {{need}}s)',
    measuredValues: {
      seconds: metrics.wait_seconds,
      need: standard.ttlWaitSeconds,
    },
    threshold: TTL_RULE,
    thresholdValues: ttlValues,
    verdict: 'na',
  }
  if (metrics.wait_seconds >= standard.ttlWaitSeconds) {
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
      thresholdValues: hitValues,
      verdict: hitVerdict,
    },
    ttlRow,
  ]
  return { rows, overall: worstVerdict(rows.map((row) => row.verdict)) }
}
