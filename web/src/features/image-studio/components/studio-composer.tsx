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
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { SystemBrand } from '@/components/layout/components/system-brand'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Spinner } from '@/components/ui/spinner'
import { Textarea } from '@/components/ui/textarea'
import type { ApiKey } from '@/features/keys/types'

import { useStudioComposer } from '../hooks/use-studio-composer'
import { studioErrorMessage } from '../studio-utils'
import type { StudioRun, StudioSettings } from '../types'
import { StudioModelSettings } from './studio-model-settings'
import { StudioSidebar } from './studio-sidebar'

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
  ) => { canvas: ReactNode; history: ReactNode }
}) {
  const { t } = useTranslation()
  const studio = useStudioComposer(props)
  const historyRef = useRef<HTMLDivElement>(null)
  const views = props.children(studio.busy, studio.reusePrompt)
  const showHistory = () => {
    historyRef.current?.scrollIntoView({ block: 'start' })
    historyRef.current?.focus({ preventScroll: true })
  }

  return (
    <>
      <header className='sticky top-0 z-10 flex shrink-0 basis-[68px] items-center justify-between gap-2 border-b border-(--studio-subtle-border) bg-(--studio-surface) px-3 py-2.5 md:basis-[76px] md:gap-4 md:px-6 md:py-3'>
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
            size='icon'
            className='md:hidden'
            aria-label={t('New creation')}
            disabled={studio.busy}
            onClick={studio.newCreation}
          >
            <HugeiconsIcon icon={Add01Icon} aria-hidden='true' />
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
      <div className='grid min-h-0 flex-1 auto-rows-max grid-cols-[180px_minmax(0,1fr)] content-start overflow-y-auto max-md:flex max-md:flex-col min-[75rem]:auto-rows-auto min-[75rem]:grid-cols-[208px_minmax(0,1fr)_288px] min-[75rem]:content-normal min-[75rem]:overflow-hidden min-[100rem]:grid-cols-[224px_minmax(0,1fr)_320px]'>
        <StudioSidebar
          authenticated={props.authenticated}
          disabled={studio.busy}
          onHistory={showHistory}
          onNewCreation={studio.newCreation}
        />
        <div className='studio-center col-start-2 flex min-h-[calc(100svh-68px)] min-w-0 flex-none flex-col md:min-h-[calc(100svh-76px)] min-[75rem]:min-h-0'>
          <div className='studio-canvas-toolbar text-muted-foreground flex min-h-12 shrink-0 items-center justify-between gap-3 px-4 text-[10px] md:min-h-[58px] md:px-7 md:text-[11px]'>
            <div className='flex min-w-0 items-center gap-2'>
              <span
                className='bg-primary size-1 shrink-0 rounded-full shadow-[0_0_8px_#b7dbff66]'
                aria-hidden='true'
              />
              <span>{t('Creative canvas')}</span>
            </div>
            <Badge variant='outline'>{t('Private workspace')}</Badge>
          </div>
          <div className='min-h-0 flex-none overflow-visible px-3.5 pb-4.5 md:px-7 md:pb-5 min-[75rem]:flex-1 min-[75rem]:overflow-y-auto min-[75rem]:overscroll-contain min-[100rem]:px-10'>
            {views.canvas}
          </div>
          <form
            noValidate
            onSubmit={studio.submit}
            onChange={() => {
              studio.requestKey.current = ''
            }}
            className='studio-composer focus-within:border-ring mx-3.5 mt-auto flex shrink-0 flex-col gap-2.5 rounded-[14px] border border-(--studio-panel-border) bg-(--studio-panel) px-3 pt-3.5 pb-2.5 transition-colors md:mx-7 md:px-4 min-[75rem]:mt-0 min-[100rem]:mx-10'
            aria-label={t('Image prompt editor')}
          >
            <div className='studio-composer-top flex min-w-0 items-center justify-between gap-3'>
              <div className='flex min-w-0 items-center gap-2 text-[10px] md:text-[11px]'>
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
            </div>
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
                className='studio-prompt-input [field-sizing:fixed] max-h-[180px] min-h-[84px] resize-y rounded-none border-0 px-0 py-1 leading-[1.8] md:min-h-[70px] md:text-[13px]'
                placeholder={t(
                  'Describe the subject, scene, lighting and style, or try an example below.'
                )}
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
            {studio.optimize.isError && (
              <p role='alert' className='text-destructive text-xs'>
                {studioErrorMessage(studio.optimize.error)}
              </p>
            )}
            {studio.generate.isError && (
              <p role='alert' className='text-destructive text-xs'>
                {studioErrorMessage(studio.generate.error)}
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
                <span
                  className='text-muted-foreground rounded-md border px-[5px] py-1 text-[9px] md:px-2 md:text-[10px]'
                  title={t(
                    'Model defaults apply; you can also specify proportions in your prompt.'
                  )}
                >
                  {t('Model default')}
                </span>
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
                  {studio.generating ? t('Generating...') : t('Create images')}
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
          </form>
          <p className='text-muted-foreground shrink-0 px-4.5 py-3 text-center text-[9px] leading-normal md:px-7'>
            {t(
              'Generated images are kept for 30 days, then automatically deleted. Please download them in time.'
            )}
          </p>
        </div>
        <div
          id='studio-history'
          ref={historyRef}
          tabIndex={-1}
          className='col-start-2 flex min-h-[280px] min-w-0 flex-none border-t bg-(--studio-surface) -outline-offset-3 min-[75rem]:col-auto min-[75rem]:min-h-0 min-[75rem]:border-t-0 min-[75rem]:border-l min-[75rem]:border-(--studio-subtle-border)'
        >
          {views.history}
        </div>
      </div>
    </>
  )
}
