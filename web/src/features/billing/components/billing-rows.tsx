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
import { CircleHelp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableFooter,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  ADMIN_PERMISSION_ACTIONS,
  ADMIN_PERMISSION_RESOURCES,
  hasPermission,
} from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import type { BillingRow } from '../types'

export function BillingRows(props: {
  rows: BillingRow[]
  total: BillingRow
  symbol: string
  roundingDifference?: string
  onSelectRow?: (row: BillingRow) => void
}) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const showFinancialAccounting =
    hasPermission(
      user,
      ADMIN_PERMISSION_RESOURCES.FINANCIAL_ACCOUNTING,
      ADMIN_PERMISSION_ACTIONS.VIEW
    ) &&
    (props.total.cost !== undefined ||
      props.rows.some((row) => row.cost !== undefined))
  const financialColumnTip = t(
    'This column is only visible to users with financial accounting access'
  )
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Date / hour')}</TableHead>
          <TableHead className='text-right'>{t('Consumption')}</TableHead>
          <TableHead className='text-right'>{t('Refund')}</TableHead>
          <TableHead className='text-right'>{t('Net amount')}</TableHead>
          {showFinancialAccounting ? (
            <>
              <TableHead className='text-right text-amber-600 dark:text-amber-400'>
                <Tooltip>
                  <TooltipTrigger
                    render={<span className='inline-flex items-center gap-1' />}
                  >
                    {t('Cost amount')}
                    <CircleHelp className='size-3.5' aria-hidden='true' />
                  </TooltipTrigger>
                  <TooltipContent>{financialColumnTip}</TooltipContent>
                </Tooltip>
              </TableHead>
              <TableHead className='text-right text-emerald-600 dark:text-emerald-400'>
                <Tooltip>
                  <TooltipTrigger
                    render={<span className='inline-flex items-center gap-1' />}
                  >
                    {t('Profit amount')}
                    <CircleHelp className='size-3.5' aria-hidden='true' />
                  </TooltipTrigger>
                  <TooltipContent>{financialColumnTip}</TooltipContent>
                </Tooltip>
              </TableHead>
            </>
          ) : null}
          <TableHead className='text-right'>{t('Records')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.rows.map((row) => (
          <TableRow key={row.label}>
            <TableCell>
              {props.onSelectRow &&
              row.count > 0 &&
              row.state !== 'future' &&
              row.state !== 'outside_period' ? (
                <Button
                  variant='link'
                  className='h-auto p-0'
                  onClick={() => props.onSelectRow?.(row)}
                >
                  {row.label}
                </Button>
              ) : (
                row.label
              )}
              {row.state === 'in_progress' && (
                <Badge variant='outline' className='ml-2'>
                  {t('In progress')}
                </Badge>
              )}
            </TableCell>
            {row.state === 'future' || row.state === 'outside_period' ? (
              <TableCell
                colSpan={showFinancialAccounting ? 6 : 4}
                className='text-muted-foreground text-right'
              >
                —{' '}
                {row.state === 'future'
                  ? t('Not yet occurred')
                  : t('Outside accounting period')}
              </TableCell>
            ) : (
              <>
                <TableCell className='text-right tabular-nums'>
                  {props.symbol} {row.charge}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {props.symbol} {row.refund}
                </TableCell>
                <TableCell className='text-right font-medium tabular-nums'>
                  {props.symbol} {row.amount}
                </TableCell>
                {showFinancialAccounting ? (
                  <>
                    <TableCell className='bg-amber-500/5 text-right text-amber-700 tabular-nums dark:text-amber-300'>
                      {row.cost ? `${props.symbol} ${row.cost}` : '—'}
                    </TableCell>
                    <TableCell className='bg-emerald-500/5 text-right font-medium text-emerald-700 tabular-nums dark:text-emerald-300'>
                      {row.profit ? `${props.symbol} ${row.profit}` : '—'}
                    </TableCell>
                  </>
                ) : null}
                <TableCell className='text-right tabular-nums'>
                  {row.count}
                </TableCell>
              </>
            )}
          </TableRow>
        ))}
      </TableBody>
      <TableFooter>
        <TableRow>
          <TableCell>{t('Total')}</TableCell>
          <TableCell className='text-right tabular-nums'>
            {props.symbol} {props.total.charge}
          </TableCell>
          <TableCell className='text-right tabular-nums'>
            {props.symbol} {props.total.refund}
          </TableCell>
          <TableCell className='text-right tabular-nums'>
            {props.symbol} {props.total.amount}
          </TableCell>
          {showFinancialAccounting ? (
            <>
              <TableCell className='bg-amber-500/10 text-right text-amber-700 tabular-nums dark:text-amber-300'>
                {props.total.cost ? `${props.symbol} ${props.total.cost}` : '—'}
              </TableCell>
              <TableCell className='bg-emerald-500/10 text-right font-medium text-emerald-700 tabular-nums dark:text-emerald-300'>
                {props.total.profit
                  ? `${props.symbol} ${props.total.profit}`
                  : '—'}
              </TableCell>
            </>
          ) : null}
          <TableCell className='text-right tabular-nums'>
            {props.total.count}
          </TableCell>
        </TableRow>
        {props.roundingDifference &&
          props.roundingDifference !== '0.000000' && (
            <TableRow>
              <TableCell colSpan={3}>
                {t('Display rounding difference')}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {props.symbol} {props.roundingDifference}
              </TableCell>
              <TableCell colSpan={showFinancialAccounting ? 3 : 1} />
            </TableRow>
          )}
      </TableFooter>
    </Table>
  )
}
