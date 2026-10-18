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
import { DEFAULT_LOGO, DEFAULT_SYSTEM_NAME } from '@/lib/constants'

import type { ReportBranding } from './report'

export async function printHtmlReport(
  html: string,
  title: string
): Promise<void> {
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

export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
}

export function pdfLetterhead(
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

export function pdfPageFooterStyles(
  branding: ReportBranding | undefined,
  title: string
): string {
  const platformName = branding?.platformName || DEFAULT_SYSTEM_NAME
  const operatingName = branding?.operatingEntityName || ''
  const footer = `${platformName}${operatingName ? ` · ${operatingName}` : ''} · ${title}`
  const safeFooter = footer
    .replaceAll('\\', '\\\\')
    .replaceAll('"', '\\"')
    .replaceAll('<', '\\3c ')
    .replaceAll('>', '\\3e ')
    .replaceAll('\r', ' ')
    .replaceAll('\n', ' ')
  return `@page {
  size:A4; margin:33pt 40pt 58pt;
  @bottom-left { content:"${safeFooter}"; color:#637184; font:7.5pt "Noto Sans SC","PingFang SC",sans-serif; border-top:0.5pt solid #dde4ed; vertical-align:top; padding-top:5pt; }
  @bottom-center { content:""; border-top:0.5pt solid #dde4ed; }
  @bottom-right { content:counter(page, decimal-leading-zero) " / " counter(pages, decimal-leading-zero); color:#637184; font:8pt "Noto Sans SC","PingFang SC",sans-serif; border-top:0.5pt solid #dde4ed; vertical-align:top; padding-top:5pt; }
}`
}

export function pdfRow(content: string): string {
  return `<tr><td class="pdf-document">${content}</td></tr>`
}

export function pdfSummaryCards(
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

export const pdfDocumentStyles = `
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
