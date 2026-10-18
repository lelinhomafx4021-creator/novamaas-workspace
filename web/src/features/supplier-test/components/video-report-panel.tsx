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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  downloadVideoEvidence,
  getVerificationSettings,
  saveVerificationSettings,
} from '../api'
import type { VideoReportChecks, VideoRunReport } from '../types'
import {
  compactEvidenceJSON,
  videoPollingSummary,
  videoReportSections,
  videoStageTitle,
  VIDEO_REPORT_CAPABILITIES,
} from '../video-report'

export function VideoReportPanel(props: {
  run?: VideoRunReport | null
  busy: boolean
  verify: boolean
  onVerifyChange: (checked: boolean) => void
  onVerifyRun?: () => void
  reportChecks?: VideoReportChecks
  onReportChecksChange?: (checks: VideoReportChecks) => void
}) {
  const { t } = useTranslation()
  const [configured, setConfigured] = useState(false)
  const [key, setKey] = useState('')
  const [secret, setSecret] = useState('')
  const [saving, setSaving] = useState(false)
  useEffect(() => {
    let active = true
    void getVerificationSettings()
      .then((value) => {
        if (active) setConfigured(value.configured)
      })
      .catch(() => {})
    return () => {
      active = false
    }
  }, [])
  const save = async () => {
    setSaving(true)
    try {
      await saveVerificationSettings(key, secret)
      setConfigured(true)
      setKey('')
      setSecret('')
      toast.success(t('Official verification credentials saved'))
    } catch {
      toast.error(t('Unable to save verification credentials'))
    } finally {
      setSaving(false)
    }
  }
  return (
    <Card className='space-y-3 p-3'>
      <div className='flex items-center gap-2'>
        <Checkbox
          id='verify-official-video'
          checked={props.verify}
          disabled={props.busy}
          onCheckedChange={(checked) => props.onVerifyChange(Boolean(checked))}
        />
        <Label htmlFor='verify-official-video'>
          {t('Verify the artifact with Volcano Ark')}
        </Label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Official verification uses a separate enterprise Ark AK/SK, not platform material credentials.'
        )}
      </p>
      {!configured && (
        <div className='space-y-2'>
          <Input
            aria-label={t('Official Access Key ID')}
            placeholder={t('Official Access Key ID')}
            value={key}
            autoComplete='off'
            onChange={(event) => setKey(event.target.value)}
            disabled={props.busy || saving}
          />
          <Input
            type='password'
            aria-label={t('Official Secret Access Key')}
            placeholder={t('Official Secret Access Key')}
            value={secret}
            autoComplete='new-password'
            onChange={(event) => setSecret(event.target.value)}
            disabled={props.busy || saving}
          />
          <Button
            variant='outline'
            disabled={props.busy || saving || !key.trim() || !secret.trim()}
            onClick={() => void save()}
          >
            {t('Save verification credentials')}
          </Button>
        </div>
      )}
      {configured && (
        <Button
          variant='ghost'
          disabled={props.busy}
          onClick={() => setConfigured(false)}
        >
          {t('Replace verification credentials')}
        </Button>
      )}
      {props.run && (
        <div className='space-y-3'>
          <h3 className='font-medium'>
            {t('Doubao Video Generation Test Report')}
          </h3>
          <FieldSet className='gap-2'>
            <FieldLegend variant='label'>
              {t('Manual confirmation')}
            </FieldLegend>
            <FieldDescription>
              {t('Select the capabilities confirmed for this report.')}
            </FieldDescription>
            <FieldGroup className='gap-2 sm:flex-row sm:gap-6'>
              {VIDEO_REPORT_CAPABILITIES.map((item) => (
                <Field
                  key={item.key}
                  orientation='horizontal'
                  data-disabled={props.busy || !props.onReportChecksChange}
                >
                  <Checkbox
                    id={`video-report-${item.key}`}
                    checked={Boolean(props.reportChecks?.[item.key])}
                    disabled={props.busy || !props.onReportChecksChange}
                    onCheckedChange={(checked) =>
                      props.onReportChecksChange?.({
                        ...props.reportChecks,
                        [item.key]: Boolean(checked),
                      })
                    }
                  />
                  <FieldLabel htmlFor={`video-report-${item.key}`}>
                    {t(item.label)}
                  </FieldLabel>
                </Field>
              ))}
            </FieldGroup>
          </FieldSet>
          <p className='text-xs break-all'>
            {t('Task ID')}: {props.run.task_id || '—'} · QueryID:{' '}
            {props.run.query_id || '—'}
          </p>
          <div className='flex flex-wrap gap-2'>
            <Button
              variant='outline'
              disabled={props.busy || !props.run.video_url || !configured}
              onClick={props.onVerifyRun}
            >
              {t('Continue official verification')}
            </Button>
            <Button
              variant='outline'
              disabled={!props.run.evidence_available}
              onClick={() => {
                if (props.run) {
                  void downloadVideoEvidence(props.run.id).catch(() =>
                    toast.error(t('Original evidence is unavailable'))
                  )
                }
              }}
            >
              {t('Download original evidence')}
            </Button>
          </div>
          {props.run.evidence_error && (
            <p className='text-xs'>{t(props.run.evidence_error)}</p>
          )}
          <p className='text-muted-foreground text-xs'>
            {videoPollingSummary(props.run, (key) => t(key))}
          </p>
          {videoReportSections(props.run).map((section, index) => (
            <section key={section.title} className='space-y-2'>
              <h4 className='border-b pb-1 text-sm font-medium'>
                {index + 1}. {t(section.title)}
              </h4>
              {section.stages.map((stage) => (
                <article key={stage.id} className='space-y-1 py-1'>
                  <h5 className='text-xs font-medium'>
                    {t(videoStageTitle(stage.id))}
                  </h5>
                  {stage.method && (
                    <p className='text-muted-foreground font-mono text-xs break-all'>
                      {stage.method} {stage.endpoint || stage.path} · HTTP{' '}
                      {stage.http_status || '—'}
                      {!stage.request_body && <> · {t('No request body')}</>}
                    </p>
                  )}
                  {stage.request_body && (
                    <>
                      <Label>{t('Request body')}</Label>
                      <pre className='bg-muted max-h-80 overflow-auto rounded p-2 text-xs break-all whitespace-pre-wrap'>
                        {compactEvidenceJSON(stage.request_body)}
                      </pre>
                    </>
                  )}
                  {(stage.method || stage.response_body) && (
                    <>
                      <Label>{t('Response body')}</Label>
                      <pre className='bg-muted max-h-80 overflow-auto rounded p-2 text-xs break-all whitespace-pre-wrap'>
                        {compactEvidenceJSON(stage.response_body) ||
                          t('No response received')}
                      </pre>
                    </>
                  )}
                  <p className='text-muted-foreground text-xs'>
                    {t(stage.conclusion)}
                    {!stage.final && stage.id.endsWith('_result') && (
                      <> · {t('No final response was obtained.')}</>
                    )}
                  </p>
                </article>
              ))}
            </section>
          ))}
        </div>
      )}
    </Card>
  )
}
