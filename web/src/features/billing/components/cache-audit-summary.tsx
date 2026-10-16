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

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { CacheAuditReport, CacheAuditTotals } from '../cache-audit-api'
import { formatTokens } from '../lib/cache-audit'

function Metric(props: { label: string; value: string; highlight?: boolean }) {
  return (
    <div className='rounded-md border p-3'>
      <p className='text-muted-foreground text-xs'>{props.label}</p>
      <p
        className={
          props.highlight
            ? 'text-lg font-semibold text-amber-600 tabular-nums dark:text-amber-400'
            : 'text-lg font-semibold tabular-nums'
        }
      >
        {props.value}
      </p>
    </div>
  )
}

function GroupTable(props: {
  rows: CacheAuditTotals[]
  label: string
  symbol: string
}) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{props.label}</TableHead>
          <TableHead className='text-right'>{t('Requests')}</TableHead>
          <TableHead className='text-right'>{t('Affected')}</TableHead>
          <TableHead className='text-right'>{t('Total input')}</TableHead>
          <TableHead className='text-right'>{t('Uncached input')}</TableHead>
          <TableHead className='text-right'>{t('Cache read')}</TableHead>
          <TableHead className='text-right'>{t('Cache write')}</TableHead>
          <TableHead className='text-right'>{t('Output')}</TableHead>
          <TableHead className='text-right'>
            {t('Cache counted as input')}
          </TableHead>
          <TableHead className='text-right'>{t('Actual charge')}</TableHead>
          <TableHead className='text-right'>{t('Group price')}</TableHead>
          <TableHead className='text-right'>{t('List price')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.rows.map((row) => (
          <TableRow key={row.key}>
            <TableCell className='break-all'>{row.key || '-'}</TableCell>
            <TableCell className='text-right'>{row.requests}</TableCell>
            <TableCell className='text-right'>{row.affected_requests}</TableCell>
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
            <TableCell className='text-right font-medium text-amber-600 dark:text-amber-400'>
              {formatTokens(row.duplicate_tokens)}
            </TableCell>
            <TableCell className='text-right'>
              {props.symbol}
              {row.actual_amount}
            </TableCell>
            <TableCell className='text-right'>
              {props.symbol}
              {row.discounted_amount}
            </TableCell>
            <TableCell className='text-right'>
              {props.symbol}
              {row.list_amount}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

export function CacheAuditSummary(props: { report: CacheAuditReport }) {
  const { t } = useTranslation()
  const total = props.report.total
  const symbol = props.report.currency.symbol
  const unpriced = total.requests - total.priced_requests
  return (
    <section className='flex flex-col gap-3'>
      <h3 className='font-semibold'>{t('Period summary')}</h3>
      <div className='grid gap-3 sm:grid-cols-3 lg:grid-cols-6'>
        <Metric label={t('Requests')} value={String(total.requests)} />
        <Metric
          label={t('Affected requests')}
          value={String(total.affected_requests)}
        />
        <Metric
          label={t('Cache counted as input')}
          value={formatTokens(total.duplicate_tokens)}
          highlight
        />
        <Metric
          label={t('Actual charge')}
          value={`${symbol}${total.actual_amount}`}
        />
        <Metric
          label={t('Group price')}
          value={`${symbol}${total.discounted_amount}`}
        />
        <Metric label={t('List price')} value={`${symbol}${total.list_amount}`} />
        <Metric
          label={t('Total input')}
          value={formatTokens(total.tokens.total_input)}
        />
        <Metric
          label={t('Uncached input')}
          value={formatTokens(total.tokens.input)}
        />
        <Metric
          label={t('Cache read')}
          value={formatTokens(total.tokens.cache_read)}
        />
        <Metric
          label={t('Cache write')}
          value={formatTokens(total.tokens.cache_write)}
        />
        <Metric label={t('Output')} value={formatTokens(total.tokens.output)} />
        <Metric
          label={t('Needs review / insufficient evidence')}
          value={`${total.review_requests} / ${total.no_evidence_requests}`}
        />
      </div>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Total input = uncached input + cache read + cache write. Actual charge is the settled amount; group price and list price are rebuilt from the historical ratios saved with each request.'
        )}
      </p>
      {unpriced > 0 && (
        <p className='text-sm'>
          {t(
            '{{count}} requests could not be repriced from history; they are included in the actual charge only.',
            { count: unpriced }
          )}
        </p>
      )}
      {total.duplicate_tokens_max > total.duplicate_tokens && (
        <p className='text-sm'>
          {t(
            'Up to {{count}} more tokens may have been duplicated in requests that need review. Compare them with the original responses.',
            {
              count: total.duplicate_tokens_max - total.duplicate_tokens,
            }
          )}
        </p>
      )}
      <Tabs defaultValue='day'>
        <TabsList>
          <TabsTrigger value='day'>{t('By day')}</TabsTrigger>
          <TabsTrigger value='model'>{t('By model')}</TabsTrigger>
          <TabsTrigger value='key'>{t('By key')}</TabsTrigger>
        </TabsList>
        <TabsContent value='day' className='overflow-x-auto'>
          <GroupTable rows={props.report.by_day} label={t('Date')} symbol={symbol} />
        </TabsContent>
        <TabsContent value='model' className='overflow-x-auto'>
          <GroupTable rows={props.report.by_model} label={t('Model')} symbol={symbol} />
        </TabsContent>
        <TabsContent value='key' className='overflow-x-auto'>
          <GroupTable rows={props.report.by_token} label={t('API key')} symbol={symbol} />
        </TabsContent>
      </Tabs>
    </section>
  )
}
