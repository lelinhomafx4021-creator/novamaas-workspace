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

import type { CacheAuditReport, CacheAuditTotals } from '../cache-audit-api'
import { formatTokens } from '../lib/cache-audit'

type Part = 'input' | 'cache_read' | 'cache_write' | 'output'

const PARTS: { key: Part; label: string }[] = [
  { key: 'input', label: 'Uncached input' },
  { key: 'cache_read', label: 'Cache read' },
  { key: 'cache_write', label: 'Cache write' },
  { key: 'output', label: 'Output' },
]

function difference(platform: string, list: string) {
  return (Number(platform) - Number(list)).toFixed(6)
}

function Tile(props: { label: string; value: number }) {
  return (
    <div className='rounded-xl border p-3'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <p className='text-2xl font-semibold text-sky-600 tabular-nums dark:text-sky-400'>
        {formatTokens(props.value)}
      </p>
    </div>
  )
}

export function CacheAuditModelCard(props: {
  row: CacheAuditTotals
  title: string
  symbol: string
}) {
  const { t } = useTranslation()
  const row = props.row
  const symbol = props.symbol
  const review = row.duplicate_tokens_max - row.duplicate_tokens
  const comp = row.compensation
  return (
    <article className='flex flex-col gap-4 rounded-xl border p-4'>
      <header className='flex flex-col gap-1'>
        <h4 className='text-lg font-semibold break-all'>{props.title}</h4>
        <p className='text-muted-foreground text-sm'>
          {t('Priced {{priced}} / {{total}} records.', {
            priced: row.priced_requests,
            total: row.requests,
          })}
        </p>
        {row.rates.length > 0 && (
          <div className='flex flex-wrap gap-1'>
            {row.rates.map((rate) => (
              <Badge key={rate} variant='secondary'>
                {t('Billing rate')}: {rate}
              </Badge>
            ))}
          </div>
        )}
      </header>

      <div className='rounded-xl border border-amber-500/50 bg-amber-500/5 p-4'>
        <p className='text-sm font-medium'>
          {t('Tokens to correct (cache counted as input)')}
        </p>
        <p className='text-4xl font-bold text-amber-600 tabular-nums dark:text-amber-400'>
          {formatTokens(row.duplicate_tokens)}
        </p>
        <p className='text-muted-foreground text-sm'>
          {t('{{count}} affected requests', { count: row.affected_requests })}
          {' · '}
          {t('Returned input_tokens {{reported}} → correct {{correct}}', {
            reported: formatTokens(row.affected_reported_input),
            correct: formatTokens(row.affected_correct_input),
          })}
        </p>
        {review > 0 && (
          <p className='text-sm'>
            {t('Plus up to {{count}} tokens that need review', {
              count: formatTokens(review),
            })}
          </p>
        )}
        <div className='mt-3 grid gap-3 sm:grid-cols-3'>
          <div>
            <p className='text-muted-foreground text-xs'>
              {t('List input price / 1M')}
            </p>
            <p className='font-semibold tabular-nums'>
              {row.input_price_per_million
                ? `${symbol}${row.input_price_per_million}${row.price_mixed ? '*' : ''}`
                : '-'}
            </p>
          </div>
          <div>
            <p className='text-muted-foreground text-xs'>
              {t('Extra charge at list price')}
            </p>
            <p className='font-semibold tabular-nums'>
              {symbol}
              {comp.list}
            </p>
          </div>
          <div>
            <p className='text-muted-foreground text-xs'>
              {t('Extra charge at customer price')}
            </p>
            <p className='font-semibold tabular-nums'>
              {comp.downstream ? `${symbol}${comp.downstream}` : '-'}
            </p>
          </div>
        </div>
      </div>

      <div className='flex items-center justify-between'>
        <h5 className='font-semibold'>{t('Token usage')}</h5>
        <p className='text-muted-foreground text-sm'>
          {t('Total tokens')}:{' '}
          <span className='text-foreground font-semibold tabular-nums'>
            {formatTokens(row.tokens.total_input + row.tokens.output)}
          </span>
        </p>
      </div>
      <div className='grid grid-cols-2 gap-3'>
        {PARTS.map((part) => (
          <Tile
            key={part.key}
            label={t(part.label)}
            value={row.tokens[part.key]}
          />
        ))}
      </div>

      <div className='grid grid-cols-3 gap-3'>
        <div>
          <p className='text-muted-foreground text-xs'>
            {t('Platform charge')}
          </p>
          <p className='text-lg font-semibold tabular-nums'>
            {symbol}
            {row.actual_amount}
          </p>
        </div>
        <div>
          <p className='text-muted-foreground text-xs'>{t('List price')}</p>
          <p className='text-lg font-semibold tabular-nums'>
            {symbol}
            {row.list_amount}
          </p>
        </div>
        <div>
          <p className='text-muted-foreground text-xs'>
            {t('Price difference')}
          </p>
          <p className='text-lg font-semibold tabular-nums'>
            {symbol}
            {difference(row.discounted_amount, row.list_amount)}
          </p>
        </div>
      </div>

      <div className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Token type')}</TableHead>
              <TableHead className='text-right'>Token</TableHead>
              <TableHead className='text-right'>
                {t('Platform charge')}
              </TableHead>
              <TableHead className='text-right'>{t('List price')}</TableHead>
              <TableHead className='text-right'>
                {t('Price difference')}
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {PARTS.map((part) => (
              <TableRow key={part.key}>
                <TableCell>{t(part.label)}</TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatTokens(row.tokens[part.key])}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {symbol}
                  {row.discounted_cost[part.key]}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {symbol}
                  {row.list_cost[part.key]}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {symbol}
                  {difference(
                    row.discounted_cost[part.key],
                    row.list_cost[part.key]
                  )}
                </TableCell>
              </TableRow>
            ))}
            <TableRow className='bg-amber-500/5 font-medium'>
              <TableCell>{t('Cache counted as input')}</TableCell>
              <TableCell className='text-right text-amber-600 tabular-nums dark:text-amber-400'>
                {formatTokens(row.duplicate_tokens)}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {symbol}
                {comp.group}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {symbol}
                {comp.list}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {symbol}
                {difference(comp.group, comp.list)}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Platform charge in the table is rebuilt at the historical group price; the headline platform charge is the settled amount. The highlighted row is what customers over-billed: those tokens were already billed as cache.'
        )}
      </p>
    </article>
  )
}

export function CacheAuditModelUsage(props: { report: CacheAuditReport }) {
  const { t } = useTranslation()
  const symbol = props.report.currency.symbol
  return (
    <section className='flex flex-col gap-3'>
      <h3 className='font-semibold'>{t('Usage and amount by model')}</h3>
      <div className='grid gap-4 xl:grid-cols-2'>
        {props.report.by_model.map((row) => (
          <CacheAuditModelCard
            key={row.key}
            row={row}
            title={row.key || '-'}
            symbol={symbol}
          />
        ))}
      </div>
    </section>
  )
}
