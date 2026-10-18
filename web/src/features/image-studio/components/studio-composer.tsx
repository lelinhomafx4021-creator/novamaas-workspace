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
      <header className='studio-header sticky top-0'>
        <div className='studio-brand'>
          <SystemBrand variant='inline' />
          <span className='studio-brand-divider' aria-hidden='true' />
          <div>
            <p className='studio-eyebrow'>{t('Image studio')}</p>
            <h1>{t('Image creation')}</h1>
          </div>
        </div>
        <div className='flex min-w-0 items-center gap-2'>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='studio-mobile-new'
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
            className='studio-history-toggle'
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
      <div className='studio-workspace'>
        <StudioSidebar
          authenticated={props.authenticated}
          disabled={studio.busy}
          onHistory={showHistory}
          onNewCreation={studio.newCreation}
        />
        <div className='studio-center'>
          <div className='studio-canvas-toolbar'>
            <div className='flex min-w-0 items-center gap-2'>
              <span className='studio-canvas-dot' aria-hidden='true' />
              <span>{t('Creative canvas')}</span>
            </div>
            <Badge variant='outline'>{t('Private workspace')}</Badge>
          </div>
          <div className='studio-canvas-scroll'>{views.canvas}</div>
          <form
            noValidate
            onSubmit={studio.submit}
            onChange={() => {
              studio.requestKey.current = ''
            }}
            className='studio-composer'
            aria-label={t('Image prompt editor')}
          >
            <div className='studio-composer-top'>
              <div className='studio-model-label'>
                <span className='studio-model-icon'>
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
              <p role='status' className='studio-composer-notice'>
                {t('Image generation is disabled')}
              </p>
            )}
            {props.authenticated && !props.keys.length && (
              <p role='status' className='studio-composer-notice'>
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
                className='studio-prompt-input'
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
            <div className='studio-composer-actions'>
              <div className='flex flex-wrap items-center gap-2'>
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
                  className='studio-default-size'
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
                  className='studio-generate'
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
            <div id='studio-prompt-help' className='studio-composer-help'>
              <span className='truncate'>
                {t('Text model')}:{' '}
                {props.authenticated
                  ? studio.textModel || t('No models available')
                  : t('Sign in to choose a model')}
              </span>
              <span className='shrink-0 tabular-nums'>
                {studio.prompt.length} / {studio.maxCharacters}
                <kbd className='ml-3'>⌘ / Ctrl ↵</kbd>
              </span>
            </div>
          </form>
          <p className='studio-retention'>
            {t(
              'Generated images are kept for 30 days, then automatically deleted. Please download them in time.'
            )}
          </p>
        </div>
        <div
          id='studio-history'
          ref={historyRef}
          tabIndex={-1}
          className='studio-history-panel'
        >
          {views.history}
        </div>
      </div>
    </>
  )
}
