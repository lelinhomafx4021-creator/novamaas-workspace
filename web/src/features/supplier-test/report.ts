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
import { api } from '@/lib/api'

import {
  displayMeasured,
  displayThreshold,
  formatPercent,
  isInformationalRow,
  worstVerdict,
  type Assessment,
  type Verdict,
} from './baselines'
import { PROTOCOL_BASIC_IDS, SHALLOW_BASIC_IDS } from './constants'
import { checkMessage } from './formatters'
import {
  escapeHtml,
  pdfLetterhead,
  pdfPageFooterStyles,
  pdfRow,
  pdfSummaryCards,
  pdfDocumentStyles,
  printHtmlReport,
} from './report-layout'
import type {
  CacheMetrics,
  CheckResult,
  CheckStatus,
  VideoRunReport,
  StressMetrics,
} from './types'
import {
  buildVideoRunHtmlReport,
  buildVideoRunMarkdownReport,
} from './video-report'

export type StressConfig = {
  concurrency: number
  rounds: number
  stream: boolean
  breakCache: boolean
  maxTokens?: number
  corpus?: string
}

export type ReportInput = {
  language?: string
  branding?: ReportBranding
  baseUrl: string
  model: string
  vendor?: string
  basicChecks: CheckResult[]
  cacheChecks: CheckResult[]
  summaries: { basic: string; cache: string; stress: string }
  stressAssessment: Assessment | null
  cacheAssessment: Assessment | null
  errorMessage: string
  standardLabel: string
  statusLabel: (status: CheckStatus) => string
  stressConfig?: StressConfig
  stressMetrics?: StressMetrics | null
  cacheMetrics?: CacheMetrics | null
  t: (key: string, options?: Record<string, string | number>) => string
}

export type ReportBranding = {
  platformName: string
  platformLogo: string
  operatingEntityName: string
  operatingEntityLogo: string
}

export function stampFileName(): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
}

export function supplierReportName(t: ReportInput['t']): string {
  return `${translate(t, 'Supplier Test')}${stampFileName().slice(0, 8)}`
}

export function downloadFile(filename: string, content: string, mime: string) {
  const blob = new Blob([content], { type: `${mime};charset=utf-8` })
  const href = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = href
  link.download = filename
  link.click()
  URL.revokeObjectURL(href)
}

export async function exportPdfReport(input: ReportInput): Promise<void> {
  const branding = await preparePdfBranding(input.branding)
  const html = buildHtmlReport({ ...input, branding }, { format: 'pdf' })
  await printHtmlReport(html, 'supplier-test-report-print')
}

export async function preparePdfBranding(
  branding: ReportBranding | undefined
): Promise<ReportBranding | undefined> {
  if (!branding?.operatingEntityLogo) return branding

  const response = await api.get<Blob>('/api/pdf-branding/operating-logo', {
    responseType: 'blob',
    skipErrorHandler: true,
  })
  if (!(response.data instanceof Blob) || response.data.type !== 'image/png') {
    throw new Error('Unable to load PDF branding image')
  }

  const logo = await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.addEventListener(
      'load',
      () => {
        if (typeof reader.result === 'string') resolve(reader.result)
        else reject(new Error('Unable to load PDF branding image'))
      },
      { once: true }
    )
    reader.addEventListener(
      'error',
      () => reject(new Error('Unable to load PDF branding image')),
      { once: true }
    )
    reader.readAsDataURL(response.data)
  })
  return { ...branding, operatingEntityLogo: logo }
}

function translate(
  t: ReportInput['t'],
  key: string,
  options?: Record<string, string | number>
): string {
  const value = t(key, options)
  return value || key
}

