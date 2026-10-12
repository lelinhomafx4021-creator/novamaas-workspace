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
import {
  ArrowTurnBackwardIcon,
  Image01Icon,
  SparklesIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery } from '@tanstack/react-query'
import { nanoid } from 'nanoid'
import { useRef, useState, type ReactNode } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import type { ApiKey } from '@/features/keys/types'
import { useAuthStore } from '@/stores/auth-store'

import { assistStudioPrompt, createStudioRun, listStudioModels } from '../api'
import { studioErrorMessage } from '../studio-utils'
import type { StudioRun, StudioSettings } from '../types'
import {
  StudioModelSettings,
  type StudioModelSelection,
} from './studio-model-settings'
import { StudioPromptExamples } from './studio-prompt-examples'

export function StudioComposer(props: {
  settings: StudioSettings
  keys: ApiKey[]
  disabled: boolean
  onGenerated: (run: StudioRun) => void
  children: (
    generating: boolean,
    reusePrompt: (prompt: string) => void
  ) => ReactNode
}) {
  const { t } = useTranslation()
  const userGroup = useAuthStore((state) => state.auth.user?.group || 'default')
  const maxCharacters = props.settings.max_prompt_characters
  const [text, setText] = useState<StudioModelSelection>({
    tokenId: props.keys[0]?.id ?? 0,
    model: '',
  })
  // One key per logical submission: retries of the same request replay the
  // stored run instead of generating and billing again. Any input change
  // starts a new submission.
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
  const models = useQuery({
    queryKey: ['image-studio', 'models', tokenId],
    queryFn: () => listStudioModels(tokenId),
    enabled: Boolean(tokenId),
  })
  const selectedModel = form.watch('model')
  const model = models.data?.includes(selectedModel)
    ? selectedModel
    : (models.data?.[0] ?? '')
  const textModels = useQuery({
    queryKey: ['image-studio', 'text-models', text.tokenId],
    queryFn: () => listStudioModels(text.tokenId, 'text'),
    enabled: Boolean(text.tokenId),
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
    props.settings.enabled && Boolean(model) && Boolean(prompt.trim()) && !busy
  const submit = form.handleSubmit((values) => {
    if (canGenerate) generate.mutate(values)
  })
  return (
    <>
      <header className='flex shrink-0 flex-wrap items-center justify-between gap-3 border-b px-4 py-3 sm:px-6'>
        <div className='flex min-w-0 items-center gap-3'>
          <span className='bg-primary/10 text-primary flex size-10 shrink-0 items-center justify-center rounded-xl'>
            <HugeiconsIcon
              icon={Image01Icon}
              className='size-5'
              aria-hidden='true'
            />
          </span>
          <div>
            <h1 className='text-lg font-semibold tracking-tight'>
              {t('Image generation')}
            </h1>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t('Describe an idea. Create something new.')}
            </p>
          </div>
        </div>
        <StudioModelSettings
          keys={props.keys}
          userGroup={userGroup}
          image={{ tokenId, model }}
          text={{ ...text, model: textModel }}
          imageModels={models}
          textModels={textModels}
          disabled={busy}
          onImageChange={(selection) => {
            requestKey.current = ''
            form.setValue('tokenId', String(selection.tokenId))
            form.setValue('model', selection.model)
          }}
          onTextChange={setText}
        />
      </header>
      <div className='grid shrink-0 items-start gap-3 p-3 sm:p-4 xl:min-h-0 xl:flex-1 xl:grid-cols-[17rem_minmax(0,1fr)_16rem] xl:grid-rows-[minmax(0,1fr)] xl:items-stretch 2xl:grid-cols-[18rem_minmax(0,1fr)_17rem]'>
        <form
          noValidate
          onSubmit={submit}
          onChange={() => {
            requestKey.current = ''
          }}
          className='bg-card/90 flex min-h-0 min-w-0 flex-col rounded-2xl border shadow-sm xl:overflow-hidden'
        >
          <div className='min-h-0 p-3 xl:flex-1 xl:overflow-y-auto'>
            <FieldGroup className='gap-3'>
              <div className='grid gap-1'>
                <h2 className='font-semibold'>{t('Start with an idea')}</h2>
                <p className='text-muted-foreground flex min-w-0 items-center gap-1.5 text-xs'>
                  <HugeiconsIcon
                    icon={Image01Icon}
                    className='size-3.5 shrink-0'
                    aria-hidden='true'
                  />
                  <span className='truncate' title={model}>
                    {model || t('No image models available')}
                  </span>
                </p>
              </div>
              {!props.settings.enabled && (
                <p role='status' className='text-muted-foreground text-sm'>
                  {t('Image generation is disabled')}
                </p>
              )}
              {!props.keys.length && (
                <p role='status' className='text-muted-foreground text-sm'>
                  {t(
                    'No API keys available. Create your first API key to get started.'
                  )}
                </p>
              )}
              {models.isError && (
                <p role='alert' className='text-destructive text-xs'>
                  {t('Failed to load enabled models')}
                </p>
              )}
              <Field
                data-disabled={busy}
                data-invalid={Boolean(form.formState.errors.prompt)}
              >
                <FieldLabel htmlFor='studio-prompt'>
                  {t('Describe your image')}
                </FieldLabel>
                <Textarea
                  id='studio-prompt'
                  rows={4}
                  className='[field-sizing:fixed] min-h-24 resize-y'
                  placeholder={t(
                    'Describe the subject, scene, lighting and style, or try an example below.'
                  )}
                  {...form.register('prompt')}
                  disabled={busy}
                  aria-invalid={Boolean(form.formState.errors.prompt)}
                  maxLength={maxCharacters}
                  onKeyDown={(event) => {
                    if (
                      (event.metaKey || event.ctrlKey) &&
                      event.key === 'Enter'
                    ) {
                      event.preventDefault()
                      void submit()
                    }
                  }}
                />
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    disabled={
                      !props.settings.enabled ||
                      busy ||
                      !textModel ||
                      textModels.isFetching ||
                      !prompt.trim() ||
                      prompt.length > maxCharacters
                    }
                    onClick={() => optimize.mutate(prompt)}
                  >
                    <HugeiconsIcon icon={SparklesIcon} aria-hidden='true' />
                    {optimize.isPending
                      ? t('Optimizing...')
                      : t('Optimize prompt')}
                  </Button>
                  <span className='text-muted-foreground text-[11px]'>
                    {prompt.length} / {maxCharacters}
                  </span>
                </div>
                <FieldDescription className='text-xs break-words'>
                  {t('Text model')}: {textModel || t('No models available')}
                </FieldDescription>
                {optimize.isError && (
                  <p role='alert' className='text-destructive text-xs'>
                    {studioErrorMessage(optimize.error)}
                  </p>
                )}
                {optimize.isSuccess &&
                  prompt === optimize.data.suggested_prompt && (
                    <div className='text-muted-foreground flex items-center justify-between text-xs'>
                      <span role='status'>{t('Prompt optimized')}</span>
                      <Button
                        type='button'
                        size='xs'
                        variant='ghost'
                        disabled={busy}
                        onClick={() => {
                          changePrompt(optimize.variables)
                          optimize.reset()
                        }}
                      >
                        <HugeiconsIcon
                          icon={ArrowTurnBackwardIcon}
                          aria-hidden='true'
                        />
                        {t('Undo')}
                      </Button>
                    </div>
                  )}
              </Field>
              <StudioPromptExamples
                maxCharacters={maxCharacters}
                disabled={!props.settings.enabled || busy}
                onChange={changePrompt}
              />
              <Field>
                <FieldLabel>{t('Image proportions')}</FieldLabel>
                <p className='bg-muted/30 rounded-lg border px-3 py-2 text-sm'>
                  {t('Model default')}
                </p>
                <FieldDescription>
                  {t(
                    'Model defaults apply; you can also specify proportions in your prompt.'
                  )}
                </FieldDescription>
              </Field>
            </FieldGroup>
          </div>
          <div className='shrink-0 border-t p-4'>
            {generate.isError && (
              <p role='alert' className='text-destructive mb-3 text-xs'>
                {studioErrorMessage(generate.error)}
              </p>
            )}
            <Button
              type='submit'
              size='lg'
              className='h-10 w-full'
              disabled={!canGenerate}
            >
              <HugeiconsIcon icon={SparklesIcon} aria-hidden='true' />
              {generating ? t('Generating...') : t('Create images')}
            </Button>
          </div>
        </form>
        {props.children(busy, (value) => {
          if (busy || value.length > maxCharacters) return
          changePrompt(value)
          optimize.reset()
          form.setFocus('prompt')
        })}
      </div>
    </>
  )
}
