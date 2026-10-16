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
  Area,
  AreaChart,
  CartesianGrid,
  Line,
  LineChart,
  XAxis,
  YAxis,
} from 'recharts'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import { toIntlLocale } from '@/i18n/languages'

import {
  buildResourceHistory,
  formatResourceBytes,
  formatResourceNumber,
} from '../lib/resource-monitor-format'
import type { ResourceHistoryPoint } from '../types'

export function ResourceMonitorCharts(props: {
  history: ResourceHistoryPoint[]
  minutes: number
}) {
  const { t, i18n } = useTranslation()
  const data = buildResourceHistory(props.history, props.minutes)
  const hasActivity = data.some(
    (point) => point.activeRequests !== null || point.connections !== null
  )
  const timeFormat = new Intl.DateTimeFormat(toIntlLocale(i18n.language), {
    ...(props.minutes >= 1440 ? { month: 'numeric', day: 'numeric' } : {}),
    hour: '2-digit',
    minute: '2-digit',
  })
  const networkConfig = {
    received: { label: t('Network received'), color: 'var(--chart-1)' },
    sent: { label: t('Network sent'), color: 'var(--chart-2)' },
  }
  const resourceConfig = {
    cpu: { label: t('Process CPU'), color: 'var(--chart-1)' },
    hostMemory: { label: t('Host memory'), color: 'var(--chart-2)' },
    rss: { label: t('Process memory'), color: 'var(--chart-4)' },
  }
  const activityConfig = {
    activeRequests: { label: t('Active requests'), color: 'var(--chart-1)' },
    connections: { label: t('Connections'), color: 'var(--chart-2)' },
  }

  return (
    <div className='grid min-w-0 gap-4 xl:grid-cols-2'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Network throughput')}</CardTitle>
          <CardDescription>
            {t('Host network interfaces · bytes per second')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer
            config={networkConfig}
            className='aspect-auto h-56 w-full'
            role='img'
            aria-label={t('Network throughput')}
          >
            <AreaChart
              data={data}
              margin={{ top: 8, right: 8, left: 0, bottom: 0 }}
            >
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey='timestamp'
                type='number'
                domain={['dataMin', 'dataMax']}
                tickFormatter={(value: number) =>
                  timeFormat.format(value * 1000)
                }
                minTickGap={40}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                tickFormatter={formatResourceBytes}
                width={70}
                tickLine={false}
                axisLine={false}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    labelFormatter={(_, payload) =>
                      timeFormat.format(
                        Number(payload[0]?.payload.timestamp) * 1000
                      )
                    }
                    formatter={(value, name) => (
                      <div className='flex min-w-40 justify-between gap-5'>
                        <span className='text-muted-foreground'>
                          {name === 'received'
                            ? t('Network received')
                            : t('Network sent')}
                        </span>
                        <span className='font-mono tabular-nums'>
                          {formatResourceBytes(Number(value))}/s
                        </span>
                      </div>
                    )}
                  />
                }
              />
              <Area
                type='linear'
                dataKey='received'
                stroke='var(--color-received)'
                fill='var(--color-received)'
                fillOpacity={0.08}
                strokeWidth={2}
                isAnimationActive={false}
                connectNulls={false}
              />
              <Area
                type='linear'
                dataKey='sent'
                stroke='var(--color-sent)'
                fill='var(--color-sent)'
                fillOpacity={0.05}
                strokeWidth={2}
                isAnimationActive={false}
                connectNulls={false}
              />
              <ChartLegend content={<ChartLegendContent />} />
            </AreaChart>
          </ChartContainer>
        </CardContent>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('CPU and memory')}</CardTitle>
          <CardDescription>
            {t('Process CPU, resident memory and host memory usage')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <ChartContainer
            config={resourceConfig}
            className='aspect-auto h-56 w-full'
            role='img'
            aria-label={t('CPU and memory')}
          >
            <LineChart
              data={data}
              margin={{ top: 8, right: 0, left: 0, bottom: 0 }}
            >
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey='timestamp'
                type='number'
                domain={['dataMin', 'dataMax']}
                tickFormatter={(value: number) =>
                  timeFormat.format(value * 1000)
                }
                minTickGap={40}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                yAxisId='percent'
                tickFormatter={(value: number) =>
                  `${formatResourceNumber(value)}%`
                }
                width={48}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                yAxisId='bytes'
                orientation='right'
                tickFormatter={formatResourceBytes}
                width={70}
                tickLine={false}
                axisLine={false}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    labelFormatter={(_, payload) =>
                      timeFormat.format(
                        Number(payload[0]?.payload.timestamp) * 1000
                      )
                    }
                    formatter={(value, name) => (
                      <div className='flex min-w-44 justify-between gap-5'>
                        <span className='text-muted-foreground'>
                          {
                            resourceConfig[name as keyof typeof resourceConfig]
                              ?.label
                          }
                        </span>
                        <span className='font-mono tabular-nums'>
                          {name === 'rss'
                            ? formatResourceBytes(Number(value))
                            : `${formatResourceNumber(Number(value))}%`}
                        </span>
                      </div>
                    )}
                  />
                }
              />
              <Line
                yAxisId='percent'
                type='linear'
                dataKey='cpu'
                stroke='var(--color-cpu)'
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
                connectNulls={false}
              />
              <Line
                yAxisId='percent'
                type='linear'
                dataKey='hostMemory'
                stroke='var(--color-hostMemory)'
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
                connectNulls={false}
              />
              <Line
                yAxisId='bytes'
                type='linear'
                dataKey='rss'
                stroke='var(--color-rss)'
                strokeWidth={2}
                dot={false}
                isAnimationActive={false}
                connectNulls={false}
              />
              <ChartLegend content={<ChartLegendContent />} />
            </LineChart>
          </ChartContainer>
        </CardContent>
      </Card>
      <Card className='xl:col-span-2'>
        <CardHeader>
          <CardTitle>{t('Requests and connections')}</CardTitle>
          <CardDescription>
            <span>{t('Sampled peaks per minute')}</span>
            {' · '}
            {t(
              'Five-second samples; wider history buckets preserve sampled peaks.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {hasActivity ? (
            <ChartContainer
              config={activityConfig}
              className='aspect-auto h-44 w-full'
              role='img'
              aria-label={t('Requests and connections')}
            >
              <LineChart
                data={data}
                margin={{ top: 8, right: 8, left: 0, bottom: 0 }}
              >
                <CartesianGrid vertical={false} />
                <XAxis
                  dataKey='timestamp'
                  type='number'
                  domain={['dataMin', 'dataMax']}
                  tickFormatter={(value: number) =>
                    timeFormat.format(value * 1000)
                  }
                  minTickGap={40}
                  tickLine={false}
                  axisLine={false}
                />
                <YAxis
                  allowDecimals={false}
                  domain={[0, 'auto']}
                  tickFormatter={formatResourceNumber}
                  width={48}
                  tickLine={false}
                  axisLine={false}
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      labelFormatter={(_, payload) =>
                        timeFormat.format(
                          Number(payload[0]?.payload.timestamp) * 1000
                        )
                      }
                      formatter={(value, name) => (
                        <div className='flex min-w-40 justify-between gap-5'>
                          <span className='text-muted-foreground'>
                            {name === 'activeRequests'
                              ? t('Active requests')
                              : t('Connections')}
                          </span>
                          <span className='font-mono tabular-nums'>
                            {formatResourceNumber(Number(value))}
                          </span>
                        </div>
                      )}
                    />
                  }
                />
                <Line
                  type='linear'
                  dataKey='activeRequests'
                  stroke='var(--color-activeRequests)'
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                  connectNulls={false}
                />
                <Line
                  type='linear'
                  dataKey='connections'
                  stroke='var(--color-connections)'
                  strokeWidth={2}
                  dot={false}
                  isAnimationActive={false}
                  connectNulls={false}
                />
                <ChartLegend content={<ChartLegendContent />} />
              </LineChart>
            </ChartContainer>
          ) : (
            <Empty className='h-44'>
              <EmptyHeader>
                <EmptyDescription>
                  {t('No connection history is available for this period.')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
