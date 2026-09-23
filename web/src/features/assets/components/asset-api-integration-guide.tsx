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
import { ShieldKeyIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

function ConnectionValue(props: {
  label: string
  value: string
  copyLabel: string
}) {
  return (
    <div className='grid min-w-0 gap-1'>
      <dt className='text-muted-foreground text-xs'>{props.label}</dt>
      <dd className='flex min-w-0 items-center gap-1'>
        <code className='bg-muted min-w-0 flex-1 truncate rounded-md px-2 py-1.5 text-xs'>
          {props.value}
        </code>
        <CopyButton value={props.value} aria-label={props.copyLabel} />
      </dd>
    </div>
  )
}

export function AssetApiIntegrationGuide() {
  const { t } = useTranslation()
  const origin = window.location.origin
  const officialEndpoint = `${origin}/`
  const compatibilityEndpoint = `${origin}/api/v3/`

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('Connection details')}</CardTitle>
        <CardDescription>
          {t(
            'Use the same HMAC-SHA256 V4 request format as the Volcengine Ark asset library.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='grid gap-4'>
        <dl className='grid gap-3 sm:grid-cols-2'>
          <ConnectionValue
            label={t('Official SDK endpoint')}
            value={officialEndpoint}
            copyLabel={t('Copy official SDK endpoint')}
          />
          <ConnectionValue
            label={t('Compatible request endpoint')}
            value={compatibilityEndpoint}
            copyLabel={t('Copy compatible request endpoint')}
          />
        </dl>

        <dl className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
          <div>
            <dt className='text-muted-foreground text-xs'>{t('Region')}</dt>
            <dd className='font-mono text-xs'>cn-beijing</dd>
          </div>
          <div>
            <dt className='text-muted-foreground text-xs'>{t('Service')}</dt>
            <dd className='font-mono text-xs'>ark</dd>
          </div>
          <div>
            <dt className='text-muted-foreground text-xs'>
              {t('API version')}
            </dt>
            <dd className='font-mono text-xs'>2024-01-01</dd>
          </div>
          <div>
            <dt className='text-muted-foreground text-xs'>
              {t('Authentication')}
            </dt>
            <dd className='font-mono text-xs'>HMAC-SHA256</dd>
          </div>
        </dl>

        <Alert>
          <HugeiconsIcon icon={ShieldKeyIcon} aria-hidden='true' />
          <AlertTitle>{t('Signing requirements')}</AlertTitle>
          <AlertDescription>
            {t(
              'Send Action and Version in the query string and JSON in the POST body. The canonical path used for signing must match the endpoint path exactly.'
            )}
          </AlertDescription>
        </Alert>

        <div className='grid gap-1 text-xs'>
          <p className='font-medium'>{t('Supported actions')}</p>
          <p className='text-muted-foreground break-words'>
            CreateAssetGroup · ListAssetGroups · GetAssetGroup ·
            UpdateAssetGroup · DeleteAssetGroup
          </p>
          <p className='text-muted-foreground break-words'>
            CreateAsset · ListAssets · GetAsset · UpdateAsset · DeleteAsset
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
