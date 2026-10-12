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
import { useEffect, useMemo, useRef, useState } from 'react'
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

function modelSizeOptions(model: string): string[] {
  if (model === 'dall-e-3') return ['1024x1024', '1792x1024', '1024x1792']
  if (model === 'dall-e-2' || model === 'dall-e') return ['1024x1024']
  if (model.includes('seedream-4-5') || model.includes('seedream-5-0-lite')) {
    return ['2048x2048', '2496x1664', '1664x2496']
  }
  return ['1024x1024', '1536x1024', '1024x1536']
}

function modelMaxImages(model: string, limit: number): number {
  if (model === 'dall-e-3') return 1
  if (
    model === 'dall-e-2' ||
    model === 'dall-e' ||
    model.startsWith('gpt-image-') ||
    model === 'chatgpt-image-latest'
  ) {
    return Math.min(limit, 10)
  }
  return limit
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
        n: z.number().int().min(1).max(options.settings.max_images),
        size: z.string().trim(),
      })
    ),
    defaultValues: {
      tokenId: String(options.keys[0]?.id ?? ''),
      model: '',
      prompt: '',
      n: 1,
      size: '',
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
  const sizeOptions = useMemo(() => modelSizeOptions(model), [model])
  const maxImages = modelMaxImages(model, options.settings.max_images)
  const size = form.watch('size')
  const previousModel = useRef(model)
  useEffect(() => {
    if (form.getValues('n') > maxImages) {
      form.setValue('n', maxImages, { shouldValidate: true })
      requestKey.current = ''
    }
    if (previousModel.current !== model) {
      previousModel.current = model
      form.setValue('size', '')
      requestKey.current = ''
    }
  }, [form, maxImages, model])
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
        ...(values.n > 1 ? { n: values.n } : {}),
        ...(values.size ? { size: values.size } : {}),
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
    size,
    sizeOptions,
    maxImages,
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
