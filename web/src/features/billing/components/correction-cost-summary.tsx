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

import type { CorrectionBatch } from '../correction-api'
import {
  formatCorrectionAmount,
  getCorrectionCostDelta,
} from '../lib/correction-amount'

export function CorrectionCostSummary(props: { batch: CorrectionBatch }) {
  const { t } = useTranslation()
  const batch = props.batch
  const costDelta = getCorrectionCostDelta(batch)
  if (batch.current_cost_quota == null || batch.corrected_cost_quota == null) {
    return (
      <p className='text-muted-foreground text-sm'>
        {t('Platform costs are not included in this batch.')}
      </p>
    )
  }
  const reversed = batch.status === 'reversed'
  const direction = reversed ? -1 : 1
  const currentCost = reversed
    ? batch.corrected_cost_quota
    : batch.current_cost_quota
  const correctedCost = reversed
    ? batch.current_cost_quota
    : batch.corrected_cost_quota
  const title = reversed
    ? t('Reversal cost changes')
    : t('Platform cost changes')
  return (
    <section
      aria-label={title}
      className='flex flex-col gap-2 rounded-lg border p-3'
    >
      <h3 className='font-semibold'>{title}</h3>
      <dl className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <div>
          <dt>{t('Current platform cost')}</dt>
          <dd>{formatCorrectionAmount(currentCost)}</dd>
        </div>
        <div>
          <dt>{t('Recalculated platform cost')}</dt>
          <dd>{formatCorrectionAmount(correctedCost)}</dd>
        </div>
        <div>
          <dt>{t('Platform cost change')}</dt>
          <dd>{formatCorrectionAmount(costDelta, direction)}</dd>
        </div>
        <div>
          <dt>{t('Profit change')}</dt>
          <dd>
            {formatCorrectionAmount(
              batch.net_delta - (costDelta ?? 0),
              direction
            )}
          </dd>
        </div>
      </dl>
      <p className='text-muted-foreground text-sm'>
        {t('Cost corrections do not add wallet charges.')}
      </p>
      <p className='text-muted-foreground text-sm'>
        {t('These amounts are frozen for this adjustment batch.')}
      </p>
    </section>
  )
}
