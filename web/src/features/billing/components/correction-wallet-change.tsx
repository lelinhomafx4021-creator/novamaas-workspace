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

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { formatQuotaWithCurrency } from '@/lib/currency'

import type { CorrectionBatch } from '../correction-api'

export function CorrectionWalletChange(props: { batch: CorrectionBatch }) {
  const { t } = useTranslation()
  const delta =
    props.batch.status === 'reversed'
      ? -props.batch.net_delta
      : props.batch.net_delta
  const amount = formatQuotaWithCurrency(Math.abs(delta), {
    digitsLarge: 6,
    digitsSmall: 6,
    abbreviate: false,
  })
  const preview = props.batch.status === 'preview'
  let title = t('Wallet balance unchanged')
  let description = preview
    ? t('The wallet balance will remain unchanged after confirmation.')
    : t('The wallet balance was unchanged.')
  if (delta > 0) {
    title = t('Additional wallet charge')
    description = preview
      ? t('{{amount}} will be deducted from the wallet after confirmation.', {
          amount,
        })
      : t('{{amount}} was deducted from the wallet.', { amount })
  } else if (delta < 0) {
    title = t('Wallet refund')
    description = preview
      ? t('{{amount}} will be returned to the wallet after confirmation.', {
          amount,
        })
      : t('{{amount}} was returned to the wallet.', { amount })
  }
  return (
    <Alert role='status' aria-label={t('Wallet balance change')}>
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>{description}</AlertDescription>
    </Alert>
  )
}
