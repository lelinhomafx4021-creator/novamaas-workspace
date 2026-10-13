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
import { useMutation } from '@tanstack/react-query'
import { Sparkles, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import type { ApiKey } from '@/features/keys/types'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { optimizePrompt } from '../api'
import { draftIssues } from '../lib/draft'
import { studioErrorMessage } from '../lib/errors'
import { FIELD_LABEL } from '../lib/styles'
import type { Capability, ReferenceRole } from '../types'
import { ModelConfig } from './model-config'
import { ReferenceThumbnail } from './reference-thumbnail'

const ROLE_LABELS: Record<ReferenceRole, string> = {
  first_frame: 'First frame',
  last_frame: 'Last frame',
  reference_image: 'Reference image',
}

type PropertiesPanelProps = {
  capabilities: Capability[]
  keys: ApiKey[]
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
  const optimize = useMutation({
    mutationFn: (input: { model: string; group?: string; prompt: string }) =>
      optimizePrompt(input.model, input.group, input.prompt),
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  if (!nodeId || !draft) {
    return (
      <aside className='bg-background/80 text-muted-foreground flex w-80 shrink-0 items-center justify-center border-l p-8 text-center text-sm'>
        {t('Select a generation node to edit it.')}
      </aside>
    )
  }

  const capability = props.capabilities.find(
    (item) => item.model === draft.model
  )
  const issues = draftIssues(draft, capability)

  return (
    <aside className='bg-background/80 flex w-80 shrink-0 flex-col gap-5 overflow-y-auto border-l p-4 backdrop-blur-xl'>
      <ModelConfig
        nodeId={nodeId}
        draft={draft}
        capability={capability}
        capabilities={props.capabilities}
        keys={props.keys}
      />

      <div className='grid gap-1.5'>
        <div className='flex items-center justify-between'>
          <Label htmlFor='studio-prompt' className={FIELD_LABEL}>
            {t('Prompt')}
          </Label>
          {draft.text_token_id > 0 && draft.text_model && (
            <Button
              size='xs'
              variant='ghost'
              disabled={!draft.prompt.trim() || optimize.isPending}
              onClick={() =>
                optimize.mutate(
                  {
                    model: draft.text_model,
                    group:
                      props.keys.find((key) => key.id === draft.text_token_id)
                        ?.group || undefined,
                    prompt: draft.prompt.trim(),
                  },
                  { onSuccess: (prompt) => updateNode(nodeId, { prompt }) }
                )
              }
            >
              <Sparkles />
              {optimize.isPending ? t('Optimizing...') : t('Optimize prompt')}
            </Button>
          )}
        </div>
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
        <Label className={FIELD_LABEL}>{t('References')}</Label>
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
