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

import { RequestSnapshotProgress } from './request-snapshot-progress'
import type { RequestSnapshotStatus } from './types'

export function RequestSnapshotNodeStatus(props: {
  status: RequestSnapshotStatus
}) {
  const { t } = useTranslation()
  const rows = [
    [t('Current node'), props.status.node_name || '—'],
    [t('Local spool directory'), props.status.spool_directory || '—'],
    [t('Upload workers'), props.status.workers],
    [t('Queued snapshots'), props.status.pending_files],
    [
      t('Queued data (MiB)'),
      (props.status.pending_bytes / 1024 / 1024).toFixed(2),
    ],
  ]

  return (
    <div className='flex min-w-0 flex-col gap-4'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Queue status belongs to the current node. Configure its spool directory and workers with SNAPSHOT_SPOOL_DIR and SNAPSHOT_WORKERS.'
        )}
      </p>
      <dl className='grid min-w-0 gap-3 sm:grid-cols-2'>
        {rows.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{label}</dt>
            <dd className='mt-1 text-sm break-all tabular-nums'>{value}</dd>
          </div>
        ))}
      </dl>
      {props.status.last_error && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Last snapshot upload error')}</AlertTitle>
          <AlertDescription className='break-all'>
            {props.status.last_error}
          </AlertDescription>
        </Alert>
      )}
      <RequestSnapshotProgress job={props.status.migration} />
    </div>
  )
}
