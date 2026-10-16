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
import { useQuery } from '@tanstack/react-query'
import { Activity, Clock3, RefreshCw } from 'lucide-react'
import { lazy, Suspense, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'

import { getResourceMonitor } from '../api'
import { formatResourceNumber } from '../lib/resource-monitor-format'
import type { ResourceMonitorPeriod } from '../types'
import { ResourceMetrics } from './resource-metrics'
import { ResourceTrafficPanel } from './resource-traffic-panel'

const ResourceMonitorCharts = lazy(() =>
  import('./resource-monitor-charts').then((module) => ({
    default: module.ResourceMonitorCharts,
  }))
)
const PERIODS = [60, 360, 1440, 10080] as const
const PERIOD_LABELS: Record<ResourceMonitorPeriod, string> = {
  60: '1 hour',
  360: '6 hours',
  1440: '24 hours',
  10080: '7 days',
}

export function SystemResourceMonitor(props: { active: boolean }) {
  const { t, i18n } = useTranslation()
  const [minutes, setMinutes] = useState<ResourceMonitorPeriod>(60)
  const [retentionHours, setRetentionHours] = useState(24)
  const [visible, setVisible] = useState(
    () => document.visibilityState === 'visible'
  )
  useEffect(() => {
    const onVisibilityChange = () =>
      setVisible(document.visibilityState === 'visible')
    document.addEventListener('visibilitychange', onVisibilityChange)
    return () =>
      document.removeEventListener('visibilitychange', onVisibilityChange)
  }, [])
  const enabled = props.active && visible
  const query = useQuery({
    queryKey: ['system-resource-monitor', minutes],
    queryFn: ({ signal }) => getResourceMonitor(minutes, signal),
    enabled,
    staleTime: 10_000,
    refetchInterval: enabled ? 15_000 : false,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const report = query.data
  useEffect(() => {
    if (!report) return
    const hours = Math.max(1, report.meta.retention_hours)
    setRetentionHours(hours)
    setMinutes((selected) => {
      if (selected <= hours * 60) return selected
      let supported: ResourceMonitorPeriod = 60
      for (const period of PERIODS) {
        if (period > hours * 60) break
        supported = period
      }
      return supported
    })
  }, [report])

  return (
    <section
      className='flex min-w-0 flex-col gap-5'
      aria-label={t('Resource monitoring')}
    >
      <div className='flex flex-wrap items-start justify-between gap-4'>
        <div className='flex min-w-0 flex-col gap-1.5'>
          <div className='flex flex-wrap items-center gap-2'>
            <h2 className='text-lg font-semibold'>
              {t('Resource monitoring')}
            </h2>
            {report && <Badge variant='outline'>{report.meta.node_name}</Badge>}
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('Current node hardware and application traffic')}
          </p>
          {report && (
            <p className='text-muted-foreground text-xs'>
              {report.meta.hostname} · {report.meta.os}/{report.meta.arch} ·{' '}
              {t('{{count}} CPU cores', { count: report.meta.cpu_cores })}
            </p>
          )}
        </div>
        <div className='flex flex-wrap items-center gap-2'>
          <div
            className='bg-muted flex rounded-lg p-0.5'
            role='group'
            aria-label={t('History period')}
          >
            {PERIODS.filter((period) => period <= retentionHours * 60).map(
              (period) => (
                <Button
                  key={period}
                  size='sm'
                  variant={minutes === period ? 'secondary' : 'ghost'}
                  aria-pressed={minutes === period}
                  onClick={() => setMinutes(period)}
                >
                  {t(PERIOD_LABELS[period])}
                </Button>
              )
            )}
          </div>
          <Button
            variant='outline'
            size='icon-sm'
            disabled={!enabled || query.isFetching}
            onClick={() => query.refetch()}
            aria-label={t('Refresh resource monitoring')}
          >
            <RefreshCw aria-hidden='true' />
          </Button>
        </div>
      </div>

      {query.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Unable to load resource monitoring')}</AlertTitle>
          <AlertDescription>
            {t('Check the node connection and try again.')}
          </AlertDescription>
          <Button
            variant='outline'
            size='sm'
            onClick={() => query.refetch()}
            disabled={!enabled || query.isFetching}
          >
            {t('Retry')}
          </Button>
        </Alert>
      )}

      {!report && query.isLoading && (
        <div
          className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'
          aria-label={t('Loading resource monitoring')}
        >
          {['cpu', 'memory', 'network', 'activity'].map((key) => (
            <Skeleton key={key} className='h-32 rounded-xl' />
          ))}
        </div>
      )}

      {report && !report.meta.started && (
        <Alert>
          <Activity aria-hidden='true' />
          <AlertTitle>
            {t('Resource monitoring is not running on this node.')}
          </AlertTitle>
        </Alert>
      )}
      {report?.meta.started && (
        <>
          <ResourceMetrics report={report} />

          {report.history.length < 2 ? (
            <Card>
              <Empty className='min-h-52'>
                <EmptyHeader>
                  <EmptyMedia variant='icon'>
                    <Clock3 />
                  </EmptyMedia>
                  <EmptyTitle>{t('Collecting resource history')}</EmptyTitle>
                  <EmptyDescription>
                    {t(
                      'Charts appear after two samples. Unavailable metrics remain gaps.'
                    )}
                  </EmptyDescription>
                </EmptyHeader>
              </Empty>
            </Card>
          ) : (
            <Suspense
              fallback={
                <div className='grid gap-4 xl:grid-cols-2'>
                  <Skeleton className='h-80 rounded-xl' />
                  <Skeleton className='h-80 rounded-xl' />
                </div>
              }
            >
              <ResourceMonitorCharts
                history={report.history}
                minutes={report.meta.requested_minutes}
              />
            </Suspense>
          )}

          <ResourceTrafficPanel traffic={report.traffic} />

          <div className='text-muted-foreground flex flex-wrap items-start justify-between gap-x-6 gap-y-2 text-xs leading-relaxed'>
            <p className='max-w-3xl'>
              {t(
                'Host network includes other processes and protocol overhead. It uses a different scope from application traffic; these values cannot be added or used as billing totals.'
              )}{' '}
              {t(
                'Network rates sum selected OS interfaces; virtual interfaces may overlap.'
              )}
            </p>
            <div className='flex shrink-0 flex-col gap-1 sm:items-end'>
              <span>
                {t('Network errors / drops per second')}:{' '}
                {report.current.network.available
                  ? `${formatResourceNumber(report.current.network.receive_errors_per_second + report.current.network.send_errors_per_second)} / ${formatResourceNumber(report.current.network.receive_drops_per_second + report.current.network.send_drops_per_second)}`
                  : '—'}
              </span>
              <span>
                {enabled
                  ? t('Refreshes every 15 seconds while visible')
                  : t('Automatic refresh paused')}
              </span>
            </div>
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'History is stored only in memory and is cleared when this node restarts.'
            )}
          </p>
          <p className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'>
            <span>
              {t(
                'Samples every {{seconds}} seconds · minute history · memory window up to {{hours}} h',
                {
                  seconds: report.meta.sample_interval_seconds,
                  hours: report.meta.retention_hours,
                }
              )}
            </span>
            <span>
              {t('Last sample')}:{' '}
              {report.meta.sampled_at > 0
                ? new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
                    dateStyle: 'short',
                    timeStyle: 'medium',
                  }).format(report.meta.sampled_at * 1000)
                : '—'}
            </span>
          </p>
        </>
      )}
      {!report && !query.isLoading && !query.isError && !enabled && (
        <p className='text-muted-foreground text-sm'>
          {t('Automatic refresh paused')}
        </p>
      )}
    </section>
  )
}