export function formatTokenCompact(tokens: number | undefined): string {
  if (tokens === undefined || !Number.isFinite(tokens) || tokens <= 0) {
    return '0'
  }
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)}k`
  return String(tokens)
}

function checkExplanation(check: CheckResult, t: ReportInput['t']): string {
  const message = checkMessage(check, t)
  if (message) return message
  if (check.status === 'skip' && check.hintKey) return t(check.hintKey)
  return ''
}

function checkLines(
  checks: CheckResult[],
  input: ReportInput,
  t: ReportInput['t']
): string[] {
  return checks.map((check) => {
    const explanation = checkExplanation(check, t)
    const reasonLabel = check.status === 'skip' ? `**${t('Reason')}:** ` : ''
    const detail = explanation ? ` — ${reasonLabel}${explanation}` : ''
    return `- ${t(check.title)}: **${t(input.statusLabel(check.status))}**${detail}`
  })
}

function assessmentResultLabel(verdict: Verdict): string {
  if (verdict === 'ok') return 'Passed'
  if (verdict === 'na') return 'Skipped'
  return 'Failed'
}

function markdownTable(assessment: Assessment, t: ReportInput['t']): string[] {
  const benchmarkRows = assessment.rows.filter(
    (row) => !isInformationalRow(row)
  )
  if (benchmarkRows.length === 0) return []
  const lines = [
    `| ${t('Metric')} | ${t('Measured')} | ${t('Threshold')} | ${t('Verdict')} |`,
    '|---|---|---|---|',
  ]
  for (const row of benchmarkRows) {
    lines.push(
      `| ${t(row.label)} | ${displayMeasured(row, t)} | ${displayThreshold(row, t)} | ${t(assessmentResultLabel(row.verdict))} |`
    )
  }
  return lines
}

export function buildMarkdownReport(input: ReportInput): string {
  const t: ReportInput['t'] = (key, options) => translate(input.t, key, options)
  const reportName = supplierReportName(t)
  const shallowChecks = input.basicChecks.filter((check) =>
    (SHALLOW_BASIC_IDS as readonly string[]).includes(check.id)
  )
  const protocolChecks = input.basicChecks.filter((check) => {
    if (check.id === 'kimi_kvv' && input.vendor && input.vendor !== 'kimi') {
      return false
    }
    return (PROTOCOL_BASIC_IDS as readonly string[]).includes(check.id)
  })
  const lines = [
    `# ${reportName}`,
    '',
    `- ${t('Time')}: ${new Date().toLocaleString()}`,
    `- ${t('Base URL')}: ${input.baseUrl || '-'}`,
    `- ${t('Model')}: ${input.model || '-'}`,
    `- ${t('Judgment standard')}: ${input.standardLabel}`,
    '',
    `## 1. ${t('Connectivity and protocol')}`,
    `### 1.1 ${t('Basic connectivity checks')}`,
    ...checkLines(shallowChecks, input, t),
    '',
    `### 1.2 ${t('Advanced protocol capabilities')}`,
    ...checkLines(protocolChecks, input, t),
  ]
  if (input.summaries.basic) {
    lines.push('', input.summaries.basic)
  }

  // Section 2: Stress test
  if (input.stressAssessment || input.stressMetrics || input.summaries.stress) {
    lines.push('', `## 2. ${t('Concurrency and stress test')}`)
    if (input.stressAssessment) {
      lines.push(
        `**${t('Stress test assessment')}：${t(assessmentResultLabel(input.stressAssessment.overall))}**`,
        ''
      )
    }
    if (input.stressConfig) {
      const cfg = input.stressConfig
      const modeText = cfg.stream ? t('Stream') : t('Non-stream')
      const cacheText = cfg.breakCache ? t('Break cache') : t('Allow cache')
      const totalReq = cfg.concurrency * cfg.rounds
      lines.push(
        `- ${t('Concurrency spec')}: ${cfg.concurrency} × ${cfg.rounds} (${totalReq})`,
        `- ${t('Transmission mode')}: ${modeText}`,
        `- ${t('Cache strategy')}: ${cacheText}`,
        `- ${t('Max tokens cap')}: max_tokens = ${cfg.maxTokens ?? 1024}`,
        ''
      )
    }
    if (input.stressMetrics) {
      const sm = input.stressMetrics
      const completeUsage =
        sm.usage_n === undefined || sm.usage_n === sm.succeeded
      lines.push(
        `- ${t('Success / failed')}: ${sm.succeeded} / ${sm.failed}`,
        `- ${t('Requests completed / not run')}: ${sm.attempted ?? sm.total} / ${sm.not_run ?? 0}`,
        `- ${t('Batch wall time')}: ${(sm.elapsed_ms / 1000).toFixed(2)} s`,
        `- ${t('Batch output throughput')}: ${completeUsage ? `${sm.tokens_per_sec.toFixed(1)} tok/s` : '—'}`,
        `- ${t('Per-request output rate')}: ${completeUsage && sm.request_tokens_per_sec ? `${sm.request_tokens_per_sec.toFixed(1)} tok/s` : '—'}`,
        `- ${t('Request duration avg / P50 / P90')}: ${sm.request_avg_ms ? `${(sm.request_avg_ms / 1000).toFixed(2)} / ${((sm.request_p50_ms ?? 0) / 1000).toFixed(2)} / ${((sm.request_p90_ms ?? 0) / 1000).toFixed(2)} s` : '—'}`,
        `- ${t('Tokens (prompt / completion)')}: ${formatTokenCompact(sm.prompt_tokens)} / ${formatTokenCompact(sm.completion_tokens)}`,
        `- ${t('Estimated capacity')}: ${Math.round(sm.rpm)} RPM / ${completeUsage ? formatTokenCompact(sm.tpm) : '—'} TPM`,
        ''
      )
      const missingUsage = Math.max(
        0,
        sm.succeeded - (sm.usage_n ?? sm.succeeded)
      )
      const notRun =
        sm.not_run ?? Math.max(0, sm.total - (sm.attempted ?? sm.total))
      const missingTTFT = input.stressConfig?.stream
        ? Math.max(0, sm.succeeded - sm.ttft_n)
        : 0
      const missingTPOT = input.stressConfig?.stream
        ? Math.max(0, sm.ttft_n - sm.tpot_n)
        : 0
      const slowRows =
        input.stressAssessment?.rows.filter(
          (row) => row.verdict === 'slow' || row.verdict === 'abnormal'
        ) ?? []
      lines.push(`### ${t('Problems found')}`)
      if (
        sm.failed === 0 &&
        notRun === 0 &&
        missingUsage === 0 &&
        missingTTFT === 0 &&
        missingTPOT === 0 &&
        slowRows.length === 0
      ) {
        lines.push(
          t('No request errors or threshold violations were found in this run.')
        )
      }
      for (const issue of sm.issues ?? []) {
        const status = issue.status_code || t('No HTTP response')
        lines.push(
          `- ${issue.count} × ${status}: ${issue.message.replaceAll('|', '\\|')} (W${issue.worker}/R${issue.round}, ${(issue.elapsed_ms / 1000).toFixed(2)} s)`
        )
      }
      if (sm.other_issue_count) {
        lines.push(
          `- ${t('{{count}} additional failures are included in the failed total but omitted from the detailed list.', { count: sm.other_issue_count })}`
        )
      }
      if (sm.failed > 0 && !sm.issues?.length && !sm.other_issue_count) {
        lines.push(`- ${t('Failure details are unavailable for this run.')}`)
      }
      if (missingUsage > 0) {
        lines.push(
          `- ${t('{{count}} successful requests had no complete token usage; token rates and TPM are unavailable.', { count: missingUsage })}`
        )
      }
      if (notRun > 0) {
        lines.push(
          `- ${t('{{count}} planned requests were not started because the run stopped or was cancelled.', { count: notRun })}`
        )
      }
      if (missingTTFT > 0) {
        lines.push(
          `- ${t('{{count}} successful stream requests had no observable first output; TTFT is unavailable for them.', { count: missingTTFT })}`
        )
      }
      if (missingTPOT > 0) {
        lines.push(
          `- ${t('{{count}} streamed responses lacked enough timed output and token usage to calculate TPOT.', { count: missingTPOT })}`
        )
      }
      for (const row of slowRows) {
        lines.push(
          `- ${t(row.label)}: ${displayMeasured(row, t)} · ${displayThreshold(row, t)}`
        )
      }
      lines.push('')
    }
    if (input.stressAssessment) {
      lines.push(...markdownTable(input.stressAssessment, t))
    }
    if (input.summaries.stress) {
      lines.push('', input.summaries.stress)
    }
  }

  // Section 3: Cache test
  if (input.cacheAssessment || input.cacheMetrics || input.summaries.cache) {
    lines.push('', `## 3. ${t('Prompt cache test')}`)
    if (input.cacheAssessment) {
      lines.push(
        `**${t('Prompt cache assessment')}：${t(assessmentResultLabel(input.cacheAssessment.overall))}**`,
        ''
      )
    }
    if (input.cacheChecks.length > 0) {
      lines.push(
        `### 3.1 ${t('Cache probe details')}`,
        ...checkLines(input.cacheChecks, input, t),
        ''
      )
    }
    if (input.cacheMetrics) {
      const cm = input.cacheMetrics
      const modeLabel =
        cm.mode === 'cumulative' ? t('Cumulative chat') : t('Static prefix')
      const freqText =
        cm.hit_count !== undefined
          ? `${cm.hit_count}/${cm.rounds} (${formatPercent(cm.rounds > 0 ? (cm.hit_count ?? 0) / cm.rounds : 0)})`
          : '-'
      lines.push(
        `- ${t('Cache mode')}: ${modeLabel}`,
        `- ${t('Probe rounds')}: ${cm.rounds}`,
        `- ${t('Hit frequency')}: ${freqText}`,
        `- ${t('Hit depth')}: ${cm.avg_depth_rate !== undefined ? formatPercent(cm.avg_depth_rate) : '-'}`,
        `- ${t('TTL wait (s)')}: ${cm.wait_seconds}s`,
        ''
      )
    }
    if (input.cacheAssessment) {
      lines.push(...markdownTable(input.cacheAssessment, t))
    }
    if (input.summaries.cache) {
      lines.push('', input.summaries.cache)
    }
  }

  if (input.errorMessage) {
    lines.push('', `## ${t('Error')}`, input.errorMessage)
  }
  return `${lines.join('\n')}\n`
}

