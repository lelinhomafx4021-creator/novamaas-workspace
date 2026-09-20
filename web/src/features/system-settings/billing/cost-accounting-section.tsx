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
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'

import { SettingsSection } from '../components/settings-section'
import { CostAccountingBackfillCard } from './cost-accounting-backfill-card'
import { CostAccountingRepriceCard } from './cost-accounting-reprice-card'
import type {
  CostAccountingBackfillValues,
  CostAccountingRepriceValues,
} from './cost-accounting-schema'

export function CostAccountingSection() {
  const { t } = useTranslation()
  const repriceCardRef = useRef<HTMLDivElement>(null)
  const [repriceDraft, setRepriceDraft] = useState<{
    revision: number
    values?: CostAccountingRepriceValues
  }>({ revision: 0 })

  const moveExistingSnapshotsToReprice = (
    values: CostAccountingBackfillValues
  ) => {
    setRepriceDraft((current) => ({
      revision: current.revision + 1,
      values: {
        start_time: values.start_time,
        end_time: values.end_time,
        channel_id: values.channel_id,
        cost_discount: values.cost_discount,
        limit: Math.min(values.limit, 1000),
        reason: '',
      },
    }))
    repriceCardRef.current?.focus({ preventScroll: true })
    repriceCardRef.current?.scrollIntoView?.({
      behavior: 'smooth',
      block: 'start',
    })
  }

  return (
    <SettingsSection title={t('Financial accounting')}>
      <Alert>
        <AlertTitle>{t('Accounting-only operations')}</AlertTitle>
        <AlertDescription>
          {t(
            'These tools only create cost snapshots or cost adjustments. They do not change customer prices, wallet balances, or usage logs.'
          )}{' '}
          {t(
            'All times are interpreted in Asia/Shanghai. Always preview and verify a batch before applying it.'
          )}
        </AlertDescription>
      </Alert>
      <CostAccountingBackfillCard
        onRepriceExisting={moveExistingSnapshotsToReprice}
      />
      <div
        ref={repriceCardRef}
        tabIndex={-1}
        className='focus-visible:ring-ring scroll-mt-4 rounded-xl outline-none focus-visible:ring-2'
      >
        <CostAccountingRepriceCard
          key={repriceDraft.revision}
          initialValues={repriceDraft.values}
        />
      </div>
    </SettingsSection>
  )
}
