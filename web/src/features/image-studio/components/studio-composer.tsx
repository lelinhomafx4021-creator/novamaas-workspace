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
import { Image01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  useStudioCreation,
  type StudioCreationOptions,
} from '../hooks/use-studio-creation'
import { StudioCreationForm } from './studio-creation-form'
import { StudioModelSettings } from './studio-model-settings'

type StudioComposerProps = StudioCreationOptions & {
  children: (
    generating: boolean,
    reusePrompt: (prompt: string) => void
  ) => ReactNode
}

export function StudioComposer(props: StudioComposerProps) {
  const { t } = useTranslation()
  const creation = useStudioCreation(props)
  const busy = creation.generating || creation.optimize.isPending
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
          userGroup={creation.userGroup}
          image={{ tokenId: creation.tokenId, model: creation.model }}
          text={{ ...creation.textSelection, model: creation.textModel }}
          imageModels={creation.models}
          textModels={creation.textModels}
          disabled={busy}
          onImageChange={(selection) => {
            creation.resetRequestKey()
            creation.form.setValue('tokenId', String(selection.tokenId))
            creation.form.setValue('model', selection.model)
          }}
          onTextChange={creation.setTextSelection}
        />
      </header>
      <div className='grid shrink-0 items-start gap-3 p-3 sm:p-4 xl:min-h-0 xl:flex-1 xl:grid-cols-[17rem_minmax(0,1fr)_16rem] xl:grid-rows-[minmax(0,1fr)] xl:items-stretch 2xl:grid-cols-[18rem_minmax(0,1fr)_17rem]'>
        <StudioCreationForm
          settings={props.settings}
          hasKeys={props.keys.length > 0}
          creation={creation}
        />
        {props.children(busy, (value) => {
          if (busy || value.length > props.settings.max_prompt_characters) {
            return
          }
          creation.changePrompt(value)
          creation.optimize.reset()
          creation.form.setFocus('prompt')
        })}
      </div>
    </>
  )
}