function verdictHtmlColor(verdict: Assessment['overall']): string {
  if (verdict === 'ok') return '#15803d'
  if (verdict === 'na') return '#a16207'
  return '#dc2626'
}

function verdictHtmlClass(verdict: Assessment['overall']): string {
  if (verdict === 'ok') return 'verdict-ok'
  if (verdict === 'na') return 'verdict-na'
  return 'verdict-abnormal'
}

function renderCheckListHtml(
  checks: CheckResult[],
  input: ReportInput,
  t: ReportInput['t']
): string {
  if (checks.length === 0) return ''
  const items = checks
    .map((check) => {
      let statusCls = 'check-status-fail'
      if (check.status === 'pass') {
        statusCls = 'check-status-pass'
      } else if (check.status === 'skip') {
        statusCls = 'check-status-skip'
      }
      const explanation = checkExplanation(check, t)
      let detail = ''
      if (explanation) {
        if (check.status === 'skip') {
          detail = ` <span class="check-reason"><strong>${escapeHtml(t('Reason'))}:</strong> ${escapeHtml(explanation)}</span>`
        } else {
          detail = ` <span class="check-detail">— ${escapeHtml(explanation)}</span>`
        }
      }
      return `<li class="check-list-item">
<span class="check-bullet">•</span>
<span class="check-name">${escapeHtml(t(check.title))}：</span>
<span class="check-status ${statusCls}">${escapeHtml(t(input.statusLabel(check.status)))}</span>${detail}
</li>`
    })
    .join('')
  return `<ul class="check-list">${items}</ul>`
}

