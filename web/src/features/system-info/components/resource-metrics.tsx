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
import {
  Activity,
  ArrowDown,
  ArrowUp,
  Cpu,
  HardDrive,
  Network,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from '@/components/ui/card'
import { toIntlLocale } from '@/i18n/languages'

import {
  formatResourceBytes,
  formatResourceNumber,
} from '../lib/resource-monitor-format'
import type { ResourceMonitorReport } from '../types'

function ResourceMetric(props: {
  label: string
  icon: ReactNode
  value: ReactNode
  detail: ReactNode
}) {
  return (
    <Card size='sm'>
      <CardHeader>
        <CardDescription className='flex items-center justify-between gap-2'>
          {props.label}
          <span
            className='text-muted-foreground [&_svg]:size-4'
            aria-hidden='true'
          >
            {props.icon}
          </span>
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-2'>
        <div className='text-2xl font-semibold tracking-tight tabular-nums'>
          {props.value}
        </div>
        <div className='text-muted-foreground text-xs leading-relaxed'>
          {props.detail}
        </div>
      </CardContent>
    </Card>
  )
}

export function ResourceMetrics(props: { report: ResourceMonitorReport }) {
  const { t, i18n } = useTranslation()
  const report = props.report
  const numberFormat = new Intl.NumberFormat(toIntlLocale(i18n.language), {
    maximumFractionDigits: 1,
  })
  const percent = (value: number) => `${numberFormat.format(value)}%`
  return (
    <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
      <ResourceMetric
        label={t('Process CPU')}
        icon={<Cpu />}
        value={
          report.current.process.cpu_available
            ? percent(report.current.process.cpu_percent)
            : t('Unavailable')
        }
        detail={
          <>
            {t('Host CPU')}:{' '}
            {report.current.host.cpu_available
              ? percent(report.current.host.cpu_percent)
              : '—'}
          </>
        }
      />
      <ResourceMetric
        label={t('Process memory')}
        icon={<HardDrive />}
        value={
          report.current.process.memory_available
            ? formatResourceBytes(report.current.process.rss_bytes)
            : t('Unavailable')
        }
        detail={
          <>
            {t('Host memory')}:{' '}
            {report.current.host.memory_available
              ? `${percent(report.current.host.memory_percent)} · ${formatResourceBytes(report.current.host.memory_used_bytes)} / ${formatResourceBytes(report.current.host.memory_total_bytes)}`
              : '—'}
          </>
        }
      />
      <ResourceMetric
        label={t('Host network')}
        icon={<Network />}
        value={
          report.current.network.available ? (
            <div className='flex flex-wrap items-center gap-x-3 gap-y-1 text-lg'>
              <span className='inline-flex items-center gap-1'>
                <ArrowDown className='text-chart-1 size-4' aria-hidden='true' />
                {formatResourceBytes(
                  report.current.network.received_bytes_per_second
                )}
                <span className='text-muted-foreground text-xs'>/s</span>
              </span>
              <span className='inline-flex items-center gap-1'>
                <ArrowUp className='text-chart-2 size-4' aria-hidden='true' />
                {formatResourceBytes(
                  report.current.network.sent_bytes_per_second
                )}
                <span className='text-muted-foreground text-xs'>/s</span>
              </span>
            </div>
          ) : (
            t('Unavailable')
          )
        }
        detail={
          <>
            {t('Packets received / sent')}:{' '}
            {report.current.network.available
              ? `${formatResourceNumber(report.current.network.received_packets_per_second)} / ${formatResourceNumber(report.current.network.sent_packets_per_second)}`
              : '—'}{' '}
            /s
          </>
        }
      />
      <ResourceMetric
        label={t('Active requests')}
        icon={<Activity />}
        value={formatResourceNumber(
          report.traffic
            .filter((row) => row.scope === 'inbound')
            .reduce((sum, row) => sum + row.active, 0)
        )}
        detail={
          <>
            {t('Application connections')}:{' '}
            {formatResourceNumber(
              report.traffic.reduce((sum, row) => sum + row.connections, 0)
            )}
          </>
        }
      />
    </div>
  )
}
