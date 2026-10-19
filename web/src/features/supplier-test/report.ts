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
import { DEFAULT_LOGO, DEFAULT_SYSTEM_NAME } from '@/lib/constants'

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
import type {
  VideoTestRecord,
  VideoExchange,
  CacheMetrics,
  CheckResult,
  CheckStatus,
  StressMetrics,
} from './types'

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

async function printHtmlReport(html: string, title: string): Promise<void> {
  const iframe = document.createElement('iframe')
  iframe.style.position = 'fixed'
  iframe.style.right = '0'
  iframe.style.bottom = '0'
  iframe.style.width = '0'
  iframe.style.height = '0'
  iframe.style.border = '0'
  iframe.title = title
  document.body.appendChild(iframe)

  const doc = iframe.contentWindow?.document
  if (!doc) {
    document.body.removeChild(iframe)
    throw new Error('Unable to prepare PDF report')
  }

  doc.open()
  doc.write(html)
  doc.close()

  let timeoutId: ReturnType<typeof setTimeout> | undefined
  try {
    const images = Array.from(
      doc.images,
      (img) =>
        new Promise<void>((resolve, reject) => {
          if (img.complete) {
            if (img.naturalWidth > 0) resolve()
            else reject(new Error('Unable to load PDF branding image'))
            return
          }
          img.addEventListener('load', () => resolve(), { once: true })
          img.addEventListener(
            'error',
            () => reject(new Error('Unable to load PDF branding image')),
            { once: true }
          )
        })
    )
    await Promise.race([
      Promise.all([doc.fonts?.ready, ...images]),
      new Promise<never>((_, reject) => {
        timeoutId = setTimeout(
          () => reject(new Error('PDF branding image load timed out')),
          10000
        )
      }),
    ])
    const cleanup = () => {
      if (iframe.isConnected) document.body.removeChild(iframe)
    }
    iframe.contentWindow?.addEventListener('afterprint', cleanup, {
      once: true,
    })
    iframe.contentWindow?.focus()
    iframe.contentWindow?.print()
    setTimeout(cleanup, 30000)
  } catch (error) {
    if (iframe.isConnected) document.body.removeChild(iframe)
    throw error
  } finally {
    if (timeoutId) clearTimeout(timeoutId)
  }
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

function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

function pdfLetterhead(
  branding: ReportBranding | undefined,
  title: string,
  operatorLabel: string
): string {
  const platformName = branding?.platformName || DEFAULT_SYSTEM_NAME
  const platformLogo = branding?.platformLogo || DEFAULT_LOGO
  const operatingName = branding?.operatingEntityName || ''
  const operatingLogo = branding?.operatingEntityLogo || ''
  let operator = ''
  if (operatingLogo) {
    const logoSource = operatingLogo.startsWith('data:image/png;base64,')
      ? operatingLogo
      : '/api/pdf-branding/operating-logo'
    operator = `<div class="document-operator"><img src="${escapeHtml(logoSource)}" alt="${escapeHtml(operatingName || operatorLabel)}"/></div>`
  } else if (operatingName) {
    operator = `<div class="document-operator"><span>${escapeHtml(operatingName)}</span></div>`
  }
  return `<div class="document-letterhead">
  <div class="document-platform"><img src="${escapeHtml(platformLogo)}" alt=""/><div><strong>${escapeHtml(platformName)}</strong><span>${escapeHtml(title)}</span></div></div>
  ${operator}
</div>`
}

function cssContent(value: string): string {
  const escaped = value
    .replaceAll('\\', '\\\\')
    .replaceAll('"', '\\"')
    .replaceAll('<', '\\3c ')
    .replaceAll('>', '\\3e ')
    .replaceAll('\r', ' ')
    .replaceAll('\n', ' ')
  return `"${escaped}"`
}

function pdfPageFooterStyles(
  branding: ReportBranding | undefined,
  title: string
): string {
  const platformName = branding?.platformName || DEFAULT_SYSTEM_NAME
  const operatingName = branding?.operatingEntityName || ''
  const footer = `${platformName}${operatingName ? ` · ${operatingName}` : ''} · ${title}`
  return `@page {
  size:A4; margin:33pt 40pt 58pt;
  @bottom-left { content:${cssContent(footer)}; color:#637184; font:7.5pt "Noto Sans SC","PingFang SC",sans-serif; border-top:0.5pt solid #dde4ed; vertical-align:top; padding-top:5pt; }
  @bottom-center { content:""; border-top:0.5pt solid #dde4ed; }
  @bottom-right { content:counter(page, decimal-leading-zero) " / " counter(pages, decimal-leading-zero); color:#637184; font:8pt "Noto Sans SC","PingFang SC",sans-serif; border-top:0.5pt solid #dde4ed; vertical-align:top; padding-top:5pt; }
}`
}

function pdfRow(content: string): string {
  return `<tr><td class="pdf-document">${content}</td></tr>`
}

function pdfSummaryCards(
  cards: Array<{
    label: string
    value: string
    detail: string
    valueClass?: string
  }>
): string {
  return `<div class="document-summary">${cards
    .map(
      (card) => `<div class="document-summary-card">
  <div class="document-summary-label">${escapeHtml(card.label)}</div>
  <div class="document-summary-value${card.valueClass ? ` ${card.valueClass}` : ''}">${escapeHtml(card.value)}</div>
  <div class="document-summary-detail">${escapeHtml(card.detail)}</div>
</div>`
    )
    .join('')}</div>`
}

const pdfDocumentStyles = `
html, body.pdf-document { margin:0 !important; padding:0 !important; }
body.pdf-document { width:auto; max-width:none; min-height:0; display:block; font-family:"Noto Sans SC","PingFang SC","Microsoft YaHei",sans-serif; font-size:8.5pt; line-height:1.45; color:#1c2b40; }
.pdf-shell { width:515pt; border-collapse:collapse; table-layout:fixed; }
.pdf-shell thead { display:table-header-group; }
.pdf-shell tbody { display:table-row-group; }
.pdf-shell tr, .pdf-shell td { break-inside:auto; page-break-inside:auto; }
.pdf-shell tbody tr { break-inside:avoid-page; }
.pdf-shell > thead > tr > td, .pdf-shell > tbody > tr > td { padding:0; border:0; vertical-align:top; }
.pdf-shell > thead > tr > td { height:86pt; }
.document-letterhead { display:flex; width:515pt; height:67pt; padding-top:9pt; box-sizing:border-box; align-items:flex-start; justify-content:space-between; border-bottom:1.5pt solid #27477a; }
.document-platform { display:flex; gap:12pt; align-items:center; max-width:335pt; }
.document-platform img { width:38pt; height:38pt; object-fit:contain; }
.document-platform strong { display:block; color:#19335e; font-size:15pt; font-weight:400; line-height:19pt; white-space:nowrap; }
.document-platform span { display:block; margin-top:1pt; color:#5a6a81; font-size:7.5pt; line-height:11pt; }
.document-operator { display:flex; width:125pt; height:40pt; align-items:center; justify-content:flex-end; color:#19335e; white-space:nowrap; }
.document-operator img { width:125pt; height:40pt; object-fit:contain; object-position:right center; }
.document-operator span { font-size:10pt; line-height:14pt; text-align:right; }
.pdf-document .report-header { border:0; padding:2pt 0 0; margin:0 0 20pt; }
.pdf-document .report-title-row { align-items:flex-end; }
.pdf-document .report-title-row > div:first-child { display:flex; flex-direction:column-reverse; }
.pdf-document .report-subtitle { font-size:8.5pt; line-height:11pt; color:#59697f; margin:0 0 5pt; }
.pdf-document .report-title { font-size:26pt; line-height:31pt; font-weight:400; color:#19335e; }
.pdf-document .report-badge-top { max-width:150pt; padding:0; border:0; border-radius:0; background:none; color:#19335e; font-size:16pt; font-weight:400; text-align:right; text-transform:none; }
.pdf-document > .meta-grid { display:flex; flex-direction:column; gap:6pt; padding:0; margin:0 0 24pt; border:0; border-radius:0; background:none; }
.pdf-document > .meta-grid .meta-item { font-size:8.5pt; line-height:12pt; }
.pdf-document > .meta-grid .meta-label { width:auto; margin-right:6pt; font-weight:400; color:#58687e; }
.pdf-document > .meta-grid .meta-value { color:#1c2b40; font-weight:400; }
.document-summary { display:grid; grid-template-columns:repeat(3,minmax(0,1fr)); gap:13pt; margin:0 0 26pt; }
.document-summary-card { height:79pt; padding:11pt 12pt; background:#f1f5fb; color:#19335e; }
.document-summary-card:last-child { background:#19335e; color:#fff; }
.document-summary-label { font-size:8pt; line-height:11pt; color:#536681; }
.document-summary-card:last-child .document-summary-label { color:#dce7f7; }
.document-summary-value { margin-top:8pt; font-size:16pt; line-height:21pt; text-align:right; overflow-wrap:anywhere; }
.document-summary-value.result-pass { color:#86efac; font-weight:700; }
.document-summary-value.result-fail { color:#fecaca; font-weight:700; }
.document-summary-detail { margin-top:3pt; font-size:8pt; text-align:right; }
.pdf-document .section { margin:0 0 16pt; padding:0; border:0; border-radius:0; background:none; }
.pdf-document .section-header { height:26pt; padding:0 10pt; margin:0 0 6pt; border:0; background:#19335e; break-after:avoid-page; }
.pdf-document .section-title { font-size:9pt; font-weight:400; line-height:1.3; color:#fff; }
.pdf-document .section-title::before { display:none; }
.pdf-document .verdict-badge { padding:0; border:0; background:none; color:#fff; font-size:8pt; font-weight:400; }
.pdf-document .check-group-title { margin:6pt 0 3pt; color:#59697f; font-size:8.5pt; font-weight:400; break-after:avoid-page; }
.pdf-document .check-list { display:block; margin:0 0 6pt; }
.pdf-document .check-list-item { min-height:15pt; padding:2.5pt 10pt; gap:5pt; font-size:8.2pt; line-height:10pt; break-inside:avoid; }
.pdf-document .check-list-item:nth-child(odd), .pdf-document .check-item:nth-child(odd) { background:#f5f7fb; }
.pdf-document .check-bullet { display:none; }
.pdf-document .checks-list .check-item > span:first-child { display:none; }
.pdf-document .check-name { flex:1; font-weight:400; }
.pdf-document .check-status-pass { color:#15803d; }
.pdf-document .check-status-fail { color:#b91c1c; }
.pdf-document .check-status-skip { color:#a16207; }
.pdf-document .check-detail, .pdf-document .check-reason { color:#59697f; }
.pdf-document .config-bar { grid-template-columns:repeat(2,minmax(0,1fr)); gap:4pt 12pt; margin:0 0 7pt; padding:5pt 0; border:0; border-radius:0; background:none; font-size:8.2pt; }
.pdf-document .config-item { color:#59697f; }
.pdf-document .config-item span { color:#233145; font-weight:400; }
.pdf-document .stats-grid { grid-template-columns:repeat(2,minmax(0,1fr)); gap:0; margin:0 0 6pt; }
.pdf-document .stat-card { min-height:15pt; padding:2.5pt 10pt; border:0; border-radius:0; background:none; display:flex; flex-direction:row-reverse; justify-content:space-between; align-items:baseline; text-align:left; }
.pdf-document .stat-card:nth-child(4n + 1), .pdf-document .stat-card:nth-child(4n + 2) { background:#f5f7fb; }
.pdf-document .stat-val { font-size:8.2pt; font-weight:400; line-height:10pt; color:#233145; }
.pdf-document .stat-lbl { font-size:8pt; line-height:10pt; color:#59697f; margin:0; }
.pdf-document table.benchmark-table { margin:6pt 0; font-size:8.2pt; }
.pdf-document table.benchmark-table th { padding:6pt 10pt; border:0; background:#19335e; color:#fff; font-size:9pt; font-weight:400; }
.pdf-document table.benchmark-table td { padding:3pt 10pt; border:0; color:#233145; }
.pdf-document table.benchmark-table tbody tr:nth-child(odd) { background:#f5f7fb; }
.pdf-document .summary-text { margin:5pt 0 0; padding:2pt 0; border:0; border-radius:0; background:none; color:#59697f; font-size:8pt; }
.pdf-document .summary-text strong { color:#233145; }
.pdf-document .prompt-box { padding:8pt 10pt; border:0; border-radius:0; background:#f5f7fb; color:#233145; font-size:8.5pt; }
.pdf-document .check-item { min-height:15pt; padding:2.5pt 10pt; border:0; font-size:8.2pt; }
.pdf-document .check-title, .pdf-document .check-status-text { font-size:8.2pt; font-weight:400; color:#233145; }
.pdf-document .check-msg { font-size:8pt; color:#59697f; }
.pdf-document .section > .meta-grid { margin:0; padding:0; border:0; background:none; font-size:8.2pt; }
.pdf-document .section .meta-item { font-size:8.2pt; }
.pdf-document .section .meta-label { width:auto; margin-right:5pt; }
.pdf-document .section .meta-value { font-weight:400; }
.pdf-document .output-box { padding:10pt 12pt; border:0; border-radius:0; background:#f1f5fb; }
.pdf-document .output-title, .pdf-document .output-url, .pdf-document .output-url a { color:#19335e; font-size:8.5pt; }
.pdf-document .error-box { padding:10pt 12pt; border:0; border-radius:0; }
@media print {
  .pdf-document .page-break-avoid { break-inside:auto; }
  .pdf-document .section-header, .pdf-document .report-header, .document-summary { break-inside:avoid; }
}
/* Reassert row fragmentation after the print rules so Chromium repeats thead. */
.pdf-shell tr, .pdf-shell td { break-inside:auto; page-break-inside:auto; }
.pdf-shell tbody tr { break-inside:avoid-page; }
`

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
  language?: string
  branding?: ReportBranding
  records: VideoTestRecord[]
  v3?: boolean
  materialLibrary?: boolean
  t: ReportInput['t']
}

