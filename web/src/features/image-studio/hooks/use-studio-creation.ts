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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery } from '@tanstack/react-query'
import { nanoid } from 'nanoid'
import { useEffect, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

import type { ApiKey } from '@/features/keys/types'
import { useAuthStore } from '@/stores/auth-store'

import { assistStudioPrompt, createStudioRun, listStudioModels } from '../api'
import type { StudioModelSelection } from '../components/studio-model-settings'
import type { CreateStudioRunInput, StudioRun, StudioSettings } from '../types'

export type StudioCreationOptions = {
  settings: StudioSettings
  keys: ApiKey[]
  disabled: boolean
  onGenerated: (run: StudioRun, replay: boolean) => void
}

export function useStudioCreation(options: StudioCreationOptions) {
  const userGroup = useAuthStore((state) => state.auth.user?.group || 'default')
  const [textSelection, setTextSelection] = useState<StudioModelSelection>({
    tokenId: options.keys[0]?.id ?? 0,
    model: '',
  })
  // One key per logical submission: retries of the same request replay the
  // stored run instead of generating and billing again.
  const requestKey = useRef('')
  const form = useForm({
    resolver: zodResolver(
      z.object({
        tokenId: z.string().min(1),
        model: z.string(),
        prompt: z
          .string()
          .trim()
          .min(1)
          .max(options.settings.max_prompt_characters),
      })
    ),
    defaultValues: {
      tokenId: String(options.keys[0]?.id ?? ''),
      model: '',
      prompt: '',
    },
  })
  const tokenId = Number(form.watch('tokenId'))
  const models = useQuery({
    queryKey: ['image-studio', 'models', tokenId],
    queryFn: () => listStudioModels(tokenId),
    enabled: Boolean(tokenId),
  })
  const selectedModel = form.watch('model')
  const model = models.data?.includes(selectedModel)
    ? selectedModel
    : (models.data?.[0] ?? '')
  const previousModel = useRef(model)
  useEffect(() => {
    if (previousModel.current !== model) {
      previousModel.current = model
      requestKey.current = ''
    }
  }, [model])
  const textModels = useQuery({
    queryKey: ['image-studio', 'text-models', textSelection.tokenId],
    queryFn: () => listStudioModels(textSelection.tokenId, 'text'),
    enabled: Boolean(textSelection.tokenId),
  })
  const textModel = textModels.data?.includes(textSelection.model)
    ? textSelection.model
    : (textModels.data?.[0] ?? '')
  const prompt = form.watch('prompt')
  const changePrompt = (value: string) => {
    requestKey.current = ''
    form.setValue('prompt', value, { shouldDirty: true, shouldValidate: true })
  }
  const optimize = useMutation({
    mutationFn: (originalPrompt: string) =>
      assistStudioPrompt({
        token_id: textSelection.tokenId,
        model: textModel,
        prompt: originalPrompt,
      }),
    onSuccess: (result) => changePrompt(result.suggested_prompt),
  })
  const generate = useMutation({
    mutationFn: (input: Omit<CreateStudioRunInput, 'idempotency_key'>) => {
      requestKey.current ||= nanoid()
      return createStudioRun({ ...input, idempotency_key: requestKey.current })
    },
    onSuccess: (result) => {
      requestKey.current = ''
      options.onGenerated(result.run, result.replay)
    },
  })
  const generating = generate.isPending || options.disabled
  const submit = form.handleSubmit((values) => {
    if (
      !options.settings.enabled ||
      !model ||
      generating ||
      optimize.isPending
    ) {
      return
    }
    generate.mutate({
      token_id: Number(values.tokenId),
      request: {
        model,
        prompt: values.prompt,
      },
    })
  })

  return {
    form,
    userGroup,
    tokenId,
    textSelection,
    setTextSelection,
    prompt,
    model,
    models,
    optimize,
    generating,
    changePrompt,
    submit,
    generate,
    textModel,
    textModels,
    resetRequestKey: () => {
      requestKey.current = ''
    },
  }
}

export type StudioCreationState = ReturnType<typeof useStudioCreation>
