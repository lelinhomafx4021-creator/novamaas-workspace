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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import type { ApiKey } from '@/features/keys/types'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { listTextModels } from '../api'
import { FIELD_LABEL } from '../lib/styles'
import type { GenerationDraft } from '../types'

type TextModelFieldsProps = {
  nodeId: string
  draft: GenerationDraft
  keys: ApiKey[]
}

/** Key and model that rewrite the prompt; independent of the video key. */
export function TextModelFields(props: TextModelFieldsProps) {
  const { t } = useTranslation()
  const { nodeId, draft, keys } = props
  const updateNode = useVideoStudioStore((state) => state.updateNode)
  const models = useQuery({
    queryKey: ['video-studio', 'text-models', draft.text_token_id],
    enabled: draft.text_token_id > 0,
    queryFn: async () => {
      const response = await listTextModels(draft.text_token_id)
      if (!response.success) throw new Error(response.message)
      return response.data
    },
  })

  return (
    <>
      <div className='grid gap-1.5'>
        <Label htmlFor='studio-text-key' className={FIELD_LABEL}>
          {t('Text API key')}
        </Label>
        <NativeSelect
          id='studio-text-key'
          className='w-full'
          value={draft.text_token_id}
          onChange={(event) =>
            updateNode(nodeId, {
              text_token_id: Number(event.target.value),
              text_model: '',
            })
          }
        >
          <NativeSelectOption value={0}>
            {t('Off (no prompt optimization)')}
          </NativeSelectOption>
          {keys.map((key) => (
            <NativeSelectOption key={key.id} value={key.id}>
              {key.name}
              {key.group ? ` · ${key.group}` : ''}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </div>

      {draft.text_token_id > 0 && (
        <div className='grid gap-1.5'>
          <Label htmlFor='studio-text-model' className={FIELD_LABEL}>
            {t('Text model')}
          </Label>
          <NativeSelect
            id='studio-text-model'
            className='w-full'
            value={draft.text_model}
            onChange={(event) =>
              updateNode(nodeId, { text_model: event.target.value })
            }
          >
            <NativeSelectOption value=''>
              {t('Select a model')}
            </NativeSelectOption>
            {models.data?.map((name) => (
              <NativeSelectOption key={name} value={name}>
                {name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>
      )}
    </>
  )
}
