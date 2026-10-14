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
import { lazy, Suspense, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Spinner } from '@/components/ui/spinner'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { requestSnapshotPageURL } from '../../request-snapshot-download'
import { RequestSnapshotDownloadButton } from './request-snapshot-download-button'
import { MAX_REQUEST_SNAPSHOT_PREVIEW_CHARACTERS } from './request-snapshot-limits'

const RequestJsonViewer = lazy(() => import('./request-json-viewer'))

function SnapshotJson(props: {
  filename: 'original-request.json' | 'upstream-request.json'
  json: string
  url?: string
}) {
  const { t } = useTranslation()
  const pageURL = props.url ? requestSnapshotPageURL(props.url) : null
  return (
    <div className='flex min-w-0 flex-col gap-2'>
      {props.url && (
        <div className='flex flex-wrap items-center gap-2'>
          <RequestSnapshotDownloadButton url={props.url} />
          {pageURL && (
            <CopyButton
              value={pageURL}
              aria-label={t('Copy permanent link')}
              size='sm'
              variant='outline'
            >
              {t('Copy permanent link')}
            </CopyButton>
          )}
          <span className='text-muted-foreground text-xs'>
            {t(
              'The permanent link opens a download page after administrator sign-in.'
            )}
          </span>
        </div>
      )}
      <RequestJsonViewer
        filename={props.filename}
        json={props.json}
        downloadUrl={props.url}
      />
    </div>
  )
}

function formatRequestBody(body: unknown): string | null {
  if (body === undefined) {
    return null
  }
  if (typeof body === 'string') {
    if (body.length > MAX_REQUEST_SNAPSHOT_PREVIEW_CHARACTERS) return body
    try {
      return JSON.stringify(JSON.parse(body), null, 2)
    } catch {
      return body
    }
  }
  const compact = JSON.stringify(body) ?? 'null'
  if (compact.length > MAX_REQUEST_SNAPSHOT_PREVIEW_CHARACTERS) return compact
  return JSON.stringify(body, null, 2) ?? 'null'
}

export function RequestSnapshotsViewer(props: {
  original?: unknown
  upstream?: unknown
  originalUrl?: string
  upstreamUrl?: string
}) {
  const { t } = useTranslation()
  const originalJson = useMemo(
    () => formatRequestBody(props.original),
    [props.original]
  )
  const upstreamJson = useMemo(
    () => formatRequestBody(props.upstream),
    [props.upstream]
  )

  if (originalJson === null && upstreamJson === null) {
    return null
  }
  const fallback = (
    <div className='flex min-h-32 items-center justify-center'>
      <Spinner />
    </div>
  )
  if (upstreamJson === null) {
    return (
      <Suspense fallback={fallback}>
        <SnapshotJson
          filename='original-request.json'
          json={originalJson ?? ''}
          url={props.originalUrl}
        />
      </Suspense>
    )
  }
  if (originalJson === null) {
    return (
      <Suspense fallback={fallback}>
        <SnapshotJson
          filename='upstream-request.json'
          json={upstreamJson}
          url={props.upstreamUrl}
        />
      </Suspense>
    )
  }

  return (
    <Suspense fallback={fallback}>
      <Tabs defaultValue='original' className='min-w-0 gap-2'>
        <TabsList className='grid w-full grid-cols-2'>
          <TabsTrigger value='original'>{t('Original Request')}</TabsTrigger>
          <TabsTrigger value='upstream'>
            {t('Actual Upstream Request')}
          </TabsTrigger>
        </TabsList>
        <TabsContent value='original' className='min-w-0'>
          <SnapshotJson
            filename='original-request.json'
            json={originalJson}
            url={props.originalUrl}
          />
        </TabsContent>
        <TabsContent value='upstream' className='min-w-0'>
          <SnapshotJson
            filename='upstream-request.json'
            json={upstreamJson}
            url={props.upstreamUrl}
          />
        </TabsContent>
      </Tabs>
    </Suspense>
  )
}