function htmlBenchmarkTable(
  assessment: Assessment,
  t: ReportInput['t']
): string {
  const benchmarkRows = assessment.rows.filter(
    (row) => !isInformationalRow(row)
  )
  if (benchmarkRows.length === 0) return ''
  const body = benchmarkRows
    .map((row) => {
      const color = verdictHtmlColor(row.verdict)
      return `<tr>
<td>${escapeHtml(t(row.label))}</td>
<td class="cell-measured">${escapeHtml(displayMeasured(row, t))}</td>
<td class="cell-threshold">${escapeHtml(displayThreshold(row, t))}</td>
<td style="color:${color};font-weight:700">${escapeHtml(t(assessmentResultLabel(row.verdict)))}</td>
</tr>`
    })
    .join('')
  return `<table class="benchmark-table">
<thead><tr><th style="width:25%">${escapeHtml(t('Metric'))}</th><th style="width:20%">${escapeHtml(t('Measured'))}</th><th style="width:40%">${escapeHtml(t('Threshold'))}</th><th style="width:15%">${escapeHtml(t('Verdict'))}</th></tr></thead>
<tbody>${body}</tbody>
</table>`
}

export function buildHtmlReport(
  input: ReportInput,
  options: { format?: 'html' | 'pdf' } = {}
): string {
  const t: ReportInput['t'] = (key, options) => translate(input.t, key, options)
  const reportName = supplierReportName(t)

  const shallowChecks = input.basicChecks.filter((check) =>
    (SHALLOW_BASIC_IDS as readonly string[]).includes(check.id)
  )
  const protocolChecks = input.basicChecks.filter((check) => {
    if (check.id === 'kimi_kvv' && input.vendor && input.vendor !== 'kimi') {
      return false
    }
    return (PROTOCOL_BASIC_IDS as readonly string[]).includes(check.id)
  })
  const basicResults = [...shallowChecks, ...protocolChecks]
  let basicBadge = 'All checks passed'
  let basicBadgeClass = 'verdict-ok'
  if (basicResults.some((check) => check.status === 'fail')) {
    basicBadge = 'Checks have failures'
    basicBadgeClass = 'verdict-abnormal'
  } else if (basicResults.some((check) => check.status === 'skip')) {
    basicBadge = 'Checks complete with skips'
    basicBadgeClass = 'verdict-na'
  }
  const assessedVerdicts: Verdict[] = []
  if (basicResults.some((check) => check.status === 'fail')) {
    assessedVerdicts.push('abnormal')
  } else if (basicResults.some((check) => check.status === 'pass')) {
    assessedVerdicts.push('ok')
  }
  if (input.stressAssessment && input.stressAssessment.overall !== 'na') {
    assessedVerdicts.push(input.stressAssessment.overall)
  }
  if (input.cacheAssessment && input.cacheAssessment.overall !== 'na') {
    assessedVerdicts.push(input.cacheAssessment.overall)
  }
  if (input.errorMessage) assessedVerdicts.push('abnormal')
  const pdfVerdict = worstVerdict(assessedVerdicts)
  const resultLabel = pdfVerdict === 'ok' ? 'Passed' : 'Failed'
  const resultClass = pdfVerdict === 'ok' ? 'result-pass' : 'result-fail'
  const pdfSummary = pdfSummaryCards([
    {
      label: t('Connectivity and protocol'),
      value: `${basicResults.filter((check) => check.status === 'pass').length} / ${basicResults.length}`,
      detail: t('Basic connectivity checks'),
    },
    {
      label: t('Requests'),
      value: input.stressMetrics
        ? `${input.stressMetrics.succeeded} / ${input.stressMetrics.total}`
        : '—',
      detail: t('Concurrency and stress test'),
    },
    {
      label: t('Result'),
      value: t(resultLabel),
      detail: t('Judgment standard'),
      valueClass: resultClass,
    },
  ])
  const pdfSubtitle = `SUPPLIER TEST  /  ${reportName}`

  // Section 2: Stress test HTML
  let stressSection = ''
  if (input.stressAssessment || input.stressMetrics || input.summaries.stress) {
    const overall = input.stressAssessment?.overall ?? 'ok'
    const overallCls = verdictHtmlClass(overall)
    const badgeText = `${t('Stress test assessment')}：${t(assessmentResultLabel(overall))}`

    let configBar = ''
    if (input.stressConfig) {
      const cfg = input.stressConfig
      const modeText = cfg.stream ? t('Stream') : t('Non-stream')
      const cacheText = cfg.breakCache ? t('Break cache') : t('Allow cache')
      const total = cfg.concurrency * cfg.rounds
      configBar = `<div class="config-bar">
<div class="config-item">${escapeHtml(t('Concurrency spec'))}：<span>${cfg.concurrency} × ${cfg.rounds} (${total})</span></div>
<div class="config-item">${escapeHtml(t('Transmission mode'))}：<span>${escapeHtml(modeText)}</span></div>
<div class="config-item">${escapeHtml(t('Cache strategy'))}：<span>${escapeHtml(cacheText)}</span></div>
<div class="config-item">${escapeHtml(t('Max tokens cap'))}：<span>max_tokens = ${cfg.maxTokens ?? 1024}</span></div>
</div>`
    }

    let statsGrid = ''
    let problemsHtml = ''
    if (input.stressMetrics) {
      const sm = input.stressMetrics
      const durationSec = (sm.elapsed_ms / 1000).toFixed(2)
      const completeUsage =
        sm.usage_n === undefined || sm.usage_n === sm.succeeded
      statsGrid = `<div class="stats-grid">
<div class="stat-card">
  <div class="stat-val">${sm.succeeded} / ${sm.failed}</div>
  <div class="stat-lbl">${escapeHtml(t('Success / failed'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${sm.attempted ?? sm.total} / ${sm.not_run ?? 0}</div>
  <div class="stat-lbl">${escapeHtml(t('Requests completed / not run'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${durationSec} s</div>
  <div class="stat-lbl">${escapeHtml(t('Batch wall time'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${completeUsage ? `${sm.tokens_per_sec.toFixed(1)} tok/s` : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('Batch output throughput'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${sm.request_avg_ms ? `${(sm.request_avg_ms / 1000).toFixed(2)} / ${((sm.request_p50_ms ?? 0) / 1000).toFixed(2)} / ${((sm.request_p90_ms ?? 0) / 1000).toFixed(2)} s` : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('Request duration avg / P50 / P90'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${completeUsage && sm.request_tokens_per_sec ? `${sm.request_tokens_per_sec.toFixed(1)} tok/s` : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('Per-request output rate'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${sm.ttft_n > 0 ? `${(sm.ttft_avg_ms / 1000).toFixed(2)} / ${(sm.ttft_p50_ms / 1000).toFixed(2)} / ${(sm.ttft_p90_ms / 1000).toFixed(2)} s` : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('TTFT avg / P50 / P90'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${sm.tpot_n > 0 ? `${sm.tpot_avg_ms.toFixed(1)} / ${sm.tpot_p50_ms.toFixed(1)} / ${sm.tpot_p90_ms.toFixed(1)} ms` : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('TPOT avg / P50 / P90'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${formatTokenCompact(sm.prompt_tokens)} / ${formatTokenCompact(sm.completion_tokens)}</div>
  <div class="stat-lbl">${escapeHtml(t('Tokens (prompt / completion)'))}</div>
</div>
<div class="stat-card">
  <div class="stat-val">${Math.round(sm.rpm)} / ${completeUsage ? formatTokenCompact(sm.tpm) : '—'}</div>
  <div class="stat-lbl">${escapeHtml(t('Estimated capacity (RPM/TPM)'))}</div>
</div>
</div>`
      const missingUsage = Math.max(
        0,
        sm.succeeded - (sm.usage_n ?? sm.succeeded)
      )
      const notRun =
        sm.not_run ?? Math.max(0, sm.total - (sm.attempted ?? sm.total))
      const missingTTFT = input.stressConfig?.stream
        ? Math.max(0, sm.succeeded - sm.ttft_n)
        : 0
      const missingTPOT = input.stressConfig?.stream
        ? Math.max(0, sm.ttft_n - sm.tpot_n)
        : 0
      const slowRows =
        input.stressAssessment?.rows.filter(
          (row) => row.verdict === 'slow' || row.verdict === 'abnormal'
        ) ?? []
      const issueLines = (sm.issues ?? []).map((issue) => {
        const status = issue.status_code || t('No HTTP response')
        return `<li>${issue.count} × ${escapeHtml(String(status))}: ${escapeHtml(issue.message)} (W${issue.worker}/R${issue.round}, ${(issue.elapsed_ms / 1000).toFixed(2)} s)</li>`
      })
      if (sm.other_issue_count) {
        issueLines.push(
          `<li>${escapeHtml(t('{{count}} additional failures are included in the failed total but omitted from the detailed list.', { count: sm.other_issue_count }))}</li>`
        )
      }
      if (sm.failed > 0 && !sm.issues?.length && !sm.other_issue_count) {
        issueLines.push(
          `<li>${escapeHtml(t('Failure details are unavailable for this run.'))}</li>`
        )
      }
      if (missingUsage > 0) {
        issueLines.push(
          `<li>${escapeHtml(t('{{count}} successful requests had no complete token usage; token rates and TPM are unavailable.', { count: missingUsage }))}</li>`
        )
      }
      if (notRun > 0) {
        issueLines.push(
          `<li>${escapeHtml(t('{{count}} planned requests were not started because the run stopped or was cancelled.', { count: notRun }))}</li>`
        )
      }
      if (missingTTFT > 0) {
        issueLines.push(
          `<li>${escapeHtml(t('{{count}} successful stream requests had no observable first output; TTFT is unavailable for them.', { count: missingTTFT }))}</li>`
        )
      }
      if (missingTPOT > 0) {
        issueLines.push(
          `<li>${escapeHtml(t('{{count}} streamed responses lacked enough timed output and token usage to calculate TPOT.', { count: missingTPOT }))}</li>`
        )
      }
      for (const row of slowRows) {
        issueLines.push(
          `<li>${escapeHtml(t(row.label))}: ${escapeHtml(displayMeasured(row, t))} · ${escapeHtml(displayThreshold(row, t))}</li>`
        )
      }
      if (issueLines.length === 0) {
        issueLines.push(
          `<li>${escapeHtml(t('No request errors or threshold violations were found in this run.'))}</li>`
        )
      }
      problemsHtml = `<div class="summary-text"><strong>${escapeHtml(t('Problems found'))}</strong><ul>${issueLines.join('')}</ul></div>`
    }

    const tableHtml = input.stressAssessment
      ? htmlBenchmarkTable(input.stressAssessment, t)
      : ''
    const summaryHtml = input.summaries.stress
      ? `<div class="summary-text">${escapeHtml(input.summaries.stress)}</div>`
      : ''

    stressSection = `<div class="section page-break-avoid">
<div class="section-header">
  <div class="section-title">2. ${escapeHtml(t('Concurrency and stress test'))}</div>
  <div class="verdict-badge ${overallCls}">${escapeHtml(badgeText)}</div>
</div>
${configBar}
${statsGrid}
${problemsHtml}
${tableHtml}
${summaryHtml}
</div>`
  }

  // Section 3: Cache test HTML
  let cacheSection = ''
  if (input.cacheAssessment || input.cacheMetrics || input.summaries.cache) {
    const overall = input.cacheAssessment?.overall ?? 'ok'
    const overallCls = verdictHtmlClass(overall)
    const badgeText = `${t('Prompt cache assessment')}：${t(assessmentResultLabel(overall))}`

    let configBar = ''
    if (input.cacheMetrics) {
      const cm = input.cacheMetrics
      const modeLabel =
        cm.mode === 'cumulative' ? t('Cumulative chat') : t('Static prefix')
      const freqText =
        cm.hit_count !== undefined
          ? `${cm.hit_count}/${cm.rounds} (${formatPercent(cm.rounds > 0 ? (cm.hit_count ?? 0) / cm.rounds : 0)})`
          : '-'
      const depthText =
        cm.avg_depth_rate !== undefined ? formatPercent(cm.avg_depth_rate) : '-'
      configBar = `<div class="config-bar">
<div class="config-item">${escapeHtml(t('Cache mode'))}：<span>${escapeHtml(modeLabel)}</span></div>
<div class="config-item">${escapeHtml(t('Probe rounds'))}：<span>${cm.rounds}</span></div>
<div class="config-item">${escapeHtml(t('Hit frequency'))}：<span>${escapeHtml(freqText)}</span></div>
<div class="config-item">${escapeHtml(t('Hit depth'))}：<span>${escapeHtml(depthText)}</span></div>
</div>`
    }

    const cacheChecksHtml =
      input.cacheChecks.length > 0
        ? `<div class="check-group-title">${escapeHtml(t('Cache probe details'))}</div>${renderCheckListHtml(input.cacheChecks, input, t)}`
        : ''

    const tableHtml = input.cacheAssessment
      ? htmlBenchmarkTable(input.cacheAssessment, t)
      : ''
    const summaryHtml = input.summaries.cache
      ? `<div class="summary-text">${escapeHtml(input.summaries.cache)}</div>`
      : ''

    cacheSection = `<div class="section page-break-avoid">
<div class="section-header">
  <div class="section-title">3. ${escapeHtml(t('Prompt cache test'))}</div>
  <div class="verdict-badge ${overallCls}">${escapeHtml(badgeText)}</div>
</div>
${cacheChecksHtml}
${configBar}
${tableHtml}
${summaryHtml}
</div>`
  }

  const rowOpen =
    options.format === 'pdf' ? '<tr><td class="pdf-document">' : ''
  const rowClose = options.format === 'pdf' ? '</td></tr>' : ''
  let renderedStressSection = stressSection
  let renderedCacheSection = cacheSection
  let renderedErrorSection = input.errorMessage
    ? `<div class="section page-break-avoid"><h2>${escapeHtml(t('Error'))}</h2><p style="color:#dc2626">${escapeHtml(input.errorMessage)}</p></div>`
    : ''
  if (options.format === 'pdf') {
    if (renderedStressSection) {
      renderedStressSection = pdfRow(renderedStressSection)
    }
    if (renderedCacheSection) {
      renderedCacheSection = pdfRow(renderedCacheSection)
    }
    if (renderedErrorSection) {
      renderedErrorSection = pdfRow(renderedErrorSection)
    }
  }

  return `<!doctype html>
<html lang="${escapeHtml(input.language || 'en')}">
<head>
<meta charset="utf-8"/>
<title>${escapeHtml(reportName)}${options.format === 'pdf' ? '' : `-${stampFileName()}`}</title>
<style>
${options.format === 'pdf' ? '' : '@page { size: A4; margin: 0; }'}
@media print {
  html, body {
    margin: 0 !important;
    padding: ${options.format === 'pdf' ? '0' : '12mm 16mm'} !important;
    -webkit-print-color-adjust: exact;
    print-color-adjust: exact;
  }
  .page-break-avoid {
    break-inside: auto;
    page-break-inside: auto;
  }
  .section-header, .check-group-title { break-after: avoid-page; }
}
* {
  box-sizing: border-box;
}
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
  color: #1e293b;
  line-height: 1.5;
  background: #fff;
  margin: 0 auto;
  padding: 12mm 16mm;
  max-width: 960px;
  font-size: 11.5px;
}

/* Header */
.report-header {
  border-bottom: 2px solid #2563eb;
  padding-bottom: 8px;
  margin-bottom: 10px;
}
.report-title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
}
.report-title {
  font-size: 19px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
}
.report-subtitle {
  font-size: 11px;
  color: #64748b;
  margin-top: 2px;
}
.report-badge-top {
  display: inline-flex;
  align-items: center;
  background: #eff6ff;
  border: 1px solid #bfdbfe;
  color: #1d4ed8;
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 4px;
}

/* Meta Grid */
.meta-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px 16px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 5px;
  padding: 6px 12px;
  margin-bottom: 12px;
}
.meta-item {
  font-size: 11px;
  display: flex;
}
.meta-label {
  color: #64748b;
  font-weight: 500;
  width: 80px;
  flex-shrink: 0;
}
.meta-value {
  color: #0f172a;
  font-weight: 600;
  word-break: break-all;
}

/* Section */
.section {
  margin-bottom: 14px;
}
.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  padding-bottom: 4px;
  border-bottom: 1px solid #e2e8f0;
}
.section-title {
  font-size: 13.5px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}
.section-title::before {
  content: "";
  display: inline-block;
  width: 3.5px;
  height: 13px;
  background: #2563eb;
  border-radius: 2px;
}

/* Verdict Badges */
.verdict-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}
.verdict-ok {
  background: #dcfce7;
  color: #15803d;
  border: 1px solid #86efac;
}
.verdict-slow {
  background: #fef9c3;
  color: #a16207;
  border: 1px solid #fde047;
}
.verdict-abnormal {
  background: #fee2e2;
  color: #b91c1c;
  border: 1px solid #fca5a5;
}
.verdict-na {
  background: #f1f5f9;
  color: #475569;
  border: 1px solid #cbd5e1;
}

/* Detailed Check List */
.check-group-title {
  font-weight: 600;
  font-size: 11.5px;
  color: #334155;
  margin: 6px 0 3px;
}
.check-list {
  list-style: none;
  padding: 0;
  margin: 0 0 6px 0;
  display: flex;
  flex-direction: column;
  gap: 2.5px;
}
.check-list-item {
  font-size: 11px;
  line-height: 1.4;
  color: #334155;
  display: flex;
  align-items: baseline;
  gap: 5px;
}
.check-bullet {
  color: #3b82f6;
  font-size: 12px;
  line-height: 1;
}
.check-name {
  font-weight: 600;
  color: #1e293b;
  flex-shrink: 0;
}
.check-status {
  font-weight: 700;
}
.check-status-pass {
  color: #15803d;
  flex-shrink: 0;
}
.check-status-skip {
  color: #a16207;
  flex-shrink: 0;
}
.check-status-fail {
  color: #dc2626;
  flex-shrink: 0;
}
.check-detail, .check-reason {
  color: #475569;
}
.check-reason strong {
  color: #a16207;
}

/* Config Grid */
.config-bar {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 5px;
  padding: 5px 8px;
  margin-bottom: 5px;
  font-size: 11px;
}
.config-item {
  color: #475569;
}
.config-item span {
  color: #0f172a;
  font-weight: 600;
}

/* Stat Cards */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 5px;
  margin-bottom: 5px;
}
.stat-card {
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 5px;
  padding: 4px 6px;
  text-align: center;
}
.stat-val {
  font-size: 12.5px;
  font-weight: 700;
  color: #0f172a;
  line-height: 1.2;
}
.stat-lbl {
  font-size: 10px;
  color: #64748b;
  margin-top: 1px;
}

/* Table */
table.benchmark-table {
  width: 100%;
  border-collapse: collapse;
  margin: 5px 0;
  font-size: 11px;
}
table.benchmark-table th, table.benchmark-table td {
  border: 1px solid #e2e8f0;
  padding: 4px 7px;
  text-align: left;
}
table.benchmark-table th {
  background: #f8fafc;
  font-weight: 600;
  color: #475569;
}
table.benchmark-table td.cell-measured {
  font-weight: 600;
  color: #0f172a;
}
table.benchmark-table td.cell-threshold {
  color: #64748b;
}

.summary-text {
  font-size: 10.5px;
  color: #64748b;
  background: #f8fafc;
  border-left: 3px solid #cbd5e1;
  padding: 4px 8px;
  margin-top: 5px;
  border-radius: 0 4px 4px 0;
}
.summary-text strong {
  color: #334155;
}
.summary-text ul {
  margin: 3px 0 0;
  padding-left: 17px;
}
.summary-text li {
  margin: 2px 0;
  overflow-wrap: anywhere;
}
thead {
  display: table-header-group;
}
tr {
  break-inside: avoid;
  page-break-inside: avoid;
}
.report-footer {
  border-top: 1px solid #e2e8f0;
  color: #94a3b8;
  font-size: 9px;
  margin-top: 16px;
  padding-top: 5px;
  text-align: center;
}
${options.format === 'pdf' ? pdfPageFooterStyles(input.branding, reportName) + pdfDocumentStyles : ''}
</style>
</head>
<body${options.format === 'pdf' ? ' class="pdf-document"' : ''}>
${options.format === 'pdf' ? `<table class="pdf-shell"><thead><tr><td>${pdfLetterhead(input.branding, pdfSubtitle, t('Operating entity'))}</td></tr></thead><tbody>` : ''}

${rowOpen}
<div class="report-header">
  <div class="report-title-row">
    <div>
      <h1 class="report-title">${escapeHtml(reportName)}</h1>
      <div class="report-subtitle">Supplier Benchmark &amp; SLA Compliance Test Report</div>
    </div>
    <div class="report-badge-top">${escapeHtml(t('Commercial evaluation'))}</div>
  </div>
</div>
${rowClose}

${rowOpen}
<div class="meta-grid">
  <div class="meta-item"><span class="meta-label">${escapeHtml(t('Model'))}：</span><span class="meta-value">${escapeHtml(input.model || '-')}</span></div>
  <div class="meta-item"><span class="meta-label">${escapeHtml(t('Base URL'))}：</span><span class="meta-value">${escapeHtml(input.baseUrl || '-')}</span></div>
  <div class="meta-item"><span class="meta-label">${escapeHtml(t('Judgment standard'))}：</span><span class="meta-value">${escapeHtml(input.standardLabel)}</span></div>
  ${options.format === 'pdf' ? '' : `<div class="meta-item"><span class="meta-label">${escapeHtml(t('Time'))}：</span><span class="meta-value">${escapeHtml(new Date().toLocaleString())}</span></div>`}
</div>
${rowClose}

${options.format === 'pdf' ? pdfRow(pdfSummary) : ''}

<!-- 模块一：接口连通与协议兼容 -->
${rowOpen}
<div class="section page-break-avoid">
  <div class="section-header">
    <div class="section-title">1. ${escapeHtml(t('Connectivity and protocol'))}</div>
    <div class="verdict-badge ${basicBadgeClass}">${escapeHtml(t(basicBadge))}</div>
  </div>
  ${shallowChecks.length > 0 ? `<div class="check-group-title">1. ${escapeHtml(t('Basic connectivity checks'))}</div>${renderCheckListHtml(shallowChecks, input, t)}` : ''}
  ${protocolChecks.length > 0 ? `<div class="check-group-title">2. ${escapeHtml(t('Advanced protocol capabilities'))}</div>${renderCheckListHtml(protocolChecks, input, t)}` : ''}
  ${input.summaries.basic ? `<div class="summary-text">${escapeHtml(input.summaries.basic)}</div>` : ''}
</div>
${rowClose}

${renderedStressSection}
${renderedCacheSection}

${renderedErrorSection}

${options.format === 'pdf' ? '' : `<div class="report-footer">${escapeHtml(t('Generated by Supplier Test'))} · ${escapeHtml(new Date().toLocaleString())}</div>`}
${options.format === 'pdf' ? '</tbody></table>' : ''}

</body>
</html>
`
}

export type VideoReportInput = {
  runReport: VideoRunReport
  language?: string
  branding?: ReportBranding
  t: ReportInput['t']
}

export function buildVideoMarkdownReport(input: VideoReportInput): string {
  return buildVideoRunMarkdownReport(input.runReport, input)
}

export function buildVideoHtmlReport(
  input: VideoReportInput,
  options: { format?: 'html' | 'pdf' } = {}
): string {
  return buildVideoRunHtmlReport(input.runReport, input, options)
}

export async function exportVideoPdfReport(
  input: VideoReportInput
): Promise<void> {
  const branding = await preparePdfBranding(input.branding)
  const html = buildVideoHtmlReport({ ...input, branding }, { format: 'pdf' })
  await printHtmlReport(html, 'doubao-video-report-print')
}
