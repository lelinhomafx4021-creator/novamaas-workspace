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

import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type { CacheAuditReport, CacheAuditTotals } from '../cache-audit-api'
import { cacheAuditModelTokenCsv, formatTokens } from '../lib/cache-audit'

function price(symbol: string, value?: string, mixed?: boolean) {
  if (!value) return '-'
  return `${symbol}${value}${mixed ? '*' : ''}`
}

function TokenRow(props: {
  row: CacheAuditTotals
  label: string
  symbol: string
  showPrice: boolean
}) {
  const row = props.row
  const comp = row.compensation
  return (
    <>
      <TableCell className='font-medium break-all'>{props.label}</TableCell>
      <TableCell className='text-right'>{row.affected_requests}</TableCell>
      <TableCell className='text-right'>
        {formatTokens(row.affected_reported_input)}
      </TableCell>
      <TableCell className='text-right'>
        {formatTokens(row.affected_correct_input)}
      </TableCell>
      <TableCell className='text-right text-base font-semibold text-amber-600 dark:text-amber-400'>
        {formatTokens(row.duplicate_tokens)}
      </TableCell>
      <TableCell className='text-right'>
        {row.duplicate_tokens_max > row.duplicate_tokens
          ? formatTokens(row.duplicate_tokens_max - row.duplicate_tokens)
          : '-'}
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
        {props.showPrice
          ? price(props.symbol, row.input_price_per_million, row.price_mixed)
          : ''}
      </TableCell>
      <TableCell className='text-right'>
        {props.showPrice
          ? price(
              props.symbol,
              row.cache_read_price_per_million,
              row.price_mixed
            )
          : ''}
      </TableCell>
      <TableCell className='text-right'>
        {props.symbol}
        {comp.list}
      </TableCell>
      <TableCell className='text-right font-medium'>
        {comp.downstream ? `${props.symbol}${comp.downstream}` : '-'}
      </TableCell>
    </>
  )
}

function downloadModelTokens(report: CacheAuditReport) {
  const blob = new Blob(['﻿', cacheAuditModelTokenCsv(report)], {
    type: 'text/csv;charset=utf-8',
  })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'cache-usage-audit-by-model.csv'
  anchor.click()
  URL.revokeObjectURL(url)
}

export function CacheAuditTokens(props: { report: CacheAuditReport }) {
  const { t } = useTranslation()
  const total = props.report.total
  const symbol = props.report.currency.symbol
  const models = props.report.by_model.filter(
    (row) => row.duplicate_tokens_max > 0
  )
  const reviewTokens = total.duplicate_tokens_max - total.duplicate_tokens
  return (
    <section className='flex flex-col gap-3 rounded-md border p-4'>
      <h3 className='font-semibold'>{t('Duplicated tokens by model')}</h3>
      <div className='grid gap-3 sm:grid-cols-4'>
        <div className='rounded-md border border-amber-500/40 bg-amber-500/5 p-3 sm:col-span-2'>
          <p className='text-muted-foreground text-xs'>
            {t('Cache tokens counted again as input (confirmed)')}
          </p>
          <p className='text-3xl font-bold text-amber-600 tabular-nums dark:text-amber-400'>
            {formatTokens(total.duplicate_tokens)}
          </p>
          <p className='text-muted-foreground text-xs'>
            {t('Returned input_tokens {{reported}} → correct {{correct}}', {
              reported: formatTokens(total.affected_reported_input),
              correct: formatTokens(total.affected_correct_input),
            })}
          </p>
        </div>
        <div className='rounded-md border p-3'>
          <p className='text-muted-foreground text-xs'>
            {t('Needs review (upper bound)')}
          </p>
          <p className='text-2xl font-semibold tabular-nums'>
            {formatTokens(reviewTokens)}
          </p>
        </div>
        <div className='rounded-md border p-3'>
          <p className='text-muted-foreground text-xs'>
            {t('Affected requests / models')}
          </p>
          <p className='text-2xl font-semibold tabular-nums'>
            {total.affected_requests} / {models.length}
          </p>
        </div>
      </div>
      {models.length === 0 ? (
        <p className='text-sm'>{t('No duplicated tokens in this period.')}</p>
      ) : (
        <div className='overflow-x-auto'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead className='text-right'>
                  {t('Affected requests')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Returned input_tokens')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Correct input_tokens')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Cache counted as input')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Needs review (upper bound)')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Uncached input')}
                </TableHead>
                <TableHead className='text-right'>{t('Cache read')}</TableHead>
                <TableHead className='text-right'>{t('Cache write')}</TableHead>
                <TableHead className='text-right'>{t('Output')}</TableHead>
                <TableHead className='text-right'>
                  {t('List input price / 1M')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('List cache read price / 1M')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Extra charge at list price')}
                </TableHead>
                <TableHead className='text-right'>
                  {t('Extra charge at customer price')}
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {models.map((row) => (
                <TableRow key={row.key}>
                  <TokenRow
                    row={row}
                    label={row.key || '-'}
                    symbol={symbol}
                    showPrice
                  />
                </TableRow>
              ))}
            </TableBody>
            <TableFooter>
              <TableRow>
                <TokenRow
                  row={total}
                  label={t('Total')}
                  symbol={symbol}
                  showPrice={false}
                />
              </TableRow>
            </TableFooter>
          </Table>
        </div>
      )}
      {models.some((row) => row.price_mixed) && (
        <p className='text-muted-foreground text-xs'>
          {t(
            '* This model had more than one historical price in the period; the first is shown and amounts are summed per request.'
          )}
        </p>
      )}
      <div className='flex flex-col gap-1 text-sm'>
        <p className='font-medium'>{t('How customers correct their users')}</p>
        <ol className='text-muted-foreground list-decimal pl-5'>
          <li>
            {t(
              'For each request, the correct input_tokens = returned input_tokens − duplicated tokens. Cache read and cache write tokens were already correct.'
            )}
          </li>
          <li>
            {t(
              'If cache tokens were billed separately, refund duplicated tokens × that model’s uncached input price. If only input_tokens was billed, refund duplicated tokens × (input price − cache read price).'
            )}
          </li>
          <li>
            {t(
              'Match exported request IDs to their own records to distribute the correction to each end user. Confirm review requests against the original responses first.'
            )}
          </li>
        </ol>
      </div>
      <Button
        type='button'
        variant='outline'
        className='self-start'
        disabled={models.length === 0}
        onClick={() => downloadModelTokens(props.report)}
      >
        {t('Export tokens by model')}
      </Button>
    </section>
  )
}
