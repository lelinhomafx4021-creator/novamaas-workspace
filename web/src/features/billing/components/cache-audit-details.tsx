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
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { billingTimestamp } from '../api'
import type { CacheAuditReport, CacheAuditRow } from '../cache-audit-api'
import {
  CACHE_AUDIT_PRICE_ISSUE_LABELS,
  CACHE_AUDIT_REASON_LABELS,
  CACHE_AUDIT_STATUS_LABELS,
  formatTokens,
} from '../lib/cache-audit'

function statusVariant(row: CacheAuditRow) {
  if (row.status === 'affected') return 'destructive' as const
  if (row.status === 'review' || row.status === 'no_evidence') {
    return 'secondary' as const
  }
  return 'outline' as const
}

function DuplicateCell(props: { row: CacheAuditRow }) {
  const { t } = useTranslation()
  if (props.row.status === 'review') {
    return (
      <span>
        {t('0 to {{count}}', {
          count: formatTokens(props.row.duplicate_tokens_max),
        })}
      </span>
    )
  }
  return <span>{formatTokens(props.row.duplicate_tokens)}</span>
}

export function CacheAuditDetails(props: { report: CacheAuditReport }) {
  const { t } = useTranslation()
  const symbol = props.report.currency.symbol
  const priceNote = (row: CacheAuditRow) => {
    if (row.price_issue) {
      return t(
        CACHE_AUDIT_PRICE_ISSUE_LABELS[row.price_issue] ?? row.price_issue
      )
    }
    if (row.price_mismatch) return t('Group price differs from actual charge')
    return ''
  }
  return (
    <section className='flex flex-col gap-3'>
      <h3 className='font-semibold'>{t('Request details')}</h3>
      {props.report.truncated && (
        <p className='text-sm' role='status'>
          {t(
            'Only the first 2000 records are shown. Export the report for every record.'
          )}
        </p>
      )}
      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Time')}</TableHead>
              <TableHead>{t('Request ID')}</TableHead>
              <TableHead>{t('API key')}</TableHead>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Total input')}</TableHead>
              <TableHead className='text-right'>
                {t('Uncached input')}
              </TableHead>
              <TableHead className='text-right'>{t('Cache read')}</TableHead>
              <TableHead className='text-right'>{t('Cache write')}</TableHead>
              <TableHead className='text-right'>{t('Output')}</TableHead>
              <TableHead className='text-right'>
                {t('Returned input_tokens')}
              </TableHead>
              <TableHead className='text-right'>
                {t('Cache counted as input')}
              </TableHead>
              <TableHead className='text-right'>{t('Actual charge')}</TableHead>
              <TableHead className='text-right'>{t('Group price')}</TableHead>
              <TableHead className='text-right'>{t('List price')}</TableHead>
              <TableHead className='text-right'>
                {t('Compensation (list)')}
              </TableHead>
              <TableHead className='text-right'>
                {t('Compensation (customer)')}
              </TableHead>
              <TableHead>{t('Note')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.report.rows.map((row) => (
              <TableRow
                key={`${row.log_id}-${row.request_id}-${row.created_at}`}
              >
                <TableCell className='whitespace-nowrap'>
                  {billingTimestamp(row.created_at)}
                </TableCell>
                <TableCell className='font-mono text-xs break-all'>
                  {row.request_id || '-'}
                </TableCell>
                <TableCell>
                  {row.token_name || '-'} (#{row.token_id})
                </TableCell>
                <TableCell>{row.model}</TableCell>
                <TableCell>
                  <Badge variant={statusVariant(row)}>
                    {t(CACHE_AUDIT_STATUS_LABELS[row.status])}
                  </Badge>
                  {row.reason && (
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t(CACHE_AUDIT_REASON_LABELS[row.reason] ?? row.reason)}
                    </p>
                  )}
                </TableCell>
                <TableCell className='text-right'>
                  {formatTokens(row.tokens.total_input)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatTokens(row.tokens.input)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatTokens(row.tokens.cache_read)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatTokens(row.tokens.cache_write)}
                </TableCell>
                <TableCell className='text-right'>
                  {formatTokens(row.tokens.output)}
                </TableCell>
                <TableCell className='text-right'>
                  {row.status === 'affected'
                    ? formatTokens(row.reported_input_tokens)
                    : '-'}
                </TableCell>
                <TableCell className='text-right font-medium text-amber-600 dark:text-amber-400'>
                  <DuplicateCell row={row} />
                </TableCell>
                <TableCell className='text-right'>
                  {symbol}
                  {row.actual_amount}
                </TableCell>
                <TableCell className='text-right'>
                  {row.discounted_cost
                    ? `${symbol}${row.discounted_cost.total}`
                    : '-'}
                </TableCell>
                <TableCell className='text-right'>
                  {row.list_cost ? `${symbol}${row.list_cost.total}` : '-'}
                </TableCell>
                <TableCell className='text-right'>
                  {row.compensation?.list_max
                    ? `${symbol}${row.compensation.list_max}`
                    : '-'}
                </TableCell>
                <TableCell className='text-right'>
                  {row.compensation?.downstream_max
                    ? `${symbol}${row.compensation.downstream_max}`
                    : '-'}
                </TableCell>
                <TableCell className='text-xs'>{priceNote(row)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </section>
  )
}