export function videoResult(record: VideoTestRecord): {
  model: string
  resolution: string
  generation: string
  origin: string
  comparison: string
} {
  let request: Record<string, unknown> = {},
    result: Record<string, unknown> = {}
  try {
    request = JSON.parse(record.metrics.raw_request_json || '{}')
  } catch {
    /* unavailable */
  }
  try {
    result =
      JSON.parse(record.verification?.final?.response || '{}').Result || {}
  } catch {
    /* unavailable */
  }
  const model = String(request.model || record.model || '—')
  const detected = String(result.IsOfficial).toLowerCase()
  const succeeded = ['success', 'succeeded'].includes(
    String(result.Status).toLowerCase()
  )
  let origin = 'Not verified'
  if (record.verification?.attempted) origin = 'Incomplete'
  if (
    record.verification?.last?.error ||
    (record.verification?.final && !succeeded)
  ) {
    origin = 'Failed'
  }
  if (succeeded) {
    origin = 'Cannot confirm'
    if (detected === 'true') origin = 'Matched'
    else if (detected === 'false') origin = 'No features detected'
  }
  let comparison = 'Cannot confirm'
  if (
    succeeded &&
    detected === 'true' &&
    result.ModelName &&
    /^doubao-seedance-[a-z0-9-]+-\d{6}$/.test(model) &&
    /^doubao-seedance-[a-z0-9-]+-\d{6}$/.test(String(result.ModelName))
  ) {
    comparison = result.ModelName === model ? 'Matched' : 'Mismatch'
  }
  if (
    result.ResourceType &&
    String(result.ResourceType).toLowerCase() !== 'video'
  ) {
    origin = 'Failed'
    comparison = 'Cannot confirm'
  }
  return {
    model,
    resolution: String(request.resolution || '—'),
    generation: record.outcome,
    origin,
    comparison,
  }
}

