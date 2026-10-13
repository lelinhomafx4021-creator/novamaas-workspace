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
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { draftIssues } from '../lib/draft'
import type { Capability, ReferenceRole, SubmitParameters } from '../types'
import { ReferenceThumbnail } from './reference-thumbnail'

const ROLE_LABELS: Record<ReferenceRole, string> = {
  first_frame: 'First frame',
  last_frame: 'Last frame',
  reference_image: 'Reference image',
}

type PropertiesPanelProps = {
  capabilities: Capability[]
  groups: { value: string; label: string }[]
  submitting: boolean
  onSubmit: (nodeId: string) => void
}

/** Right panel: the selected generation node's model, prompt, and inputs. */
export function PropertiesPanel(props: PropertiesPanelProps) {
  const { t } = useTranslation()
  const nodeId = useVideoStudioStore((state) => state.selectedNodeId)
  const draft = useVideoStudioStore((state) =>
    state.selectedNodeId ? state.draft.nodes[state.selectedNodeId] : undefined
  )
  const updateNode = useVideoStudioStore((state) => state.updateNode)

  if (!nodeId || !draft) {
    return (
      <aside className='bg-background text-muted-foreground w-80 shrink-0 border-l p-4 text-sm'>
        {t('Select a generation node to edit it.')}
      </aside>
    )
  }

  const capability = props.capabilities.find(
    (item) => item.model === draft.model
  )
  const issues = draftIssues(draft, capability)
  const parameters = draft.parameters
  const setParameter = (patch: Partial<typeof parameters>) =>
    updateNode(nodeId, { parameters: { ...parameters, ...patch } })

  return (
    <aside className='bg-background flex w-80 shrink-0 flex-col gap-4 overflow-y-auto border-l p-4'>
      <div className='grid gap-1.5'>
        <Label htmlFor='studio-model'>{t('Model')}</Label>
        <NativeSelect
          id='studio-model'
          className='w-full'
          value={draft.model}
          onChange={(event) => {
            const next = props.capabilities.find(
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
          {props.capabilities.map((item) => (
            <NativeSelectOption key={item.model} value={item.model}>
              {item.model}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>

      {props.groups.length > 1 && (
        <div className='grid gap-1.5'>
          <Label htmlFor='studio-group'>{t('Group')}</Label>
          <NativeSelect
            id='studio-group'
            className='w-full'
            value={draft.group}
            onChange={(event) =>
              updateNode(nodeId, { group: event.target.value })
            }
          >
            <NativeSelectOption value=''>
              {t('Default group')}
            </NativeSelectOption>
            {props.groups.map((group) => (
              <NativeSelectOption key={group.value} value={group.value}>
                {group.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      )}

      <div className='grid gap-1.5'>
        <Label htmlFor='studio-prompt'>{t('Prompt')}</Label>
        <Textarea
          id='studio-prompt'
          rows={5}
          value={draft.prompt}
          maxLength={4000}
          onChange={(event) =>
            updateNode(nodeId, { prompt: event.target.value })
          }
        />
      </div>

      <div className='grid gap-2'>
        <Label>{t('References')}</Label>
        {draft.references.length === 0 && (
          <p className='text-muted-foreground text-xs'>
            {t('No references. The video is generated from the prompt alone.')}
          </p>
        )}
        {draft.references.map((reference, index) => {
          const supported = capability?.roles.includes(reference.role) ?? false
          return (
            <div key={reference.id} className='flex items-center gap-2'>
              <span className='text-muted-foreground w-4 text-xs'>
                {index + 1}
              </span>
              <ReferenceThumbnail
                source={reference.source}
                id={reference.id}
                label={reference.label}
                className='size-10'
              />
              <NativeSelect
                aria-label={t('Reference role')}
                className='min-w-0 flex-1'
                value={reference.role}
                aria-invalid={!supported}
                onChange={(event) => {
                  const references = draft.references.map((item, itemIndex) =>
                    itemIndex === index
                      ? { ...item, role: event.target.value as ReferenceRole }
                      : item
                  )
                  updateNode(nodeId, { references })
                }}
              >
                {(Object.keys(ROLE_LABELS) as ReferenceRole[]).map((role) => (
                  <NativeSelectOption
                    key={role}
                    value={role}
                    disabled={!capability?.roles.includes(role)}
                  >
                    {t(ROLE_LABELS[role])}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
              <Button
                size='icon-sm'
                variant='ghost'
                aria-label={t('Remove reference')}
                onClick={() =>
                  updateNode(nodeId, {
                    references: draft.references.filter(
                      (_, itemIndex) => itemIndex !== index
                    ),
                  })
                }
              >
                <X />
              </Button>
            </div>
          )
        })}
      </div>

      {capability && (
        <div className='grid grid-cols-2 gap-3'>
          <div className='grid gap-1.5'>
            <Label htmlFor='studio-duration'>{t('Duration (s)')}</Label>
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
              <Label htmlFor='studio-seed'>{t('Seed')}</Label>
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
              <Label htmlFor='studio-audio'>{t('Generate audio')}</Label>
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

      {issues.length > 0 && (
        <ul className='text-destructive list-disc pl-4 text-xs'>
          {issues.map((issue) => (
            <li key={issue}>{t(issue)}</li>
          ))}
        </ul>
      )}
      <Button
        disabled={issues.length > 0 || props.submitting}
        onClick={() => props.onSubmit(nodeId)}
      >
        {props.submitting ? t('Submitting...') : t('Generate')}
      </Button>
    </aside>
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
    (capability.resolutions.length === 0 ||
      capability.resolutions.includes(parameters.resolution))
  ) {
    kept.resolution = parameters.resolution
  }
  if (
    parameters.ratio &&
    (capability.ratios.length === 0 ||
      capability.ratios.includes(parameters.ratio))
  ) {
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

/** A fixed list when the model constrains the value, free text otherwise. */
function OptionField(props: OptionFieldProps) {
  return (
    <div className='grid gap-1.5'>
      <Label htmlFor={props.id}>{props.label}</Label>
      {props.options.length > 0 ? (
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
      ) : (
        <Input
          id={props.id}
          value={props.value}
          maxLength={32}
          onChange={(event) => props.onChange(event.target.value.trim())}
        />
      )}
    </div>
  )
}
