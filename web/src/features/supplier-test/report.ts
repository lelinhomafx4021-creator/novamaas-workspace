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

import {
  displayMeasured,
  overallLabel,
  VERDICT_LABEL,
  type Assessment,
} from './baselines'
import type { CheckResult, CheckStatus } from './types'

export type ReportInput = {
  baseUrl: string
  model: string
  basicChecks: CheckResult[]
  cacheChecks: CheckResult[]
  summaries: { basic: string; cache: string; stress: string }
  stressAssessment: Assessment | null
  cacheAssessment: Assessment | null
  errorMessage: string
  statusLabel: (status: CheckStatus) => string
  t: (key: string, options?: Record<string, string | number>) => string
}

export function stampFileName(): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
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

function translate(
  t: ReportInput['t'],
  key: string,
  options?: Record<string, string | number>
): string {
  const value = t(key, options)
  return value || key
}

export function buildMarkdownReport(input: ReportInput): string {
  const t: ReportInput['t'] = (key, options) =>
    translate(input.t, key, options)
  const lines = [
    `# ${t('Supplier Test Report')}`,
    '',
    `- ${t('Time')}: ${new Date().toLocaleString()}`,
    `- ${t('Base URL')}: ${input.baseUrl || '-'}`,
    `- ${t('Model')}: ${input.model || '-'}`,
    '',
    `## ${t('Basic acceptance')}`,
  ]
  for (const check of input.basicChecks) {
    const detail = check.message ? ` — ${check.message}` : ''
    lines.push(
      `- ${t(check.title)}: ${t(input.statusLabel(check.status))}${detail}`
    )
  }
  if (input.summaries.basic) {
    lines.push('', input.summaries.basic)
  }

  lines.push('', `## ${t('Cache test')}`)
  for (const check of input.cacheChecks) {
    const detail = check.message ? ` — ${check.message}` : ''
    lines.push(
      `- ${t(check.title)}: ${t(input.statusLabel(check.status))}${detail}`
    )
  }
  if (input.cacheAssessment && input.cacheAssessment.rows.length > 0) {
    lines.push('', `**${t(overallLabel(input.cacheAssessment.overall))}**`, '')
    lines.push(
      `| ${t('Metric')} | ${t('Measured')} | ${t('Threshold')} | ${t('Verdict')} |`
    )
    lines.push('|---|---|---|---|')
    for (const row of input.cacheAssessment.rows) {
      lines.push(
        `| ${t(row.label)} | ${displayMeasured(row, t)} | ${t(row.threshold)} | ${t(VERDICT_LABEL[row.verdict])} |`
      )
    }
  }
  if (input.summaries.cache) {
    lines.push('', input.summaries.cache)
  }

  lines.push('', `## ${t('Stress test')}`)
  if (input.stressAssessment && input.stressAssessment.rows.length > 0) {
    lines.push(`**${t(overallLabel(input.stressAssessment.overall))}**`, '')
    lines.push(
      `| ${t('Metric')} | ${t('Measured')} | ${t('Threshold')} | ${t('Verdict')} |`
    )
    lines.push('|---|---|---|---|')
    for (const row of input.stressAssessment.rows) {
      lines.push(
        `| ${t(row.label)} | ${displayMeasured(row, t)} | ${t(row.threshold)} | ${t(VERDICT_LABEL[row.verdict])} |`
      )
    }
  }
  if (input.summaries.stress) {
    lines.push('', input.summaries.stress)
  }

  if (input.errorMessage) {
    lines.push('', `## ${t('Error')}`, input.errorMessage)
  }
  return `${lines.join('\n')}\n`
}

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

function verdictHtmlColor(verdict: Assessment['overall']): string {
  if (verdict === 'ok') return '#15803d'
  if (verdict === 'slow') return '#a16207'
  return '#6b7280'
}

function htmlRows(
  assessment: Assessment,
  t: ReportInput['t']
): string {
  const body = assessment.rows
    .map((row) => {
      const color = verdictHtmlColor(row.verdict)
      return `<tr>
<td>${escapeHtml(t(row.label))}</td>
<td>${escapeHtml(displayMeasured(row, t))}</td>
<td>${escapeHtml(t(row.threshold))}</td>
<td style="color:${color};font-weight:600">${escapeHtml(t(VERDICT_LABEL[row.verdict]))}</td>
</tr>`
    })
    .join('')
  return `<p><strong>${escapeHtml(t(overallLabel(assessment.overall)))}</strong></p>
<table>
<thead><tr><th>${escapeHtml(t('Metric'))}</th><th>${escapeHtml(t('Measured'))}</th><th>${escapeHtml(t('Threshold'))}</th><th>${escapeHtml(t('Verdict'))}</th></tr></thead>
<tbody>${body}</tbody>
</table>`
}

export function buildHtmlReport(input: ReportInput): string {
  const t: ReportInput['t'] = (key, options) =>
    translate(input.t, key, options)
  const checks = (items: CheckResult[]) =>
    items
      .map((check) => {
        const detail = check.message ? ` — ${escapeHtml(check.message)}` : ''
        return `<li>${escapeHtml(t(check.title))}: ${escapeHtml(t(input.statusLabel(check.status)))}${detail}</li>`
      })
      .join('')

  const cacheTable =
    input.cacheAssessment && input.cacheAssessment.rows.length > 0
      ? htmlRows(input.cacheAssessment, t)
      : ''
  const stressTable =
    input.stressAssessment && input.stressAssessment.rows.length > 0
      ? htmlRows(input.stressAssessment, t)
      : ''

  return `<!doctype html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<title>${escapeHtml(t('Supplier Test Report'))}</title>
<style>
body{font-family:system-ui,sans-serif;max-width:960px;margin:32px auto;padding:0 16px;color:#111;line-height:1.5}
table{border-collapse:collapse;width:100%;margin:12px 0 24px}
th,td{border:1px solid #d1d5db;padding:8px 10px;text-align:left;vertical-align:top}
th{background:#f3f4f6}
.muted{color:#6b7280}
</style>
</head>
<body>
<h1>${escapeHtml(t('Supplier Test Report'))}</h1>
<p>${escapeHtml(t('Time'))}: ${escapeHtml(new Date().toLocaleString())}<br/>
${escapeHtml(t('Base URL'))}: ${escapeHtml(input.baseUrl || '-')}<br/>
${escapeHtml(t('Model'))}: ${escapeHtml(input.model || '-')}</p>
<h2>${escapeHtml(t('Basic acceptance'))}</h2>
<ul>${checks(input.basicChecks)}</ul>
${input.summaries.basic ? `<p class="muted">${escapeHtml(input.summaries.basic)}</p>` : ''}
<h2>${escapeHtml(t('Cache test'))}</h2>
<ul>${checks(input.cacheChecks)}</ul>
${cacheTable}
${input.summaries.cache ? `<p class="muted">${escapeHtml(input.summaries.cache)}</p>` : ''}
<h2>${escapeHtml(t('Stress test'))}</h2>
${stressTable}
${input.summaries.stress ? `<p class="muted">${escapeHtml(input.summaries.stress)}</p>` : ''}
${input.errorMessage ? `<h2>${escapeHtml(t('Error'))}</h2><p>${escapeHtml(input.errorMessage)}</p>` : ''}
</body>
</html>
`
}
