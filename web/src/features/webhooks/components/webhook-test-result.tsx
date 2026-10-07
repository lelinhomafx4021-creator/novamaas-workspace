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

import type { WebhookProbeResult } from '../types'

export function WebhookTestResult(props: { result: WebhookProbeResult }) {
  const { t } = useTranslation()
  return (
    <Alert variant={props.result.reachable ? 'default' : 'destructive'}>
      <AlertTitle>
        {props.result.reachable
          ? t('Webhook test succeeded')
          : t('Webhook test failed')}
      </AlertTitle>
      <AlertDescription>
        {t('HTTP {{status}} · {{duration}} ms', {
          status: props.result.http_status || t('No response'),
          duration: props.result.duration_ms,
        })}
        {!props.result.reachable && (
          <p>
            {t(
              'Check the callback address, network policy and receiving service.'
            )}
          </p>
        )}
      </AlertDescription>
    </Alert>
  )
}