export function videoExchanges(
  record: VideoTestRecord
): Array<[string, VideoExchange | undefined]> {
  return [
    ...record.materials.map((exchange): [string, VideoExchange] => [
      'Material preparation',
      exchange,
    ]),
    ['Video task submission', record.metrics.submit],
    ['First queued response', record.metrics.queued],
    ['First running response', record.metrics.running],
    ['Final video result', record.metrics.final],
    ...(!record.metrics.final && record.metrics.last_query
      ? [
          ['Latest query (not final)', record.metrics.last_query] as [
            string,
            VideoExchange,
          ],
        ]
      : []),
    ['Official verification submission', record.verification?.create],
    ['Final official verification result', record.verification?.final],
    ...(!record.verification?.final && record.verification?.last
      ? [
          [
            'Latest verification query (not final)',
            record.verification.last,
          ] as [string, VideoExchange],
        ]
      : []),
  ]
}

function videoStageNote(record: VideoTestRecord, label: string): string {
  switch (label) {
    case 'Material preparation':
      return 'Material requests show the images prepared for this test in their original order.'
    case 'Video task submission':
      return record.metrics.submit
        ? 'The submission receipt shows whether a Task ID was accepted.'
        : 'Submission evidence unavailable'
    case 'First queued response':
    case 'First running response':
      return 'Each progress state is recorded once; repeated polls are summarized.'
    case 'Final video result':
      return record.metrics.final
        ? 'The terminal response determines generation success and provides the original video URL.'
        : 'No final response obtained'
    case 'Official verification submission':
      if (!record.verification?.attempted) return 'Not executed'
      return record.verification.create
        ? 'The original video URL is submitted to Volcengine; QueryID identifies the verification.'
        : 'No complete response received'
    case 'Final official verification result':
      return record.verification?.final
        ? 'IsOfficial indicates source detection; ModelName is compared only for a canonical requested model.'
        : 'No final verification response obtained'
    default:
      return 'No final response obtained'
  }
}

