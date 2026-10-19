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
import {
  ArrowTurnBackwardIcon,
  Add01Icon,
  Clock01Icon,
  Image01Icon,
  SparklesIcon,
  ArrowDown01Icon,
  ArrowUp01Icon,
  FullScreenIcon,
  MinimizeScreenIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SystemBrand } from '@/components/layout/components/system-brand'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import type { ApiKey } from '@/features/keys/types'
import { cn } from '@/lib/utils'

import { useStudioComposer } from '../hooks/use-studio-composer'
import { studioErrorMessage } from '../studio-utils'
import type { StudioRun, StudioSettings } from '../types'
import { StudioModelSettings } from './studio-model-settings'
import { StudioReferenceImages } from './studio-reference-images'

export function StudioComposer(props: {
  authenticated: boolean
  settings: StudioSettings
  keys: ApiKey[]
  disabled: boolean
  onGenerated: (run: StudioRun) => void
  onNewCreation: () => void
  children: (
    generating: boolean,
    reusePrompt: (prompt: string) => void
  ) => { canvas: ReactNode; history: ReactNode; hasResults: boolean }
}) {
  const { t } = useTranslation()
  const studio = useStudioComposer(props)
  const [editorMode, setEditorMode] = useState<
    'normal' | 'collapsed' | 'expanded'
  >('normal')
  const [dragging, setDragging] = useState(false)
  const historyRef = useRef<HTMLDivElement>(null)
  const views = props.children(studio.busy, (value) => {
    if (studio.busy) return
    setEditorMode('normal')
    studio.reusePrompt(value)
    requestAnimationFrame(() => studio.form.setFocus('prompt'))
  })
  const newCreation = () => {
    if (studio.busy) return
    setEditorMode('normal')
    studio.newCreation()
    requestAnimationFrame(() => studio.form.setFocus('prompt'))
  }
  const showHistory = () => {
    historyRef.current?.scrollIntoView({ block: 'start' })
    historyRef.current?.focus({ preventScroll: true })
  }

  return (
    <>
      <header className='sticky top-0 z-10 flex min-h-16 shrink-0 items-center justify-between gap-2 border-b border-(--studio-subtle-border) bg-(--studio-surface) px-3 py-2 md:gap-4 md:px-6'>
        <div className='flex min-w-0 items-center gap-2.5 md:gap-5 max-md:[&>a]:px-0 max-md:[&>a>span]:hidden'>
          <SystemBrand variant='inline' />
          <span
            className='bg-border h-6 w-px shrink-0 md:h-7'
            aria-hidden='true'
          />
          <div>
            <p className='text-muted-foreground text-[8px] font-medium tracking-[0.16em] uppercase md:text-[10px]'>
              {t('Image studio')}
            </p>
            <h1 className='mt-[3px] text-[13px] font-medium md:text-[15px]'>
              {t('Image creation')}
            </h1>
          </div>
        </div>
        <div className='flex min-w-0 items-center gap-1 md:gap-2 max-md:[&>a]:px-2 max-md:[&>a]:text-[11px]'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            aria-label={t('New creation')}
            disabled={studio.busy}
            onClick={newCreation}
          >
            <HugeiconsIcon icon={Add01Icon} aria-hidden='true' />
            <span className='hidden md:inline'>{t('New creation')}</span>
          </Button>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='min-[75rem]:hidden'
            aria-label={t('Generation records')}
            aria-controls='studio-history'
            onClick={showHistory}
          >
            <HugeiconsIcon icon={Clock01Icon} aria-hidden='true' />
          </Button>
          <Button
            variant='outline'
            nativeButton={false}
            role='link'
            render={
              <Link
                to={props.authenticated ? '/dashboard' : '/sign-in'}
                search={
                  props.authenticated
                    ? undefined
                    : { redirect: '/image-studio' }
                }
              />
            }
          >
            {props.authenticated ? t('Back to Dashboard') : t('Sign in')}
          </Button>
        </div>
      </header>
      <div className='flex min-h-0 flex-1 flex-col overflow-y-auto min-[75rem]:grid min-[75rem]:grid-cols-[minmax(0,1fr)_300px] min-[75rem]:overflow-hidden'>
        <section
          aria-label={t('Creative canvas')}
          className='studio-center flex min-h-[calc(100svh-64px)] min-w-0 flex-none flex-col gap-5 px-4 py-6 md:gap-6 md:px-10 md:py-8 min-[75rem]:min-h-0 min-[75rem]:overflow-y-auto min-[75rem]:overscroll-contain'
        >
          <div
            className={cn(
              'mx-auto w-full max-w-[960px] min-w-0',
              views.hasResults ? 'studio-result-viewport' : 'mt-auto'
            )}
          >
            {views.canvas}
          </div>
          <form
            noValidate
            data-dragging={dragging}
            onDragOver={(event) => {
              if (
                !event.dataTransfer.types.includes('Files') &&
                !event.dataTransfer.types.includes('text/uri-list')
              ) {
                return
              }
              event.preventDefault()
              if (!studio.busy && props.authenticated) setDragging(true)
            }}
            onDragLeave={(event) => {
              if (
                !(event.relatedTarget instanceof Node) ||
                !event.currentTarget.contains(event.relatedTarget)
              ) {
                setDragging(false)
              }
            }}
            onDrop={(event) => {
              event.preventDefault()
              setDragging(false)
              setEditorMode('normal')
              const files = [...event.dataTransfer.files]
              const url = event.dataTransfer
                .getData('text/uri-list')
                .split('\n')
                .find((line) => /^https?:\/\//.test(line))
              if (files.length) studio.uploadReferences(files)
              else if (url) studio.uploadReferences([url.trim()])
            }}
            onSubmit={studio.submit}
            onChange={() => {
              studio.requestKey.current = ''
            }}
            className='studio-composer focus-within:border-ring data-[dragging=true]:border-primary data-[dragging=true]:ring-primary/30 mx-auto flex max-h-[70svh] w-full max-w-[960px] shrink-0 flex-col gap-3 overflow-y-auto rounded-2xl border border-(--studio-panel-border) bg-(--studio-panel) p-4 transition-colors data-[dragging=true]:ring-2 md:p-5'
            aria-label={t('Image prompt editor')}
          >
            <div className='studio-composer-top flex min-w-0 items-center justify-between gap-3'>
              <div className='flex min-w-0 items-center gap-2 text-xs'>
                <span className='bg-secondary text-primary grid size-6.5 shrink-0 place-items-center rounded-[7px] border'>
                  <HugeiconsIcon
                    icon={Image01Icon}
                    className='size-4'
                    aria-hidden='true'
                  />
                </span>
                <span className='truncate' title={studio.model}>
                  {props.authenticated
                    ? studio.model || t('No image models available')
                    : t('Sign in to choose a model')}
                </span>
              </div>
              <div className='flex items-center gap-1'>
                <StudioModelSettings
                  keys={props.keys}
                  userGroup={studio.userGroup}
                  image={{ tokenId: studio.tokenId, model: studio.model }}
                  text={{ ...studio.text, model: studio.textModel }}
                  imageModels={studio.models}
                  textModels={studio.textModels}
                  disabled={studio.busy || !props.authenticated}
                  onImageChange={(selection) => {
                    studio.requestKey.current = ''
                    studio.form.setValue('tokenId', String(selection.tokenId))
                    studio.form.setValue('model', selection.model)
                  }}
                  onTextChange={studio.setText}
                />
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  aria-label={
                    editorMode === 'expanded'
                      ? t('Restore prompt editor')
                      : t('Expand prompt editor')
                  }
                  aria-pressed={editorMode === 'expanded'}
                  onClick={(event) => {
                    event.currentTarget.form
                      ?.querySelector('textarea')
                      ?.style.removeProperty('height')
                    setEditorMode(
                      editorMode === 'expanded' ? 'normal' : 'expanded'
                    )
                  }}
                >
                  <HugeiconsIcon
                    icon={
                      editorMode === 'expanded'
                        ? MinimizeScreenIcon
                        : FullScreenIcon
                    }
                    aria-hidden='true'
                  />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-sm'
                  aria-label={
                    editorMode === 'collapsed'
                      ? t('Show prompt editor')
                      : t('Collapse prompt editor')
                  }
                  aria-expanded={editorMode !== 'collapsed'}
                  aria-controls='studio-editor-body'
                  onClick={() =>
                    setEditorMode(
                      editorMode === 'collapsed' ? 'normal' : 'collapsed'
                    )
                  }
                >
                  <HugeiconsIcon
                    icon={
                      editorMode === 'collapsed'
                        ? ArrowUp01Icon
                        : ArrowDown01Icon
                    }
                    aria-hidden='true'
                  />
                </Button>
              </div>
            </div>
            {editorMode === 'collapsed' && (
              <p className='text-muted-foreground truncate text-xs'>
                {studio.prompt || t('Describe your image')}
              </p>
            )}
            <div
              id='studio-editor-body'
              hidden={editorMode === 'collapsed'}
              className='flex flex-col gap-2.5'
            >
              {!props.settings.enabled && (
                <p
                  role='status'
                  className='text-muted-foreground text-[11px] leading-[1.6]'
                >
                  {t('Image generation is disabled')}
                </p>
              )}
              {props.authenticated && !props.keys.length && (
                <p
                  role='status'
                  className='text-muted-foreground text-[11px] leading-[1.6]'
                >
                  {t(
                    'No API keys available. Create your first API key to get started.'
                  )}{' '}
                  <Link
                    to='/keys'
                    className='text-primary underline underline-offset-4'
                  >
                    {t('API Keys')}
                  </Link>
                </p>
              )}
              {studio.models.isError && (
                <p role='alert' className='text-destructive text-xs'>
                  {t('Failed to load enabled models')}
                </p>
              )}
              <Field
                data-disabled={studio.busy}
                data-invalid={Boolean(studio.form.formState.errors.prompt)}
                className='gap-2'
              >
                <FieldLabel className='sr-only' htmlFor='studio-prompt'>
                  {t('Describe your image')}
                </FieldLabel>
                <Textarea
                  id='studio-prompt'
                  rows={3}
                  className={cn(
                    'studio-prompt-input [field-sizing:fixed] max-h-[50svh] min-h-24 resize-y rounded-none border-0 px-0 py-1 text-sm leading-[1.8]',
                    editorMode === 'expanded' && 'h-[35svh]'
                  )}
                  placeholder={t('Describe your image')}
                  {...studio.form.register('prompt')}
                  disabled={studio.busy}
                  aria-invalid={Boolean(studio.form.formState.errors.prompt)}
                  aria-describedby='studio-prompt-help'
                  maxLength={studio.maxCharacters}
                  onKeyDown={(event) => {
                    if (
                      (event.metaKey || event.ctrlKey) &&
                      event.key === 'Enter' &&
                      !event.nativeEvent.isComposing
                    ) {
                      event.preventDefault()
                      void studio.submit()
                    }
                  }}
                />
              </Field>
              <StudioReferenceImages
                references={studio.references}
                uploading={studio.upload.isPending}
                disabled={
                  studio.busy || !props.authenticated || !props.settings.enabled
                }
                maxImages={props.settings.max_reference_images}
                onUpload={studio.uploadReferences}
                onRemove={studio.removeReference}
              />
              {studio.upload.isError && (
                <p role='alert' className='text-destructive text-xs'>
                  {t(studioErrorMessage(studio.upload.error))}
                </p>
              )}
              {studio.optimize.isError && (
                <p role='alert' className='text-destructive text-xs'>
                  {studioErrorMessage(studio.optimize.error)}
                </p>
              )}
              {studio.generate.isError && (
                <p role='alert' className='text-destructive text-xs'>
                  {t(studioErrorMessage(studio.generate.error))}
                </p>
              )}
              {studio.optimize.isSuccess &&
                studio.prompt === studio.optimize.data.suggested_prompt && (
                  <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                    <span role='status'>{t('Prompt optimized')}</span>
                    <Button
                      type='button'
                      size='xs'
                      variant='ghost'
                      disabled={studio.busy}
                      onClick={() => {
                        if (studio.optimize.variables !== undefined) {
                          studio.changePrompt(studio.optimize.variables)
                        }
                        studio.optimize.reset()
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
              <div className='studio-composer-actions flex flex-wrap items-center justify-between gap-2 md:gap-2.5'>
                <div className='flex flex-wrap items-center gap-[3px] md:gap-2'>
                  <Button
                    type='button'
                    size='sm'
                    variant='ghost'
                    disabled={
                      !props.authenticated ||
                      !props.settings.enabled ||
                      studio.busy ||
                      !studio.textModel ||
                      studio.textModels.isFetching ||
                      !studio.prompt.trim() ||
                      studio.prompt.length > studio.maxCharacters
                    }
                    onClick={() => studio.optimize.mutate(studio.prompt)}
                  >
                    <HugeiconsIcon icon={SparklesIcon} aria-hidden='true' />
                    {studio.optimize.isPending
                      ? t('Optimizing...')
                      : t('Optimize prompt')}
                  </Button>
                </div>
                {props.authenticated ? (
                  <Button
                    type='submit'
                    size='lg'
                    className='min-h-9 min-w-[100px] md:min-w-[126px]'
                    disabled={!studio.canGenerate}
                  >
                    {studio.generating ? (
                      <Spinner />
                    ) : (
                      <HugeiconsIcon icon={SparklesIcon} aria-hidden='true' />
                    )}
                    {studio.generating
                      ? t('Generating...')
                      : t('Create images')}
                  </Button>
                ) : (
                  <Button
                    size='lg'
                    nativeButton={false}
                    role='link'
                    render={
                      <Link
                        to='/sign-in'
                        search={{ redirect: '/image-studio' }}
                      />
                    }
                  >
                    {t('Sign in to create images')}
                  </Button>
                )}
              </div>
              <div
                id='studio-prompt-help'
                className='text-muted-foreground flex min-w-0 justify-between gap-3 border-t pt-2 text-[9px]'
              >
                <span className='truncate'>
                  {t('Text model')}:{' '}
                  {props.authenticated
                    ? studio.textModel || t('No models available')
                    : t('Sign in to choose a model')}
                </span>
                <span className='shrink-0 tabular-nums'>
                  {studio.prompt.length} / {studio.maxCharacters}
                  <kbd className='ml-3 hidden font-[inherit] md:inline'>
                    ⌘ / Ctrl ↵
                  </kbd>
                </span>
              </div>
            </div>
          </form>
          <p
            className={cn(
              'text-muted-foreground -mt-2 shrink-0 text-center text-[10px] leading-relaxed',
              !views.hasResults && 'mb-auto'
            )}
          >
            {t(
              'Generated images are kept for 30 days, then automatically deleted. Please download them in time.'
            )}
          </p>
        </section>
        <div
          id='studio-history'
          ref={historyRef}
          tabIndex={-1}
          className='flex min-h-[280px] min-w-0 flex-none border-t bg-(--studio-surface) -outline-offset-3 min-[75rem]:min-h-0 min-[75rem]:border-t-0 min-[75rem]:border-l min-[75rem]:border-(--studio-subtle-border)'
        >
          {views.history}
        </div>
      </div>
    </>
  )
}
