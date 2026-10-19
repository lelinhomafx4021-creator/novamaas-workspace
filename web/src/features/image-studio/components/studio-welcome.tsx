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
      className='studio-welcome mx-auto flex min-h-full flex-col justify-center gap-4.5 md:gap-6 min-[100rem]:max-w-[1080px]'
      aria-label={t('Generation results')}
    >
      <div className='studio-hero relative isolate shrink-0 overflow-hidden rounded-[14px] border border-(--studio-subtle-border) bg-(--studio-art)'>
        <img
          className='absolute inset-0 size-full object-cover object-[62%_center] opacity-65 md:object-contain md:object-right md:opacity-100'
          src='/images/image-studio/orbit.jpg'
          alt=''
          fetchPriority='high'
          width={1600}
          height={900}
        />
        <div className='studio-hero-copy relative flex min-h-full w-[72%] flex-col items-start justify-center gap-[15px] p-6 md:w-[55%] md:gap-4.5 md:p-[clamp(24px,3vw,44px)]'>
          <p className='text-[9px] font-medium tracking-[0.16em] text-(--studio-eyebrow) uppercase md:text-[10px]'>
            {t('A new perspective')}
          </p>
          <h2 className='text-[31px] leading-[1.18] font-medium tracking-[-0.045em] text-balance text-(--studio-title) md:text-[clamp(28px,3vw,46px)]'>
            {t('Imagine it.')}
            <br />
            <span className='text-primary'>{t('Make it real.')}</span>
          </h2>
          <p className='max-w-[180px] text-[11px] leading-[1.8] text-pretty text-(--studio-description) md:max-w-60 md:text-xs'>
            {t(
              'Turn your ideas into extraordinary images. Start with a few words, and see where they take you.'
            )}
          </p>
          <span className='studio-hero-caption text-primary mt-[3px] inline-flex items-center gap-2 text-[9px] md:text-[10px]'>
            {t('Built for your imagination')}
          </span>
        </div>
        <span
          className='absolute right-3 bottom-3 font-mono text-[7px] tracking-[0.12em] text-(--studio-art-label) uppercase md:right-4.5 md:bottom-[15px] md:text-[8px]'
          aria-hidden='true'
        >
          01 / {t('Optical exploration')}
        </span>
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
