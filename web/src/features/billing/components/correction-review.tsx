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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
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
import { CorrectionPricingEvidence } from './correction-pricing-evidence'
import { CorrectionWalletChange } from './correction-wallet-change'

export function CorrectionReview(props: { batch: CorrectionBatch }) {
  const { t } = useTranslation()
  const [page, setPage] = useState(0)
  const batch = props.batch
  const rows = batch.rows.slice(page * 50, (page + 1) * 50)
  function blocked(reason: string) {
    switch (reason) {
      case 'already_corrected':
        return t('Already adjusted')
      case 'missing_task_evidence':
        return t('Historical task evidence is missing')
      case 'unsupported_or_unfinished_task':
        return t('Unsupported or unfinished task')
      case 'missing_pricing_evidence':
      case 'missing_current_pricing_evidence':
        return t('Saved resolution or billing usage is missing')
      case 'original_price_mismatch':
        return t('Original price could not be verified')
      case 'incomplete_task_range':
        return t('Include the full consumption and refund lifecycle')
      default:
        return reason ? t('Cannot safely recalculate this record') : t('Ready')
    }
  }
  function download() {
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(batch, null, 2)], { type: 'application/json' })
    )
    const link = document.createElement('a')
    link.href = url
    link.download = `billing-correction-${batch.id}.json`
    link.click()
    URL.revokeObjectURL(url)
  }
  return (
    <section className='flex flex-col gap-3 rounded-lg border p-4'>
      <p>
        {t('Account')} #{batch.user_id} ·{' '}
        {batch.mode === 'model_pricing'
          ? t('Model pricing adjustment')
          : t('Billing group adjustment')}{' '}
        · {t('Records')}: {batch.rows.length}
      </p>
      {batch.mode === 'model_pricing' ? (
        <p className='text-muted-foreground text-sm'>
          {t(
            'Recalculate selected models using current pricing and existing billing groups. Resolution and input-video conditions come from saved task evidence.'
          )}
        </p>
      ) : (
        <p className='text-sm'>
          {batch.target_group} / {(Number(batch.target_rate) * 100).toFixed(2)}%
        </p>
      )}
      <p className='text-muted-foreground text-sm'>
        {t(
          'Positive net differences deduct wallet balance; negative net differences refund wallet balance. Previewing does not move money.'
        )}
      </p>
      <dl className='grid gap-3 sm:grid-cols-3'>
        <div>
          <dt>{t('Consumption adjustment')}</dt>
          <dd>
            {formatQuotaWithCurrency(batch.charge_delta, {
              digitsLarge: 6,
              digitsSmall: 6,
              abbreviate: false,
            })}
          </dd>
        </div>
        <div>
          <dt>{t('Refund adjustment')}</dt>
          <dd>
            {formatQuotaWithCurrency(batch.refund_delta, {
              digitsLarge: 6,
              digitsSmall: 6,
              abbreviate: false,
            })}
          </dd>
        </div>
        <div>
          <dt>{t('Net debit (negative means credit)')}</dt>
          <dd className='font-semibold'>
            {formatQuotaWithCurrency(batch.net_delta, {
              digitsLarge: 6,
              digitsSmall: 6,
              abbreviate: false,
            })}
          </dd>
        </div>
      </dl>
      <CorrectionWalletChange batch={batch} />
      <p className='text-sm'>
        {billingTimestamp(batch.start_at)} — {billingTimestamp(batch.end_at)}{' '}
        (Asia/Shanghai)
      </p>
      <p className='text-sm break-words'>{batch.reason}</p>
      {batch.rows.length > 0 && (
        <p className='text-sm'>
          {t('Affected billing months')}:{' '}
          {[
            ...new Set(
              batch.rows.map((row) =>
                new Date((row.posted_at + 8 * 3600) * 1000)
                  .toISOString()
                  .slice(0, 7)
              )
            ),
          ]
            .sort()
            .join(', ')}
        </p>
      )}
      <p className='text-muted-foreground text-sm'>
        {t(
          'Void an affected statement and create a new version to include this adjustment. Historical import evidence remains available.'
        )}
      </p>
      <p className='font-mono text-xs break-all'>{batch.sha256}</p>
      {batch.status === 'preview' && (
        <p>{t('Preview expires in 15 minutes and does not move money.')}</p>
      )}
      {batch.status !== 'preview' && (
        <p>
          {batch.status === 'applied'
            ? t('Adjustment applied')
            : t('Adjustment reversed')}
        </p>
      )}
      {!batch.can_apply && (
        <p role='alert'>
          {t('Blocked records or no amount differences prevent execution.')}
        </p>
      )}
      <div className='max-h-96 overflow-auto'>
        <Table aria-label={t('Adjustment records')} className='min-w-[72rem]'>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Model')}</TableHead>
              <TableHead>{t('Date')}</TableHead>
              <TableHead>{t('Original billing group')}</TableHead>
              <TableHead>{t('Original amount')}</TableHead>
              <TableHead>{t('Current effective amount')}</TableHead>
              <TableHead>{t('Corrected amount')}</TableHead>
              <TableHead>{t('Difference')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead>{t('Pricing evidence')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.source_entry_id}>
                <TableCell className='max-w-64 break-all whitespace-normal'>
                  {row.model_name}
                  <span className='text-muted-foreground block text-xs'>
                    #{row.source_entry_id}
                  </span>
                </TableCell>
                <TableCell>{billingTimestamp(row.posted_at)}</TableCell>
                <TableCell>
                  {row.original_group} /{' '}
                  {(Number(row.original_rate) * 100).toFixed(2)}%
                  {row.effective_group && (
                    <span className='text-muted-foreground block text-xs'>
                      {t('Current')}: {row.effective_group} /{' '}
                      {(Number(row.effective_rate) * 100).toFixed(2)}%
                    </span>
                  )}
                </TableCell>
                <TableCell>
                  {formatQuotaWithCurrency(row.original_quota, {
                    digitsLarge: 6,
                    digitsSmall: 6,
                    abbreviate: false,
                  })}
                </TableCell>
                <TableCell>
                  {formatQuotaWithCurrency(
                    batch.mode
                      ? (row.effective_quota ?? 0)
                      : row.original_quota,
                    {
                      digitsLarge: 6,
                      digitsSmall: 6,
                      abbreviate: false,
                    }
                  )}
                </TableCell>
                <TableCell>
                  {formatQuotaWithCurrency(row.corrected_quota, {
                    digitsLarge: 6,
                    digitsSmall: 6,
                    abbreviate: false,
                  })}
                </TableCell>
                <TableCell>
                  {formatQuotaWithCurrency(row.delta, {
                    digitsLarge: 6,
                    digitsSmall: 6,
                    abbreviate: false,
                  })}
                </TableCell>
                <TableCell className='whitespace-normal'>
                  {blocked(row.blocked)}
                </TableCell>
                <TableCell className='max-w-80 min-w-64 whitespace-normal'>
                  <CorrectionPricingEvidence value={row.target_pricing} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          disabled={page === 0}
          onClick={() => setPage(page - 1)}
        >
          {t('Previous')}
        </Button>
        <span>
          {page + 1} / {Math.max(1, Math.ceil(batch.rows.length / 50))}
        </span>
        <Button
          type='button'
          variant='outline'
          disabled={(page + 1) * 50 >= batch.rows.length}
          onClick={() => setPage(page + 1)}
        >
          {t('Next')}
        </Button>
        <Button type='button' variant='outline' onClick={download}>
          {t('Download adjustment evidence')}
        </Button>
      </div>
    </section>
  )
}
