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
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type { CacheAuditReport } from '../cache-audit-api'
import { formatTokens } from '../lib/cache-audit'

export function CacheAuditCompensation(props: {
  report: CacheAuditReport
  prices: Record<string, string>
  pending: boolean
  onPriceChange: (model: string, value: string) => void
  onRecalculate: () => void
}) {
  const { t } = useTranslation()
  const total = props.report.total
  const symbol = props.report.currency.symbol
  const comp = total.compensation
  const models = props.report.by_model.filter(
    (row) => row.duplicate_tokens_max > 0
  )
  const reviewTokens = total.duplicate_tokens_max - total.duplicate_tokens
  return (
    <section className='flex flex-col gap-3 rounded-md border p-4'>
      <h3 className='font-semibold'>{t('How much to compensate')}</h3>
      <p className='text-sm'>
        {t(
          'Compensation = duplicated tokens × uncached input price. Cache tokens were already billed at the cache price, so only the extra uncached input charge needs to be returned.'
        )}
      </p>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Price basis')}</TableHead>
            <TableHead className='text-right'>
              {t('Confirmed ({{count}} tokens)', {
                count: formatTokens(total.duplicate_tokens),
              })}
            </TableHead>
            <TableHead className='text-right'>
              {t('Including review upper bound')}
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow>
            <TableCell>{t('Customer input price (entered below)')}</TableCell>
            <TableCell className='text-right font-semibold text-amber-600 dark:text-amber-400'>
              {comp.downstream ? `${symbol}${comp.downstream}` : '-'}
            </TableCell>
            <TableCell className='text-right'>
              {comp.downstream_max ? `${symbol}${comp.downstream_max}` : '-'}
            </TableCell>
          </TableRow>
          <TableRow>
            <TableCell>{t('Platform list price')}</TableCell>
            <TableCell className='text-right'>
              {symbol}
              {comp.list}
            </TableCell>
            <TableCell className='text-right'>
              {symbol}
              {comp.list_max}
            </TableCell>
          </TableRow>
          <TableRow>
            <TableCell>{t('Platform group price')}</TableCell>
            <TableCell className='text-right'>
              {symbol}
              {comp.group}
            </TableCell>
            <TableCell className='text-right'>
              {symbol}
              {comp.group_max}
            </TableCell>
          </TableRow>
        </TableBody>
      </Table>
      <ul className='text-muted-foreground list-disc pl-5 text-sm'>
        <li>
          {t(
            'Use the customer input price for the actual refund. Platform prices are reference values only, because the customer resells at their own price.'
          )}
        </li>
        {reviewTokens > 0 && (
          <li>
            {t(
              '{{count}} tokens are in requests that need review. Compare them with the original responses before compensating.',
              { count: formatTokens(reviewTokens) }
            )}
          </li>
        )}
        {total.unpriced_duplicate_tokens > 0 && (
          <li>
            {t(
              '{{count}} duplicated tokens have no historical input price. Enter the customer price for those models to include them.',
              { count: formatTokens(total.unpriced_duplicate_tokens) }
            )}
          </li>
        )}
        <li>
          {t(
            'Insufficient-evidence requests are excluded. Deduct any refunds already issued before settlement.'
          )}
        </li>
      </ul>
      {models.length > 0 && (
        <div className='flex flex-col gap-2'>
          <p className='text-sm font-medium'>
            {t('Customer uncached input price per 1M tokens ({{code}})', {
              code: props.report.currency.code,
            })}
          </p>
          <div className='grid gap-2 sm:grid-cols-3'>
            {models.map((row) => (
              <label key={row.key} className='flex flex-col gap-1 text-sm'>
                <span className='break-all'>
                  {row.key} · {formatTokens(row.duplicate_tokens)}
                </span>
                <Input
                  inputMode='decimal'
                  value={props.prices[row.key] ?? ''}
                  placeholder='0.00'
                  disabled={props.pending}
                  onChange={(event) =>
                    props.onPriceChange(row.key, event.target.value)
                  }
                />
              </label>
            ))}
          </div>
          <Button
            type='button'
            variant='outline'
            className='self-start'
            disabled={props.pending}
            onClick={props.onRecalculate}
          >
            {t('Recalculate compensation')}
          </Button>
        </div>
      )}
    </section>
  )
}
