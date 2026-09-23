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
import { Delete02Icon, Key01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { formatTimestampToDate } from '@/lib/format'

import { assetErrorMessage } from '../asset-utils'
import type { AssetAccessKey, CreatedAssetAccessKey } from '../types'

export function CreatedAssetAccessKeyPanel(props: {
  accessKey: CreatedAssetAccessKey
  onSaved: () => void
}) {
  const { t } = useTranslation()

  return (
    <section
      aria-labelledby='created-asset-access-key-title'
      className='border-warning/40 bg-warning/5 grid gap-3 rounded-xl border p-3'
    >
      <div className='grid gap-1'>
        <h3 id='created-asset-access-key-title' className='font-medium'>
          {t('Save your Secret Access Key now')}
        </h3>
        <p className='text-muted-foreground text-sm'>
          {t(
            'This secret is displayed only once. Copy both values before continuing.'
          )}
        </p>
      </div>
      <dl className='grid gap-3 sm:grid-cols-2'>
        <div className='grid min-w-0 gap-1'>
          <dt className='text-xs font-medium'>
            {t('Access Key ID (Access Key)')}
          </dt>
          <dd className='flex min-w-0 items-center gap-1'>
            <code className='bg-background min-w-0 flex-1 truncate rounded-md border px-2 py-1.5 text-xs'>
              {props.accessKey.access_key_id}
            </code>
            <CopyButton
              value={props.accessKey.access_key_id}
              aria-label={t('Copy Access Key ID')}
            />
          </dd>
        </div>
        <div className='grid min-w-0 gap-1'>
          <dt className='text-xs font-medium'>
            {t('Secret Access Key (Secret Key)')}
          </dt>
          <dd className='flex min-w-0 items-center gap-1'>
            <code className='bg-background min-w-0 flex-1 truncate rounded-md border px-2 py-1.5 text-xs'>
              {props.accessKey.secret_access_key}
            </code>
            <CopyButton
              value={props.accessKey.secret_access_key}
              aria-label={t('Copy Secret Access Key')}
            />
          </dd>
        </div>
      </dl>
      <Button size='sm' className='justify-self-start' onClick={props.onSaved}>
        {t('I have saved the credentials')}
      </Button>
    </section>
  )
}

export function AssetAccessKeyList(props: {
  keys: AssetAccessKey[]
  loading: boolean
  error: unknown
  onRevoke: (key: AssetAccessKey) => void
}) {
  const { t } = useTranslation()

  if (props.loading) {
    return (
      <div className='grid gap-2' aria-label={t('Loading access keys')}>
        <Skeleton className='h-16' />
        <Skeleton className='h-16' />
      </div>
    )
  }
  if (props.error) {
    return (
      <p className='text-destructive py-4 text-center text-sm'>
        {assetErrorMessage(props.error)}
      </p>
    )
  }
  if (props.keys.length === 0) {
    return (
      <Empty className='min-h-32 border'>
        <EmptyHeader>
          <EmptyMedia variant='icon'>
            <HugeiconsIcon icon={Key01Icon} />
          </EmptyMedia>
          <EmptyTitle>{t('No access keys yet')}</EmptyTitle>
          <EmptyDescription>
            {t('Create a key above to connect a downstream client.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <div className='grid gap-2'>
      {props.keys.map((key) => (
        <div
          key={key.id}
          className='grid gap-2 rounded-xl border p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'
        >
          <div className='min-w-0'>
            <p className='truncate text-sm font-medium'>{key.name}</p>
            <div className='flex min-w-0 items-center gap-1'>
              <code className='text-muted-foreground truncate text-xs'>
                {key.access_key_id}
              </code>
              <CopyButton
                value={key.access_key_id}
                size='icon'
                aria-label={t('Copy Access Key ID')}
              />
            </div>
            <p className='text-muted-foreground text-xs'>
              {t('Secret ending in {{hint}}', { hint: key.secret_hint })} ·{' '}
              {t('Created')} {formatTimestampToDate(key.created_at)} ·{' '}
              {t('Last used:')}{' '}
              {key.last_used_at
                ? formatTimestampToDate(key.last_used_at)
                : t('Never')}
            </p>
          </div>
          <Button
            size='icon-sm'
            variant='ghost'
            className='text-destructive justify-self-end'
            aria-label={t('Revoke {{name}}', { name: key.name })}
            onClick={() => props.onRevoke(key)}
          >
            <HugeiconsIcon icon={Delete02Icon} />
          </Button>
        </div>
      ))}
    </div>
  )
}
