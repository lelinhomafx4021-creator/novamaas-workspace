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
import type { VideoReportInput } from './report'
import {
  escapeHtml,
  pdfDocumentStyles,
  pdfLetterhead,
  pdfPageFooterStyles,
  pdfRow,
  pdfSummaryCards,
} from './report-layout'
import type { StageEvidence, VideoRunReport } from './types'

export const VIDEO_REPORT_CAPABILITIES = [
  { key: 'v3Compatible', label: 'Compatible with v3 API' },
  { key: 'materialLibrary', label: 'Material library available' },
] as const

export function videoStageTitle(id: string): string {
  switch (id) {
    case 'video_submit':
      return 'Video task submission'
    case 'video_queued':
      return 'First queued response'
    case 'video_running':
      return 'First running response'
    case 'video_result':
      return 'Final video result'
    case 'verification_submit':
      return 'Official verification submission'
    case 'verification_result':
      return 'Final official verification result'
    default:
      return 'Stage evidence'
  }
}

export function videoEvidenceStatus(status: string): string {
  switch (status) {
    case 'pass':
      return 'Passed'
    case 'fail':
    case 'failed':
      return 'Failed'
    case 'skip':
      return 'Skipped'
    case 'running':
      return 'Running'
    case 'matched':
      return 'Matched'
    case 'not_detected':
      return 'No features detected'
    case 'mismatch':
      return 'Mismatch'
    case 'completed':
      return 'Completed'
    case 'cancelled':
      return 'Cancelled'
    case 'unknown':
      return 'Cannot compare'
    default:
      return 'Not verified'
  }
}

// Indent the original JSON tokens without re-encoding numbers or string values.
export function formatEvidenceJSON(raw: string): string {
  try {
    JSON.parse(raw)
  } catch {
    return raw
  }
  const tokens = raw.match(/"(?:\\.|[^"\\])*"|[{}[\],:]|[^\s{}[\],:]+/g) || []
  const output: string[] = []
  let depth = 0
  for (const [index, token] of tokens.entries()) {
    if (token === '{' || token === '[') {
      output.push(token)
      if (tokens[index + 1] !== (token === '{' ? '}' : ']')) {
        depth++
        output.push('\n', '  '.repeat(Math.min(depth, 20)))
      }
    } else if (token === '}' || token === ']') {
      if (tokens[index - 1] !== (token === '}' ? '{' : '[')) {
        depth--
        output.push('\n', '  '.repeat(Math.min(depth, 20)))
      }
      output.push(token)
    } else if (token === ',') {
      output.push(',\n', '  '.repeat(Math.min(depth, 20)))
    } else if (token === ':') {
      output.push(': ')
    } else {
      output.push(token)
    }
  }
  return output.join('')
}

export function videoReportSections(
  run: VideoRunReport
): Array<{ title: string; stages: StageEvidence[] }> {
  return [
    {
      title: 'Video generation',
      ids: ['video_submit', 'video_queued', 'video_running', 'video_result'],
    },
    {
      title: 'Official artifact verification',
      ids: ['verification_submit', 'verification_result'],
    },
  ].map((section) => ({
    title: section.title,
    stages: section.ids.map(
      (id) =>
        run.stages.find((stage) => stage.id === id) || {
          id,
          purpose: videoStageTitle(id),
          status: 'skip',
          request_body: '',
          response_body: '',
          elapsed_ms: 0,
          conclusion: 'Not recorded',
          final: false,
        }
    ),
  }))
}

export function videoPollingSummary(
  run: VideoRunReport,
  t: VideoReportInput['t']
): string {
  return `${t('Polling summary')} / ${t('Total query time')}: ${t('Video generation')} ${run.video_polling?.count ?? '—'} / ${run.video_polling?.elapsed_ms ?? '—'} ms; ${t('Official artifact verification')} ${run.verification_polling?.count ?? '—'} / ${run.verification_polling?.elapsed_ms ?? '—'} ms`
}

