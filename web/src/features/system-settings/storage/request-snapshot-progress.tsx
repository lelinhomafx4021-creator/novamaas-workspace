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

import { Badge } from '@/components/ui/badge'
import { Spinner } from '@/components/ui/spinner'
import { formatTimestampToDate } from '@/lib/format'

import type { RequestSnapshotJob } from './types'

export function RequestSnapshotProgress(props: { job: RequestSnapshotJob }) {
  const { t } = useTranslation()
  let statusLabel = t('Idle')
  if (props.job.status === 'running') statusLabel = t('Running')
  if (props.job.status === 'completed') statusLabel = t('Completed')
  if (props.job.status === 'failed') statusLabel = t('Failed')

  let phaseLabel = props.job.phase || t('Phase unavailable')
  if (props.job.phase === 'legacy_tasks') phaseLabel = t('Historical tasks')
  if (props.job.phase === 'legacy_logs') phaseLabel = t('Historical logs')
  if (props.job.phase === 'archive_table') phaseLabel = t('Snapshot archive')
  if (props.job.status === 'running' && props.job.initialized === false) {
    phaseLabel = t('Preparing maintenance')
  }

  const rows = [
    [t('Current phase'), phaseLabel],
    [t('Scanned records'), props.job.scanned ?? '—'],
    [t('Processed snapshots'), props.job.processed],
    [t('Job errors'), props.job.failed],
    [t('Migration workers'), props.job.workers ?? '—'],
    [t('Active migration files'), props.job.active_files ?? '—'],
  ]
  const checkpoints = [
    [
      t('Task scan cursor'),
      `${props.job.task_cursor ?? '—'} / ${props.job.task_upper_id ?? '—'}`,
    ],
    [
      t('Log scan cursor'),
      props.job.log_cursor ? JSON.stringify(props.job.log_cursor) : '—',
    ],
    [
      t('Log scan upper cursor'),
      props.job.log_upper_cursor
        ? JSON.stringify(props.job.log_upper_cursor)
        : '—',
    ],
    [
      t('Archive cursor'),
      `${props.job.cursor} / ${props.job.archive_upper_id ?? '—'}`,
    ],
  ]

  return (
    <section
      role='status'
      aria-label={t('Snapshot maintenance progress')}
      className='flex min-w-0 flex-col gap-3 rounded-lg border p-3'
    >
      <div className='flex flex-wrap items-center gap-2'>
        <span className='text-sm font-medium'>
          {props.job.mode === 'cleanup'
            ? t('Database body cleanup')
            : t('Snapshot migration')}
        </span>
        <Badge
          variant={props.job.status === 'failed' ? 'destructive' : 'secondary'}
        >
          {props.job.status === 'running' && (
            <Spinner data-icon='inline-start' />
          )}
          {statusLabel}
        </Badge>
      </div>
      <dl className='grid min-w-0 gap-3 sm:grid-cols-3'>
        {rows.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{label}</dt>
            <dd className='text-sm break-all tabular-nums'>{value}</dd>
          </div>
        ))}
        <div>
          <dt className='text-muted-foreground text-xs'>
            {t('Progress updated')}
          </dt>
          <dd className='text-sm tabular-nums'>
            {props.job.updated_at ? (
              <time
                dateTime={new Date(props.job.updated_at * 1000).toISOString()}
              >
                {formatTimestampToDate(props.job.updated_at)}
              </time>
            ) : (
              '—'
            )}
          </dd>
        </div>
      </dl>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Scanned records include skipped rows. Processed snapshots count archived or cleaned bodies; cursor upper bounds are checkpoints, not totals.'
        )}
      </p>
      <dl className='grid min-w-0 gap-3 sm:grid-cols-2'>
        {checkpoints.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{label}</dt>
            <dd className='text-sm break-all tabular-nums'>{value}</dd>
          </div>
        ))}
      </dl>
      {props.job.error && (
        <p role='alert' className='text-destructive text-sm break-all'>
          {props.job.error}
        </p>
      )}
    </section>
  )
}
