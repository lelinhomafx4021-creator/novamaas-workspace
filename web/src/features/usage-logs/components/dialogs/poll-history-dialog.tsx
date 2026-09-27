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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import {
  getTaskPollHistory,
  type TaskPollHistoryEntry,
} from '../../task-content-api'

export function PollHistoryDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  taskId: string
}) {
  const { t } = useTranslation()
  const [items, setItems] = useState<TaskPollHistoryEntry[]>([])
  const [nextBeforeId, setNextBeforeId] = useState(0)
  const [loaded, setLoaded] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)

  const load = useCallback(
    async (beforeId: number) => {
      if (loading) return
      setLoading(true)
      setError(false)
      try {
        const result = await getTaskPollHistory(props.taskId, beforeId)
        const data = result.data
        if (!result.success || !data) throw new Error(result.message)
        setItems((current) =>
          beforeId === 0 ? data.items : [...current, ...data.items]
        )
        setNextBeforeId(data.next_before_id)
        setLoaded(true)
      } catch {
        setError(true)
      } finally {
        setLoading(false)
      }
    },
    [loading, props.taskId]
  )

  useEffect(() => {
    if (props.open && !loaded && !loading && !error) void load(0)
  }, [props.open, loaded, loading, error, load])

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Video polling history')}
      description={t(
        'Upstream status changes and response history for this video task.'
      )}
      contentClassName='sm:max-w-3xl'
      contentHeight='min(70vh, 42rem)'
    >
      <div className='space-y-3 overflow-y-auto'>
        {loading && !loaded ? <Spinner /> : null}
        {error ? (
          <p role='alert' className='text-destructive text-sm'>
            {t('Failed to load polling history')}
          </p>
        ) : null}
        {error ? (
          <Button variant='outline' onClick={() => void load(nextBeforeId)}>
            {t('Retry')}
          </Button>
        ) : null}
        {loaded && items.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No polling history recorded')}
          </p>
        ) : null}
        {items.map((entry) => (
          <div key={entry.id} className='rounded-md border p-3 text-sm'>
            <div className='flex flex-wrap justify-between gap-2 font-medium'>
              <span>{entry.status || t('Polling error')}</span>
              <time
                dateTime={new Date(entry.first_seen_at * 1000).toISOString()}
              >
                {new Date(entry.first_seen_at * 1000).toLocaleString()}
              </time>
            </div>
            {entry.http_status ? <p>HTTP {entry.http_status}</p> : null}
            {entry.error ? (
              <p className='text-destructive break-all'>{entry.error}</p>
            ) : null}
            {entry.repeat_count > 1 ? (
              <p className='text-muted-foreground'>
                {t('Repeated {{count}} times', { count: entry.repeat_count })} ·{' '}
                {t('Last seen')}:{' '}
                {new Date(entry.last_seen_at * 1000).toLocaleString()}
              </p>
            ) : null}
            {entry.response_omitted ? (
              <p className='text-muted-foreground'>
                {t('Response omitted because it exceeds 64 KiB')} (
                {entry.response_size} B)
              </p>
            ) : null}
            {entry.response !== undefined ? (
              <details className='mt-2'>
                <summary className='cursor-pointer'>
                  {t('View response JSON')}
                </summary>
                <pre className='bg-muted mt-2 max-h-72 overflow-auto rounded p-2 text-xs'>
                  {JSON.stringify(entry.response, null, 2)}
                </pre>
              </details>
            ) : null}
          </div>
        ))}
        {nextBeforeId > 0 ? (
          <Button
            variant='outline'
            disabled={loading}
            onClick={() => void load(nextBeforeId)}
          >
            {loading ? <Spinner /> : t('Load more')}
          </Button>
        ) : null}
      </div>
    </Dialog>
  )
}