function reportSummary(run: VideoRunReport, t: VideoReportInput['t']) {
  const final = run.stages.find((stage) => stage.id === 'video_result')
  let origin = 'Not verified'
  if (run.verification?.is_official === true) origin = 'Matched'
  if (run.verification?.is_official === false) origin = 'No features detected'
  if (run.verification?.status === 'failed') origin = 'Failed'
  const officialValue = run.verification?.is_official
  return [
    {
      label: t('Video generation'),
      value: t(
        final?.final ? videoEvidenceStatus(final.status) : 'Not verified'
      ),
      detail: t(final?.conclusion || 'No final response was obtained.'),
    },
    {
      label: t('Official origin features'),
      value: t(origin),
      detail: `IsOfficial = ${officialValue === undefined ? '—' : String(officialValue)}`,
    },
    {
      label: t('Model comparison'),
      value: t(videoEvidenceStatus(run.verification?.model_match || 'unknown')),
      detail:
        run.verification?.model_name ||
        t('Reliable model mapping is unavailable'),
    },
  ]
}

export function buildVideoRunMarkdownReport(
  run: VideoRunReport,
  input: VideoReportInput
): string {
  const t = input.t
  const capabilities = VIDEO_REPORT_CAPABILITIES.filter(
    (item) => input.manualChecks?.[item.key]
  )
  const lines = [
    `# ${t('Doubao Video Generation Test Report')}`,
    '',
    `- ${t('Model')}: ${run.model}`,
    `- ${t('Base URL')}: ${run.base_url}`,
    `- ${t('Task ID')}: ${run.task_id || '—'}`,
    `- QueryID: ${run.query_id || '—'}`,
  ]
  if (capabilities.length) {
    lines.push(
      '',
      `**${t('Manual confirmation')}**`,
      ...capabilities.map((item) => `- [x] ${t(item.label)}`),
      ''
    )
  }
  for (const card of reportSummary(run, t)) {
    lines.push(`- ${card.label}: ${card.value} · ${card.detail}`)
  }
  lines.push(`- ${videoPollingSummary(run, t)}`)
  if (run.assets.length) {
    lines.push(`- ${t('Input materials')}: ${run.assets.length}`)
  }
  if (run.video_url) {
    lines.push(`- ${t('Generated Video Output')}: ${run.video_url}`)
  }
  for (const [index, section] of videoReportSections(run).entries()) {
    lines.push('', `## ${index + 1}. ${t(section.title)}`)
    for (const stage of section.stages) {
      lines.push('', `### ${t(videoStageTitle(stage.id))}`)
      if (stage.method) {
        lines.push(
          `${stage.method} ${stage.endpoint || stage.path || '—'} · HTTP ${stage.http_status || '—'} · ${stage.elapsed_ms} ms · Request ID: ${stage.request_id || '—'}`
        )
        lines.push(
          `**${t('Request headers')}**`,
          '```http',
          stage.request_headers || t('Not recorded'),
          '```'
        )
        if (stage.request_body) {
          lines.push(
            `**${t('Request body')}**`,
            '```json',
            formatEvidenceJSON(stage.request_body),
            '```'
          )
        } else lines.push(t('No request body'))
      }
      if (stage.method || stage.response_body) {
        lines.push(
          `**${t('Response body')}**`,
          '```json',
          formatEvidenceJSON(stage.response_body) || t('No response received'),
          '```'
        )
      }
      lines.push('', t(stage.conclusion))
      if (!stage.final && stage.id.endsWith('_result')) {
        lines.push(t('No final response was obtained.'))
      }
    }
  }
  if (run.verification?.message) lines.push('', t(run.verification.message))
  for (const stage of run.stages.filter(
    (stage) => stage.id.startsWith('asset_') && stage.status === 'fail'
  )) {
    lines.push('', t(stage.conclusion))
  }
  if (run.evidence_error) lines.push('', t(run.evidence_error))
  lines.push('', `${t('Run ID')}: ${run.id}`)
  return `${lines.join('\n')}\n`
}

