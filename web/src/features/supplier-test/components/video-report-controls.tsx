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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import { verifySupplierVideo } from '../api'
import {
  downloadFile,
  videoEvidenceText,
  buildVideoMarkdownReport,
  videoResult,
} from '../report'
import type { VideoTestRecord } from '../types'

export function VideoReportControls(props: {
  records: VideoTestRecord[]
  busy: boolean
  checks: { v3: boolean; materialLibrary: boolean }
  onChecksChange: (checks: { v3: boolean; materialLibrary: boolean }) => void
  onUpdate: (id: string, value: VideoTestRecord['verification']) => void
  onClear: () => void
  onBusyChange: (busy: boolean) => void
  stopVersion: number
}) {
  const { t } = useTranslation()
  const [accessKey, setAccessKey] = useState(''),
    [secretKey, setSecretKey] = useState('')
  const [automatic, setAutomatic] = useState(true)
  const [selected, setSelected] = useState('')
  const observed = useRef(new Set<string>())
  const controller = useRef<AbortController | null>(null)
  useEffect(() => () => controller.current?.abort(), [])
  useEffect(() => {
    controller.current?.abort()
  }, [props.stopVersion])
  const onUpdate = props.onUpdate
  const onBusyChange = props.onBusyChange
  const verify = useCallback(
    async (record: VideoTestRecord) => {
      if (controller.current || !accessKey.trim() || !secretKey.trim()) return
      const abort = new AbortController()
      controller.current = abort
      onBusyChange(true)
      let result: NonNullable<VideoTestRecord['verification']> = {
        ...record.verification,
        attempted: true,
        count: record.verification?.count || 0,
        elapsed: record.verification?.elapsed || 0,
        issue: undefined,
      }
      onUpdate(record.id, result)
      try {
        if (!result.queryId) {
          if (record.verification?.attempted) {
            throw new Error('Creation already attempted; inspect its receipt')
          }
          const exchange = await verifySupplierVideo(
            {
              action: 'create',
              video_url: record.metrics.video_url,
              access_key: accessKey.trim(),
              secret_key: secretKey.trim(),
            },
            abort.signal
          )
          result = { ...result, create: exchange }
          onUpdate(record.id, result)
          const body = JSON.parse(exchange.response || '{}')
          result.queryId = body.Result?.QueryID
          if (exchange.error || !result.queryId) {
            throw new Error('Verification creation was not confirmed')
          }
          onUpdate(record.id, { ...result })
        }
        const deadline = Date.now() + 10 * 60 * 1000
        while (!abort.signal.aborted && Date.now() < deadline) {
          const exchange = await verifySupplierVideo(
            {
              action: 'query',
              query_id: result.queryId,
              access_key: accessKey.trim(),
              secret_key: secretKey.trim(),
            },
            abort.signal
          )
          result = {
            ...result,
            last: exchange,
            count: result.count + 1,
            elapsed: result.elapsed + exchange.elapsed_ms,
          }
          onUpdate(record.id, result)
          const body = JSON.parse(exchange.response || '{}'),
            returnedId = body.Result?.QueryID
          if (returnedId && returnedId !== result.queryId) {
            throw new Error('Returned QueryID does not match')
          }
          const state = String(body.Result?.Status || '').toLowerCase()
          if (
            !exchange.error &&
            ['success', 'succeeded', 'failed', 'failure', 'error'].includes(
              state
            )
          ) {
            result = {
              ...result,
              final: exchange,
              issue: ['success', 'succeeded'].includes(state)
                ? undefined
                : 'Official verification failed',
            }
            onUpdate(record.id, result)
            return
          }
          if (
            (exchange.status >= 400 &&
              exchange.status < 500 &&
              ![408, 429].includes(exchange.status)) ||
            body.ResponseMetadata?.Error
          ) {
            throw new Error('Official verification request failed')
          }
          await new Promise<void>((resolve, reject) => {
            const cancel = () => {
              clearTimeout(timer)
              reject(new Error('Verification cancelled'))
            }
            const timer = setTimeout(() => {
              abort.signal.removeEventListener('abort', cancel)
              resolve()
            }, 5000)
            abort.signal.addEventListener('abort', cancel, { once: true })
          })
        }
        throw new Error('No final verification response obtained')
      } catch (error) {
        let issue = 'Official verification request failed'
        if (abort.signal.aborted) issue = 'Verification cancelled'
        else if (error instanceof Error) issue = error.message
        onUpdate(record.id, { ...result, issue })
      } finally {
        controller.current = null
        onBusyChange(false)
      }
    },
    [accessKey, secretKey, onUpdate, onBusyChange]
  )
  useEffect(() => {
    const completed = props.records.filter(
      (record) =>
        record.outcome === 'Succeeded' && !observed.current.has(record.id)
    )
    for (const record of completed) observed.current.add(record.id)
    if (
      !automatic ||
      !accessKey.trim() ||
      !secretKey.trim() ||
      controller.current
    ) {
      return
    }
    const next = completed.find(
      (record) => record.metrics.video_url && !record.verification?.attempted
    )
    if (next) void verify(next)
  }, [automatic, accessKey, secretKey, props.records, verify])
  const active =
    props.records.find((record) => record.id === selected) ||
    props.records.at(-1)
  return (
    <Card className='space-y-3 p-3'>
      <div className='flex flex-wrap gap-4'>
        <Label>
          <Checkbox
            checked={props.checks.v3}
            disabled={props.busy}
            onCheckedChange={(value) =>
              props.onChecksChange({ ...props.checks, v3: Boolean(value) })
            }
          />
          {t('Supplier has implemented the v3 API')}
        </Label>
        <Label>
          <Checkbox
            checked={props.checks.materialLibrary}
            disabled={props.busy}
            onCheckedChange={(value) =>
              props.onChecksChange({
                ...props.checks,
                materialLibrary: Boolean(value),
              })
            }
          />
          {t('Supplier has implemented a material library')}
        </Label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t('Checked means implemented; unchecked means not implemented.')}
      </p>
      <div className='grid gap-2 sm:grid-cols-2'>
        <Input
          aria-label={t('Official Access Key ID')}
          placeholder={t('Official Access Key ID')}
          value={accessKey}
          disabled={props.busy}
          autoComplete='off'
          onChange={(event) => setAccessKey(event.target.value)}
        />
        <Input
          type='password'
          aria-label={t('Official Secret Access Key')}
          placeholder={t('Official Secret Access Key')}
          value={secretKey}
          disabled={props.busy}
          autoComplete='new-password'
          onChange={(event) => setSecretKey(event.target.value)}
        />
      </div>
      <Label>
        <Checkbox
          checked={automatic}
          disabled={props.busy}
          onCheckedChange={(value) => setAutomatic(Boolean(value))}
        />
        {t('Automatically verify completed videos')}
      </Label>
      <p className='text-muted-foreground text-xs'>
        {t('Official AK/SK is used only for this page session.')}
      </p>
      <div className='overflow-x-auto'>
        <table className='w-full text-left text-xs'>
          <thead>
            <tr>
              {[
                'Model',
                'Request resolution',
                'Video generation',
                'Official origin features',
                'Model comparison',
              ].map((key) => (
                <th key={key} className='p-2'>
                  {t(key)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {props.records.map((record) => {
              const value = videoResult(record)
              return (
                <tr key={record.id}>
                  <td className='p-2'>
                    <Button
                      variant={active?.id === record.id ? 'secondary' : 'ghost'}
                      size='sm'
                      onClick={() => setSelected(record.id)}
                    >
                      {value.model}
                    </Button>
                  </td>
                  <td className='p-2'>{value.resolution}</td>
                  <td className='p-2'>{t(value.generation)}</td>
                  <td className='p-2'>{t(value.origin)}</td>
                  <td className='p-2'>{t(value.comparison)}</td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <div className='flex flex-wrap gap-2'>
        <Button
          variant='outline'
          disabled={
            props.busy ||
            !active?.metrics.video_url ||
            !accessKey.trim() ||
            !secretKey.trim() ||
            Boolean(
              active.verification?.attempted && !active.verification.queryId
            )
          }
          onClick={() => {
            if (active) void verify(active)
          }}
        >
          {t('Verify or continue querying')}
        </Button>
        <Button
          variant='outline'
          disabled={props.busy || !props.records.length}
          onClick={() =>
            downloadFile(
              'video-test-evidence.json',
              videoEvidenceText(JSON.stringify(props.records, null, 2)),
              'application/json'
            )
          }
        >
          {t('Download original evidence')}
        </Button>
        <Button
          variant='ghost'
          disabled={props.busy || !props.records.length}
          onClick={props.onClear}
        >
          {t('Clear current report')}
        </Button>
      </div>
      {active && (
        <div className='space-y-2'>
          <p className='text-xs break-all'>
            Task ID: {active.metrics.task_id || '—'} · QueryID:{' '}
            {active.verification?.queryId || '—'}
          </p>
          {active.verification?.issue && (
            <p className='text-xs'>{t(active.verification.issue)}</p>
          )}
          <details>
            <summary className='cursor-pointer text-sm'>
              {t('Request and response evidence')}
            </summary>
            <pre className='bg-muted rounded p-3 text-xs break-all whitespace-pre-wrap'>
              {buildVideoMarkdownReport({
                records: [active],
                ...props.checks,
                t: (key) => t(key),
              })}
            </pre>
          </details>
        </div>
      )}
    </Card>
  )
}
