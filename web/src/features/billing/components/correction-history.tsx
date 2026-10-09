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
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuotaWithCurrency } from '@/lib/currency'

import { billingTimestamp } from '../api'
import type { CorrectionBatch } from '../correction-api'

export function CorrectionHistory(props: {
  items: CorrectionBatch[]
  loading: boolean
  failed: boolean
  refreshing: boolean
  disabled: boolean
  onRefresh: () => void
  onSelect: (id: string) => void
}) {
  const { t } = useTranslation()
  const statuses = {
    preview: t('Preview'),
    applied: t('Adjustment applied'),
    reversed: t('Adjustment reversed'),
  }
  return (
    <Card className='min-w-0'>
      <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-3'>
        <CardTitle>{t('Adjustment history')}</CardTitle>
        <Button
          type='button'
          variant='outline'
          disabled={props.refreshing || props.disabled}
          onClick={props.onRefresh}
        >
          {t('Refresh')}
        </Button>
      </CardHeader>
      <CardContent>
        {props.failed && <p role='alert'>{t('Failed to load data')}</p>}
        <Table aria-label={t('Adjustment history')}>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Batch ID')}</TableHead>
              <TableHead>{t('Adjustment type')}</TableHead>
              <TableHead>{t('Created At')}</TableHead>
              <TableHead>{t('Target billing group')}</TableHead>
              <TableHead className='text-right'>
                {t('Net debit (negative means credit)')}
              </TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.loading && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground py-6 text-center'
                >
                  {t('Loading...')}
                </TableCell>
              </TableRow>
            )}
            {!props.loading && !props.failed && props.items.length === 0 && (
              <TableRow>
                <TableCell
                  colSpan={7}
                  className='text-muted-foreground py-6 text-center'
                >
                  {t('No data')}
                </TableCell>
              </TableRow>
            )}
            {props.items.map((item) => (
              <TableRow key={item.id}>
                <TableCell className='max-w-64 min-w-40 font-mono text-xs break-all whitespace-normal'>
                  {item.id}
                </TableCell>
                <TableCell>
                  <Badge variant='outline'>
                    {item.mode === 'model_pricing'
                      ? t('Model pricing adjustment')
                      : t('Billing group adjustment')}
                  </Badge>
                </TableCell>
                <TableCell>{billingTimestamp(item.created_at)}</TableCell>
                <TableCell className='max-w-48 break-all whitespace-normal'>
                  {item.mode === 'model_pricing'
                    ? t('Existing billing groups')
                    : item.target_group}
                  {item.mode !== 'model_pricing' && (
                    <span className='text-muted-foreground block text-xs'>
                      {(Number(item.target_rate) * 100).toFixed(2)}%
                    </span>
                  )}
                </TableCell>
                <TableCell className='text-right'>
                  {formatQuotaWithCurrency(item.net_delta, {
                    digitsLarge: 6,
                    digitsSmall: 6,
                    abbreviate: false,
                  })}
                </TableCell>
                <TableCell>
                  <Badge
                    variant={
                      item.status === 'applied' ? 'default' : 'secondary'
                    }
                  >
                    {statuses[item.status]}
                  </Badge>
                </TableCell>
                <TableCell className='text-right'>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    disabled={props.disabled}
                    onClick={() => props.onSelect(item.id)}
                  >
                    {t('View')}
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}
