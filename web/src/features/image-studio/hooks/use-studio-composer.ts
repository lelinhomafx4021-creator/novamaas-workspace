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
import { useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { z } from 'zod'

import type { ApiKey } from '@/features/keys/types'
import { useAuthStore } from '@/stores/auth-store'

import { assistStudioPrompt, createStudioRun, listStudioModels } from '../api'
import type { StudioModelSelection } from '../components/studio-model-settings'
import type { StudioRun, StudioSettings } from '../types'

export function useStudioComposer(props: {
  authenticated: boolean
  settings: StudioSettings
  keys: ApiKey[]
  disabled: boolean
  onGenerated: (run: StudioRun) => void
  onNewCreation: () => void
}) {
  const userGroup = useAuthStore((state) => state.auth.user?.group || 'default')
  const maxCharacters = props.settings.max_prompt_characters
  const [text, setText] = useState<StudioModelSelection>({
    tokenId: props.keys[0]?.id ?? 0,
    model: '',
  })
  // Retrying unchanged input replays the same run instead of billing again.
  const requestKey = useRef('')
  const form = useForm({
    resolver: zodResolver(
      z.object({
        tokenId: z.string().min(1),
        model: z.string(),
        prompt: z.string().trim().min(1).max(maxCharacters),
      })
    ),
    defaultValues: {
      tokenId: String(props.keys[0]?.id ?? ''),
      model: '',
      prompt: '',
    },
  })
  const tokenId = Number(form.watch('tokenId'))
  const prompt = form.watch('prompt')
  const selectedModel = form.watch('model')
  const models = useQuery({
    queryKey: ['image-studio', 'models', tokenId],
    queryFn: () => listStudioModels(tokenId),
    enabled: props.authenticated && Boolean(tokenId),
  })
  const model = models.data?.includes(selectedModel)
    ? selectedModel
    : (models.data?.[0] ?? '')
  const textModels = useQuery({
    queryKey: ['image-studio', 'text-models', text.tokenId],
    queryFn: () => listStudioModels(text.tokenId, 'text'),
    enabled: props.authenticated && Boolean(text.tokenId),
  })
  const textModel = textModels.data?.includes(text.model)
    ? text.model
    : (textModels.data?.[0] ?? '')
  const changePrompt = (value: string) => {
    requestKey.current = ''
    form.setValue('prompt', value, { shouldDirty: true, shouldValidate: true })
  }
  const optimize = useMutation({
    mutationFn: (original: string) =>
      assistStudioPrompt({
        token_id: text.tokenId,
        model: textModel,
        prompt: original,
      }),
    onSuccess: (result) => changePrompt(result.suggested_prompt),
  })
  const generate = useMutation({
    mutationFn: (values: { tokenId: string; prompt: string }) => {
      requestKey.current ||= nanoid()
      return createStudioRun({
        token_id: Number(values.tokenId),
        idempotency_key: requestKey.current,
        request: { model, prompt: values.prompt },
      })
    },
    onSuccess: (result) => {
      requestKey.current = ''
      props.onGenerated(result.run)
    },
  })
  const generating = generate.isPending || props.disabled
  const busy = generating || optimize.isPending
  const canGenerate =
    props.authenticated &&
    props.settings.enabled &&
    Boolean(model) &&
    Boolean(prompt.trim()) &&
    prompt.length <= maxCharacters &&
    !busy
  const submit = form.handleSubmit((values) => {
    if (canGenerate) generate.mutate(values)
  })

  return {
    form,
    userGroup,
    maxCharacters,
    text,
    setText,
    requestKey,
    tokenId,
    prompt,
    models,
    model,
    textModels,
    textModel,
    changePrompt,
    optimize,
    generate,
    generating,
    busy,
    canGenerate,
    submit,
    reusePrompt: (value: string) => {
      if (busy || value.length > maxCharacters) return
      changePrompt(value)
      optimize.reset()
      form.setFocus('prompt')
    },
    newCreation: () => {
      if (busy) return
      changePrompt('')
      optimize.reset()
      generate.reset()
      form.setFocus('prompt')
      props.onNewCreation()
    },
  }
}
