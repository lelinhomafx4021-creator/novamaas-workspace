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

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Spinner } from '@/components/ui/spinner'

import { testChannelNetwork } from '../../api'
import type { ChannelNetworkTestData } from '../../types'

function formatMilliseconds(value: number | undefined) {
  return value === undefined ? '—' : `${value} ms`
}

type ChannelNetworkTestPanelProps = {
  channelId: number
}

export function ChannelNetworkTestPanel(props: ChannelNetworkTestPanelProps) {
  const { t } = useTranslation()
  const [result, setResult] = useState<ChannelNetworkTestData>()
  const [error, setError] = useState('')
  const [isTesting, setIsTesting] = useState(false)

  const runTest = async () => {
    setIsTesting(true)
    setError('')
    try {
      const response = await testChannelNetwork(props.channelId)
      setResult(response.data)
      if (!response.success) {
        setError(response.message || t('Network test failed'))
      }
    } catch (requestError) {
      setResult(undefined)
      setError(
        requestError instanceof Error
          ? requestError.message
          : t('Network test failed')
      )
    } finally {
      setIsTesting(false)
    }
  }

  const metrics = result
    ? [
        [t('DNS lookup'), formatMilliseconds(result.dns_ms)],
        [t('TCP connection'), formatMilliseconds(result.connect_ms)],
        [t('TLS handshake'), formatMilliseconds(result.tls_ms)],
        [t('Time to first byte'), formatMilliseconds(result.ttfb_ms)],
        [t('Total network time'), formatMilliseconds(result.total_ms)],
      ]
    : []

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('Network diagnostics')}</CardTitle>
        <CardDescription>
          {t(
            'Tests the configured channel address without sending a model request.'
          )}
        </CardDescription>
        <CardAction>
          <Button size='sm' onClick={runTest} disabled={isTesting}>
            {isTesting ? <Spinner /> : null}
            {t('Test network latency')}
          </Button>
        </CardAction>
      </CardHeader>
      {error ? (
        <CardContent>
          <Alert variant='destructive'>
            <AlertTitle>{t('Network test failed')}</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </CardContent>
      ) : null}
      {result ? (
        <CardContent className='space-y-3'>
          <div className='grid gap-2 sm:grid-cols-2'>
            <div className='bg-muted/40 rounded-lg px-3 py-2 sm:col-span-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Channel address')}
              </div>
              <div className='mt-0.5 font-mono text-xs break-all'>
                {result.target_url}
              </div>
            </div>
            <div className='bg-muted/40 rounded-lg px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('Remote address')}
              </div>
              <div className='mt-0.5 font-mono text-xs break-all'>
                {result.remote_address || '—'}
              </div>
            </div>
            <div className='bg-muted/40 rounded-lg px-3 py-2'>
              <div className='text-muted-foreground text-xs'>
                {t('HTTP status')}
              </div>
              <div className='mt-0.5 font-mono text-xs'>
                {result.http_status || '—'} {result.protocol || ''}
              </div>
            </div>
          </div>
          <div className='grid grid-cols-2 gap-2 sm:grid-cols-5'>
            {metrics.map(([label, value]) => (
              <div
                key={label}
                className='border-border/60 rounded-lg border p-2'
              >
                <div className='text-muted-foreground text-[11px]'>{label}</div>
                <div className='mt-0.5 font-mono text-xs tabular-nums'>
                  {value}
                </div>
              </div>
            ))}
          </div>
          <div className='text-muted-foreground text-xs'>
            {t('Resolved addresses')}:&nbsp;
            <span className='font-mono'>
              {result.resolved_addresses?.join(', ') || '—'}
            </span>
            {' · '}
            {t('Via proxy')}: {result.via_proxy ? t('Yes') : t('No')}
            {' · '}
            {t('Connection reused')}:{' '}
            {result.connection_reused ? t('Yes') : t('No')}
          </div>
        </CardContent>
      ) : null}
    </Card>
  )
}