export function videoReportURL(value: string): string {
  try {
    const url = new URL(value)
    if (url.username || url.password) {
      url.username = '[REDACTED]'
      url.password = ''
    }
    for (const key of url.searchParams.keys()) {
      if (/signature|credential|token|access.?key|^sig$|^api_key$/i.test(key)) {
        url.searchParams.set(key, '[REDACTED]')
      }
    }
    return url.toString()
  } catch {
    return value
  }
}

// Redact JSON string tokens without parsing/re-encoding numeric values. The
// attachment may contain nested JSON strings; binary input stays in that file.
export function videoEvidenceText(raw: string): string {
  return raw.replaceAll(
    /("(?:\\.|[^"\\])*"\s*:\s*)?("(?:\\.|[^"\\])*")/g,
    (match, prefix: string | undefined, token: string, offset: number) => {
      if (
        !prefix &&
        raw
          .slice(offset + match.length)
          .trimStart()
          .startsWith(':')
      ) {
        return match
      }
      const value = JSON.parse(token) as string
      const key = prefix
        ? (JSON.parse(prefix.replace(/:\s*$/, '')) as string)
        : ''
      let replacement = value
      if (
        /authorization|cookie|secret|^api[_-]?key$|^access[_-]?key[_-]?id$|^token$|^password$/i.test(
          key
        )
      ) {
        replacement = '[REDACTED]'
      } else if (/^https?:\/\//i.test(value)) {
        replacement = videoReportURL(value)
      } else if (value.startsWith('{') || value.startsWith('[')) {
        replacement = videoEvidenceText(value)
      }
      return replacement === value
        ? match
        : (prefix || '') + JSON.stringify(replacement)
    }
  )
}

