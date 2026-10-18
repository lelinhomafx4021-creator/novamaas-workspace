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
    <section className='studio-welcome' aria-label={t('Generation results')}>
      <div className='studio-hero'>
        <img
          className='studio-hero-art'
          src='/images/image-studio/orbit.jpg'
          alt=''
          fetchPriority='high'
          width={1600}
          height={900}
        />
        <div className='studio-hero-copy'>
          <p className='studio-eyebrow'>{t('A new perspective')}</p>
          <h2>
            {t('Imagine it.')}
            <br />
            <span>{t('Make it real.')}</span>
          </h2>
          <p>
            {t(
              'Turn your ideas into extraordinary images. Start with a few words, and see where they take you.'
            )}
          </p>
          <span className='studio-hero-caption'>
            {t('Built for your imagination')}
          </span>
        </div>
        <span className='studio-art-label' aria-hidden='true'>
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
