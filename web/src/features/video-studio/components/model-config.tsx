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
import { ChevronDown } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import type { ApiKey } from '@/features/keys/types'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import type { Capability, GenerationDraft, SubmitParameters } from '../types'

export const FIELD_LABEL =
  'text-muted-foreground font-mono text-[10px] tracking-wider uppercase'

type ModelConfigProps = {
  nodeId: string
  draft: GenerationDraft
  capability?: Capability
  capabilities: Capability[]
  keys: ApiKey[]
}

/** Key, model, and generation parameters; collapses to save panel space. */
export function ModelConfig(props: ModelConfigProps) {
  const { t } = useTranslation()
  const { nodeId, draft, capability, capabilities, keys } = props
  const updateNode = useVideoStudioStore((state) => state.updateNode)
  const [configOpen, setConfigOpen] = useState(true)
  const parameters = draft.parameters
  const setParameter = (patch: Partial<typeof parameters>) =>
    updateNode(nodeId, { parameters: { ...parameters, ...patch } })

  return (
    <Collapsible open={configOpen} onOpenChange={setConfigOpen}>
      <CollapsibleTrigger
        render={
          <Button
            variant='ghost'
            size='sm'
            className='w-full justify-between px-0'
          />
        }
      >
        <span className={FIELD_LABEL}>{t('Model and parameters')}</span>
        <ChevronDown
          className={configOpen ? 'rotate-180 transition' : 'transition'}
        />
      </CollapsibleTrigger>
      <CollapsibleContent className='grid gap-4 pt-3'>
        <div className='grid gap-1.5'>
          <Label htmlFor='studio-key' className={FIELD_LABEL}>
            {t('API key')}
          </Label>
          <NativeSelect
            id='studio-key'
            className='w-full'
            value={draft.token_id}
            onChange={(event) =>
              updateNode(nodeId, {
                token_id: Number(event.target.value),
                // Another key may not reach this model; the list reloads.
                model: '',
              })
            }
          >
            <NativeSelectOption value={0}>
              {t('Default (account group)')}
            </NativeSelectOption>
            {keys.map((key) => (
              <NativeSelectOption key={key.id} value={key.id}>
                {key.name}
                {key.group ? ` · ${key.group}` : ''}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='grid gap-1.5'>
          <Label htmlFor='studio-model' className={FIELD_LABEL}>
            {t('Model')}
          </Label>
          <NativeSelect
            id='studio-model'
            className='w-full'
            value={draft.model}
            onChange={(event) => {
              const next = capabilities.find(
                (item) => item.model === event.target.value
              )
              // Keep prompt and references; only reset values the new model lacks.
              updateNode(nodeId, {
                model: event.target.value,
                parameters: next
                  ? { ...next.defaults, ...pickSupported(parameters, next) }
                  : parameters,
              })
            }}
          >
            <NativeSelectOption value=''>
              {t('Select a model')}
            </NativeSelectOption>
            {capabilities.map((item) => (
              <NativeSelectOption key={item.model} value={item.model}>
                {item.model}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        {capability && (
          <div className='grid grid-cols-2 gap-3'>
            <div className='grid gap-1.5'>
              <Label htmlFor='studio-duration' className={FIELD_LABEL}>
                {t('Duration (s)')}
              </Label>
              <Input
                id='studio-duration'
                type='number'
                min={capability.min_duration}
                max={capability.max_duration}
                value={parameters.duration ?? ''}
                onChange={(event) =>
                  setParameter({
                    duration: Number.isNaN(event.target.valueAsNumber)
                      ? undefined
                      : event.target.valueAsNumber,
                  })
                }
              />
            </div>
            <OptionField
              id='studio-resolution'
              label={t('Resolution')}
              value={parameters.resolution ?? ''}
              options={capability.resolutions}
              onChange={(resolution) => setParameter({ resolution })}
            />
            <OptionField
              id='studio-ratio'
              label={t('Aspect ratio')}
              value={parameters.ratio ?? ''}
              options={capability.ratios}
              onChange={(ratio) => setParameter({ ratio })}
            />
            {capability.seed && (
              <div className='grid gap-1.5'>
                <Label htmlFor='studio-seed' className={FIELD_LABEL}>
                  {t('Seed')}
                </Label>
                <Input
                  id='studio-seed'
                  type='number'
                  placeholder={t('Random')}
                  value={parameters.seed ?? ''}
                  onChange={(event) =>
                    setParameter({
                      seed: Number.isNaN(event.target.valueAsNumber)
                        ? undefined
                        : event.target.valueAsNumber,
                    })
                  }
                />
              </div>
            )}
            {capability.audio && (
              <div className='col-span-2 flex items-center justify-between'>
                <Label htmlFor='studio-audio' className={FIELD_LABEL}>
                  {t('Generate audio')}
                </Label>
                <Switch
                  id='studio-audio'
                  checked={parameters.generate_audio ?? false}
                  onCheckedChange={(checked) =>
                    setParameter({ generate_audio: checked })
                  }
                />
              </div>
            )}
          </div>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

function pickSupported(parameters: SubmitParameters, capability: Capability) {
  const kept: {
    resolution?: string
    ratio?: string
    duration?: number
    generate_audio?: boolean
    seed?: number
  } = {}
  if (capability.audio && parameters.generate_audio !== undefined) {
    kept.generate_audio = parameters.generate_audio
  }
  if (capability.seed && parameters.seed !== undefined) {
    kept.seed = parameters.seed
  }
  if (
    parameters.resolution &&
    capability.resolutions.includes(parameters.resolution)
  ) {
    kept.resolution = parameters.resolution
  }
  if (parameters.ratio && capability.ratios.includes(parameters.ratio)) {
    kept.ratio = parameters.ratio
  }
  if (
    parameters.duration !== undefined &&
    parameters.duration >= capability.min_duration &&
    parameters.duration <= capability.max_duration
  ) {
    kept.duration = parameters.duration
  }
  return kept
}

type OptionFieldProps = {
  id: string
  label: string
  value: string
  options: string[]
  onChange: (value: string) => void
}

function OptionField(props: OptionFieldProps) {
  return (
    <div className='grid gap-1.5'>
      <Label htmlFor={props.id} className={FIELD_LABEL}>
        {props.label}
      </Label>
      <NativeSelect
        id={props.id}
        className='w-full'
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
      >
        {props.options.map((option) => (
          <NativeSelectOption key={option} value={option}>
            {option}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  )
}