export function buildVideoMarkdownReport(input: VideoReportInput): string {
  const t = input.t
  const lines = [
    `# ${t('Doubao Video Generation Test Report')}`,
    input.branding?.operatingEntityName || '',
    `- ${t('Supplier has implemented the v3 API')}: ${t(input.v3 ? 'Yes' : 'No')}`,
    `- ${t('Supplier has implemented a material library')}: ${t(input.materialLibrary ? 'Yes' : 'No')}`,
    '',
    `| ${['Model', 'Request resolution', 'Video generation', 'Official origin features', 'Model comparison'].map((key) => t(key)).join(' | ')} |`,
    '| --- | --- | --- | --- | --- |',
  ]
  for (const record of input.records) {
    const result = videoResult(record)
    lines.push(
      `| ${[result.model, result.resolution, t(result.generation), t(result.origin), t(result.comparison)].map((value) => value.replaceAll('|', '\\|')).join(' | ')} |`
    )
  }
  for (const model of new Set(
    input.records.map((record) => videoResult(record).model)
  )) {
    lines.push('', `## ${model}`)
    for (const record of input.records.filter(
      (record) => videoResult(record).model === model
    )) {
      lines.push(
        '',
        `### ${t('Request resolution')}: ${videoResult(record).resolution} · ${new Date(record.startedAt).toLocaleString(input.language)}`,
        `${t('Base URL')}: ${videoReportURL(record.baseUrl)} · Task ID: ${record.metrics.task_id || '—'} · QueryID: ${record.verification?.queryId || '—'}`,
        `${t('Polling summary')}: ${record.metrics.poll_count || 0} / ${record.metrics.poll_ms || 0} ms · ${t('Official verification')}: ${record.verification?.count || 0} / ${record.verification?.elapsed || 0} ms`
      )
      for (const [title, exchange] of videoExchanges(record)) {
        if (
          title === 'Video task submission' ||
          title === 'Official verification submission'
        ) {
          lines.push(
            '',
            `#### ${t(title === 'Video task submission' ? 'Video generation' : 'Official verification')}`
          )
        }
        lines.push('', `##### ${t(title)}`, t(videoStageNote(record, title)))
        if (!exchange) {
          lines.push(t('Phase unavailable'))
          continue
        }
        lines.push(
          `${exchange.method} ${videoReportURL(exchange.url)} · HTTP ${exchange.status || '—'} · ${exchange.elapsed_ms} ms · Request ID: ${exchange.request_id || '—'}`
        )
        const bodies = [['Response body', exchange.response, 'json']]
        if (
          title !== 'First queued response' &&
          title !== 'First running response'
        ) {
          bodies.unshift(
            ['Request headers', exchange.headers, 'http'],
            ['Request body', exchange.request, 'json']
          )
        }
        for (const [label, body, language] of bodies) {
          const value =
            videoEvidenceText(body) ||
            t(
              label === 'Request body'
                ? 'No request body'
                : 'No complete response received'
            )
          const fence = '`'.repeat(
            Math.max(
              3,
              ...(value.match(/`+/g) || []).map((item) => item.length + 1)
            )
          )
          lines.push(`**${t(label)}**`, fence + language, value, fence)
        }
        if (exchange.error) lines.push(t(exchange.error))
      }
      if (record.metrics.video_url) {
        lines.push(
          `${t('Generated Video Output')}: ${videoReportURL(record.metrics.video_url)}`
        )
      }
      if (!record.metrics.final) lines.push(t('No final response obtained'))
      if (record.issue || record.verification?.issue) {
        lines.push(record.issue || '', t(record.verification?.issue || ''))
      }
    }
  }
  return `${lines.join('\n')}\n`
}

export function buildVideoHtmlReport(
  input: VideoReportInput,
  options: { format?: 'html' | 'pdf' } = {}
): string {
  const t = input.t,
    title = t('Doubao Video Generation Test Report')
  const operatorLogo = input.branding?.operatingEntityLogo || ''
  const headerLogo = operatorLogo.startsWith('data:image/png;base64,')
    ? operatorLogo
    : '/api/pdf-branding/operating-logo'
  const printHeader = `@media print{.video-letterhead{display:none}@page{margin-top:119pt;@top-left{content:${cssContent(input.branding?.platformName || DEFAULT_SYSTEM_NAME)};background-image:url(${cssContent(input.branding?.platformLogo || DEFAULT_LOGO)});background-size:38pt 38pt;background-repeat:no-repeat;background-position:left center;padding-left:50pt;width:335pt;color:#19335e;font:15pt "PingFang SC",sans-serif;border-bottom:1.5pt solid #27477a;margin:25pt 0 19pt;vertical-align:middle}@top-right{content:${cssContent(operatorLogo ? '' : input.branding?.operatingEntityName || '')};${operatorLogo ? `background-image:url(${cssContent(headerLogo)});background-size:125pt 40pt;background-repeat:no-repeat;background-position:right center;` : ''}width:130pt;color:#19335e;font:10pt "PingFang SC",sans-serif;text-align:right;border-bottom:1.5pt solid #27477a;margin:25pt 0 19pt;vertical-align:middle}}}`
  const rows: string[] = [
    `<h1>${escapeHtml(title)}</h1><p>${escapeHtml(t('Supplier has implemented the v3 API'))}: ${escapeHtml(t(input.v3 ? 'Yes' : 'No'))} · ${escapeHtml(t('Supplier has implemented a material library'))}: ${escapeHtml(t(input.materialLibrary ? 'Yes' : 'No'))}</p>`,
  ]
  rows.push(
    `<table class="benchmark-table"><thead><tr>${['Model', 'Request resolution', 'Video generation', 'Official origin features', 'Model comparison'].map((key) => `<th>${escapeHtml(t(key))}</th>`).join('')}</tr></thead><tbody>${input.records
      .map((record) => {
        const result = videoResult(record)
        return `<tr>${[result.model, result.resolution, t(result.generation), t(result.origin), t(result.comparison)].map((value) => `<td>${escapeHtml(value)}</td>`).join('')}</tr>`
      })
      .join('')}</tbody></table>`
  )
  for (const model of new Set(
    input.records.map((record) => videoResult(record).model)
  )) {
    rows.push(`<h2>${escapeHtml(model)}</h2>`)
    for (const record of input.records.filter(
      (record) => videoResult(record).model === model
    )) {
      rows.push(
        `<h3>${escapeHtml(t('Request resolution'))}: ${escapeHtml(videoResult(record).resolution)} · ${escapeHtml(new Date(record.startedAt).toLocaleString(input.language))}</h3><p>${escapeHtml(videoReportURL(record.baseUrl))} · Task ID: ${escapeHtml(record.metrics.task_id || '—')} · QueryID: ${escapeHtml(record.verification?.queryId || '—')}</p><p>${escapeHtml(t('Polling summary'))}: ${record.metrics.poll_count || 0} / ${record.metrics.poll_ms || 0} ms · ${escapeHtml(t('Official verification'))}: ${record.verification?.count || 0} / ${record.verification?.elapsed || 0} ms</p>`
      )
      for (const [label, exchange] of videoExchanges(record)) {
        let html = ''
        if (
          label === 'Video task submission' ||
          label === 'Official verification submission'
        ) {
          html = `<h4>${escapeHtml(t(label === 'Video task submission' ? 'Video generation' : 'Official verification'))}</h4>`
        }
        html += `<h5>${escapeHtml(t(label))}</h5><p>${escapeHtml(t(videoStageNote(record, label)))}</p>`
        if (!exchange) html += `<p>${escapeHtml(t('Phase unavailable'))}</p>`
        else {
          html += `<p>${escapeHtml(exchange.method)} ${escapeHtml(videoReportURL(exchange.url))} · HTTP ${exchange.status || '—'} · ${exchange.elapsed_ms} ms · Request ID: ${escapeHtml(exchange.request_id || '—')}</p>`
          const bodies = [['Response body', exchange.response]]
          if (
            label !== 'First queued response' &&
            label !== 'First running response'
          ) {
            bodies.unshift(
              ['Request headers', exchange.headers],
              ['Request body', exchange.request]
            )
          }
          for (const [key, value] of bodies) {
            html += `<h5>${escapeHtml(t(key))}</h5><pre>${escapeHtml(videoEvidenceText(value) || t(key === 'Request body' ? 'No request body' : 'No complete response received'))}</pre>`
          }
          if (exchange.error) html += `<p>${escapeHtml(t(exchange.error))}</p>`
        }
        rows.push(html)
      }
      rows.push(
        `<p>${escapeHtml(t('Generated Video Output'))}: ${escapeHtml(videoReportURL(record.metrics.video_url || '—'))}</p>${!record.metrics.final ? `<p>${escapeHtml(t('No final response obtained'))}</p>` : ''}<p>${escapeHtml(record.issue || '')} ${escapeHtml(t(record.verification?.issue || ''))}</p>`
      )
    }
  }
  return `<!doctype html><html lang="${escapeHtml(input.language || 'en')}"><head><meta charset="utf-8"><title>${escapeHtml(title)}</title><style>${pdfPageFooterStyles(input.branding, title)}${pdfDocumentStyles}
    .video-body{width:515pt}body{font-family:"Noto Sans SC","PingFang SC",sans-serif;color:#233145}h1,h2,h3,h4{color:#19335e}h1{font-size:22pt}h2{font-size:15pt;margin:15pt 0 5pt}h3{font-size:11pt}h4{font-size:10pt;margin:10pt 0 4pt}h5{font-size:8.5pt;margin:4pt 0 2pt}p{font-size:8pt;overflow-wrap:anywhere;margin:4pt 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;word-break:break-word;font:8.5pt/1.5 monospace;background:#f5f7fb;padding:8pt;margin:0 0 6pt;overflow:visible}h2,h3,h4,h5{break-after:avoid-page}.benchmark-table{width:100%;border-collapse:collapse;table-layout:fixed}.benchmark-table td,.benchmark-table th{overflow-wrap:anywhere;padding:5pt;text-align:left}${printHeader}
    ${options.format === 'pdf' ? '' : 'body{max-width:900px;margin:20px auto}.pdf-shell,.document-letterhead{width:100%}'}
    </style></head><body class="pdf-document"><header class="video-letterhead">${pdfLetterhead(input.branding, title, t('Operating entity'))}</header><main class="video-body">${rows.map((row) => `<section class="pdf-document">${row}</section>`).join('')}</main></body></html>`
}

export async function exportVideoPdfReport(
  input: VideoReportInput
): Promise<void> {
  const branding = await preparePdfBranding(input.branding)
  const html = buildVideoHtmlReport({ ...input, branding }, { format: 'pdf' })
  await printHtmlReport(html, 'doubao-video-report-print')
}
