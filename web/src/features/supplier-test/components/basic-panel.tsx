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
import type { Dispatch, SetStateAction } from 'react'
import { useTranslation } from 'react-i18next'

import { Label } from '@/components/ui/label'
import { TabsContent } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  MAX_TOKENS_CAP,
  PROTOCOL_BASIC_IDS,
  SHALLOW_BASIC_IDS,
} from '../constants'
import type { BasicForm, CheckResult } from '../types'
import { CheckTable } from './check-table'
import { NumberField, OptionalNumberField, StreamSwitch } from './form-controls'

export function BasicPanel(props: {
  basic: BasicForm
  busy: boolean
  basicChecks: CheckResult[]
  basicStreamText: string
  basicSummary: string
  onBasicChange: Dispatch<SetStateAction<BasicForm>>
  onRunCheck: (id: string) => void
}) {
  const { t } = useTranslation()

  return (
    <TabsContent value='basic' className='space-y-4'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Shallow checks ask whether the door opens. Protocol checks are skipped when the vendor has no matching API. Empty temperature / top_p are not sent.'
        )}
      </p>
      <div className='grid gap-4 md:grid-cols-4'>
        <NumberField
          id='basic-max-tokens'
          label={t('Max tokens')}
          value={props.basic.maxTokens}
          disabled={props.busy}
          min={1}
          max={MAX_TOKENS_CAP}
          presets={[
            { label: '64', value: 64 },
            { label: '256', value: 256 },
            { label: '1k', value: 1024 },
            { label: '4k', value: 4096 },
          ]}
          onChange={(value) =>
            props.onBasicChange((current) => ({ ...current, maxTokens: value }))
          }
        />
        <OptionalNumberField
          id='basic-temperature'
          label={t('Temperature')}
          value={props.basic.temperature}
          disabled={props.busy}
          min={0}
          max={2}
          step={0.1}
          placeholder={t('Leave empty to omit')}
          onChange={(value) =>
            props.onBasicChange((current) => ({ ...current, temperature: value }))
          }
        />
        <OptionalNumberField
          id='basic-top-p'
          label={t('Top P')}
          value={props.basic.topP}
          disabled={props.busy}
          min={0}
          max={1}
          step={0.05}
          placeholder={t('Leave empty to omit')}
          onChange={(value) =>
            props.onBasicChange((current) => ({ ...current, topP: value }))
          }
        />
        <StreamSwitch
          id='basic-stream'
          checked={props.basic.stream}
          disabled={props.busy}
          onChange={(checked) =>
            props.onBasicChange((current) => ({ ...current, stream: checked }))
          }
        />
      </div>
      <div className='mt-4 space-y-2'>
        <Label htmlFor='basic-prompt'>{t('Prompt')}</Label>
        <Textarea
          id='basic-prompt'
          rows={3}
          value={props.basic.prompt}
          disabled={props.busy}
          onChange={(event) =>
            props.onBasicChange((current) => ({
              ...current,
              prompt: event.target.value,
            }))
          }
        />
      </div>
      {props.basicStreamText ? (
        <pre className='bg-muted mt-4 max-h-48 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
          {props.basicStreamText}
        </pre>
      ) : null}
      {props.basicSummary ? (
        <p className='text-muted-foreground mt-4 text-sm'>
          {props.basicSummary}
        </p>
      ) : null}
      <div className='space-y-4'>
        <div>
          <p className='mb-2 text-sm font-medium'>
            {t('Shallow · connectivity')}
          </p>
          <CheckTable
            checks={props.basicChecks.filter((check) =>
              (SHALLOW_BASIC_IDS as readonly string[]).includes(check.id)
            )}
            busy={props.busy}
            onRun={props.onRunCheck}
          />
        </div>
        <div>
          <p className='mb-2 text-sm font-medium'>
            {t('Deep · protocol')}
          </p>
          <p className='text-muted-foreground mb-2 text-sm'>
            {t(
              'Protocol checks are skipped when the vendor has no matching API. That is incomplete protocol, not a broken supplier.'
            )}
          </p>
          <CheckTable
            checks={props.basicChecks.filter((check) =>
              (PROTOCOL_BASIC_IDS as readonly string[]).includes(check.id)
            )}
            busy={props.busy}
            onRun={props.onRunCheck}
          />
        </div>
      </div>
    </TabsContent>
  )
}
