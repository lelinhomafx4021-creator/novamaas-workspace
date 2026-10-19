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
import { useTranslation } from 'react-i18next'

import { StudioPromptExamples } from './studio-prompt-examples'

export function StudioWelcome(props: {
  disabled: boolean
  maxCharacters: number
  onPrompt?: (prompt: string) => void
}) {
  const { t } = useTranslation()

  return (
    <section
      className='studio-welcome flex flex-col gap-5 md:gap-6'
      aria-label={t('Generation results')}
    >
      <div className='studio-hero relative isolate flex min-h-48 items-center md:min-h-56'>
        <img
          className='studio-hero-art pointer-events-none absolute inset-y-0 right-0 h-full w-[65%] object-cover object-right opacity-35 md:w-[58%] md:opacity-90'
          src='/images/image-studio/orbit.jpg'
          alt=''
          fetchPriority='high'
          width={1600}
          height={900}
        />
        <div className='studio-hero-copy relative flex w-[82%] flex-col items-start gap-3 py-3 md:w-[55%] md:gap-4'>
          <p className='text-[9px] font-medium tracking-[0.16em] text-(--studio-eyebrow) uppercase md:text-[10px]'>
            {t('A new perspective')}
          </p>
          <h2 className='text-[30px] leading-[1.25] font-medium tracking-[-0.045em] text-balance text-(--studio-title) md:text-[38px]'>
            {t('Imagine it.')}
            <br />
            <span className='text-primary'>{t('Make it real.')}</span>
          </h2>
          <p className='max-w-60 text-xs leading-[1.8] text-pretty text-(--studio-description) md:max-w-72'>
            {t(
              'Turn your ideas into extraordinary images. Start with a few words, and see where they take you.'
            )}
          </p>
        </div>
      </div>
      {props.onPrompt && (
        <StudioPromptExamples
          maxCharacters={props.maxCharacters}
          disabled={props.disabled}
          onChange={props.onPrompt}
        />
      )}
    </section>
  )
}