export function buildVideoRunHtmlReport(
  run: VideoRunReport,
  input: VideoReportInput,
  options: { format?: 'html' | 'pdf' }
): string {
  const t = input.t
  const pdf = options.format === 'pdf'
  const capabilities = VIDEO_REPORT_CAPABILITIES.filter(
    (item) => input.manualChecks?.[item.key]
  )
  const title = t('Doubao Video Generation Test Report')
  const code = (label: string, body: string) =>
    `<div class="evidence-label">${escapeHtml(t(label))}</div><pre class="evidence-json">${escapeHtml(formatEvidenceJSON(body))}</pre>`
  const metadata = [
    ['Model', run.model],
    ['Base URL', run.base_url],
    ['Task ID', run.task_id || '—'],
    ['QueryID', run.query_id || '—'],
  ]
  const blocks = [
    `<div class="report-header"><h1 class="report-title">${escapeHtml(title)}</h1><span class="record-reference">${escapeHtml(t('Run ID'))}: ${escapeHtml(run.id)}</span></div>`,
    `<div class="meta-grid">${metadata.map(([key, value]) => `<div class="meta-item"><span class="meta-label">${escapeHtml(t(key))}：</span><span class="meta-value">${escapeHtml(value)}</span></div>`).join('')}</div>`,
    ...(capabilities.length
      ? [
          `<p class="report-note">${escapeHtml(t('Manual confirmation'))}: ${capabilities.map((item) => `☑ ${escapeHtml(t(item.label))}`).join(' · ')}</p>`,
        ]
      : []),
    pdfSummaryCards(reportSummary(run, t)),
    `<p class="report-note">${escapeHtml(videoPollingSummary(run, t))}${run.assets.length ? ` · ${escapeHtml(t('Input materials'))}: ${run.assets.length}` : ''}</p>`,
  ]
  if (run.video_url) {
    blocks.push(
      `<p class="request-line">${escapeHtml(t('Generated Video Output'))}: ${escapeHtml(run.video_url)}</p>`
    )
  }
  for (const [index, section] of videoReportSections(run).entries()) {
    for (const [stageIndex, stage] of section.stages.entries()) {
      const stageTitle = t(videoStageTitle(stage.id))
      const conclusion = t(stage.conclusion)
      let body = `<h3 class="evidence-title">${escapeHtml(stageTitle)}${conclusion && conclusion !== stageTitle ? `<span class="stage-caption"> · ${escapeHtml(conclusion)}</span>` : ''}</h3>`
      if (stage.method) {
        body += `<p class="exchange-meta">${escapeHtml(stage.method)} ${escapeHtml(stage.endpoint || stage.path || '—')} · HTTP ${stage.http_status || '—'} · ${stage.elapsed_ms} ms · Request ID: ${escapeHtml(stage.request_id || '—')}${stage.request_body ? '' : ` · ${escapeHtml(t('No request body'))}`}</p>`
        body += code(
          'Request headers',
          stage.request_headers || t('Not recorded')
        )
        if (stage.request_body) body += code('Request body', stage.request_body)
      }
      if (stage.method || stage.response_body) {
        body += code(
          'Response body',
          stage.response_body || t('No response received')
        )
      }
      if (
        !stage.final &&
        stage.id.endsWith('_result') &&
        stage.conclusion !== 'No final response was obtained.'
      ) {
        body += `<p class="stage-conclusion">${escapeHtml(t('No final response was obtained.'))}</p>`
      }
      const heading =
        stageIndex === 0
          ? `<h2 class="part-title">${index + 1}. ${escapeHtml(t(section.title))}</h2>`
          : ''
      blocks.push(`${heading}<article class="evidence-stage">${body}</article>`)
    }
  }
  if (run.verification?.message) {
    blocks.push(`<p>${escapeHtml(t(run.verification.message))}</p>`)
  }
  for (const stage of run.stages.filter(
    (stage) => stage.id.startsWith('asset_') && stage.status === 'fail'
  )) {
    blocks.push(`<p>${escapeHtml(t(stage.conclusion))}</p>`)
  }
  if (run.evidence_error) {
    blocks.push(`<p>${escapeHtml(t(run.evidence_error))}</p>`)
  }
  const rows = blocks.map((body) => (pdf ? pdfRow(body) : body)).join('')
  return `<!doctype html><html lang="${escapeHtml(input.language || 'en')}"><head><meta charset="utf-8"><title>${escapeHtml(title)}</title><style>
    *{box-sizing:border-box}body{font-family:"Noto Sans SC","PingFang SC",sans-serif;color:#233145;max-width:960px;margin:20px auto;padding:16px;font-size:12px;line-height:1.4}.report-title{color:#19335e}.meta-grid{display:grid;grid-template-columns:1fr 1fr;gap:4px 12px}.meta-item,.request-line,.exchange-meta{overflow-wrap:anywhere}.document-summary{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:10px;margin:12px 0}.document-summary-card{background:#f1f5fb;padding:10px;overflow-wrap:anywhere}.document-summary-value{font-size:18px}.part-title{font-size:15px;color:#19335e;border-bottom:1px solid #bac8da;margin:12px 0 5px;padding-bottom:3px;break-after:avoid-page}.evidence-stage{margin:6px 0 9px}.evidence-title{font-size:12px;margin:0 0 2px;break-after:avoid-page}.exchange-meta,.report-note,.record-reference{font-size:10px;color:#536681;margin:3px 0}.evidence-label{font-size:10px;font-weight:600;margin:4px 0 2px;break-after:avoid-page}.evidence-json{white-space:pre-wrap;overflow-wrap:anywhere;word-break:break-word;font-family:monospace;background:#f5f7fb;padding:5px 7px;margin:0;font-size:10px;line-height:1.35;max-height:none;height:auto;overflow:visible;break-inside:auto}.stage-caption{font-size:10px;font-weight:400;color:#536681}.stage-conclusion{font-size:10px;margin:3px 0}
    ${pdf ? pdfPageFooterStyles(input.branding, title) + pdfDocumentStyles : ''}
    .video-report .report-header{display:flex;align-items:baseline;justify-content:space-between;gap:8pt;margin:0 0 7pt;padding:0}.video-report .report-title{font-size:19pt;line-height:24pt;margin:0}.video-report .meta-grid{display:grid;grid-template-columns:1fr 1fr;gap:3pt 10pt;margin:0 0 7pt}.video-report .meta-item{font-size:8pt;line-height:11pt}.video-report .document-summary{gap:8pt;margin:0 0 7pt}.video-report .document-summary-card{height:auto;padding:4pt 8pt}.video-report .document-summary-label{font-size:7.5pt;line-height:10pt}.video-report .document-summary-value{margin-top:2pt;font-size:12pt;line-height:14pt}.video-report .document-summary-detail{font-size:7pt;line-height:8.5pt;margin-top:2pt;overflow-wrap:anywhere}.video-report .part-title{font-size:12pt;line-height:16pt;margin:10pt 0 5pt;padding-bottom:3pt}.video-report .evidence-title{font-size:10pt;line-height:13pt}.video-report .evidence-stage{margin:6pt 0 10pt}.video-report .exchange-meta,.video-report .report-note,.video-report .record-reference{font-size:7pt;line-height:10pt;margin:2pt 0}.video-report .evidence-label{font-size:8pt;line-height:11pt;margin:4pt 0 2pt}.video-report .evidence-json{font-size:9pt;line-height:1.5;padding:6pt 8pt;margin:3pt 0 6pt}.video-report .stage-conclusion{font-size:7.5pt;line-height:10pt;margin:2pt 0}.video-report .pdf-shell > thead > tr > td{height:52pt}.video-report .document-letterhead{height:44pt;padding-top:5pt}.video-report .document-platform img{width:29pt;height:29pt}.video-report .document-platform strong{font-size:13pt;line-height:16pt}.video-report .document-operator{height:31pt}
    @media print{.video-report .evidence-json,.video-report .pdf-shell tbody tr,.video-report .pdf-shell tbody td{break-inside:auto!important;page-break-inside:auto!important}.video-report .evidence-json{overflow:visible!important;max-height:none!important}.video-report .part-title,.video-report .evidence-title,.video-report .evidence-label{break-after:avoid-page}.video-report .exchange-meta{break-after:avoid-page}.video-report .pdf-shell > thead{display:table-header-group}}
    </style></head><body class="video-report${pdf ? ' pdf-document' : ''}">${pdf ? `<table class="pdf-shell"><thead><tr><td>${pdfLetterhead(input.branding, `VIDEO TEST REPORT / ${title}`, t('Operating entity'))}</td></tr></thead><tbody>${rows}</tbody></table>` : rows}</body></html>`
}
