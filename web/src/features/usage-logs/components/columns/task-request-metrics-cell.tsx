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
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import {
  formatTaskRequestBytes,
  formatTaskRequestDuration,
} from '../../lib/task-request-metrics-format'
import type { TaskRequestMetrics } from '../../types'

type TaskRequestMetricsCellProps = {
  metrics?: TaskRequestMetrics
}

export function TaskRequestMetricsCell(props: TaskRequestMetricsCellProps) {
  const { t } = useTranslation()

  if (!props.metrics) {
    return <span className='text-muted-foreground/60 text-xs'>-</span>
  }

  const requestBodySize = formatTaskRequestBytes(
    props.metrics.request_body_bytes
  )
  const totalDuration = formatTaskRequestDuration(props.metrics.total_ms)
  const details = [
    [t('Request body'), requestBodySize],
    [
      t('Upstream body'),
      formatTaskRequestBytes(props.metrics.upstream_body_bytes),
    ],
    [t('Body read'), formatTaskRequestDuration(props.metrics.body_read_ms)],
    [
      t('Request preparation'),
      formatTaskRequestDuration(props.metrics.request_preparation_ms),
    ],
    [
      t('Temporary storage'),
      formatTaskRequestDuration(props.metrics.temporary_storage_ms),
    ],
    [
      t('Upstream request'),
      formatTaskRequestDuration(props.metrics.upstream_request_ms),
    ],
    [t('Total request'), totalDuration],
    [t('Attempts'), String(props.metrics.attempts)],
  ]

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type='button'
            className='flex max-w-[160px] flex-col text-left'
          />
        }
      >
        <span className='font-mono text-xs font-medium tabular-nums'>
          {totalDuration}
        </span>
        <span className='text-muted-foreground truncate text-[11px]'>
          {t('Request body')}: {requestBodySize}
        </span>
      </TooltipTrigger>
      <TooltipContent className='block min-w-56'>
        <div className='grid grid-cols-[auto_1fr] gap-x-4 gap-y-1'>
          {details.map(([label, value]) => (
            <div key={label} className='contents'>
              <span className='text-background/70'>{label}</span>
              <span className='text-right font-mono tabular-nums'>{value}</span>
            </div>
          ))}
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
