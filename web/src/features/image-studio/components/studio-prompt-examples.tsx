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
  Camera01Icon,
  CubeIcon,
  Image01Icon,
  PaintBrush01Icon,
  SparklesIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

export function StudioPromptExamples(props: {
  maxCharacters: number
  disabled: boolean
  onChange: (prompt: string) => void
}) {
  const { t } = useTranslation()
  const examples = [
    {
      name: t('Cinematic photography'),
      icon: Camera01Icon,
      prompt: t(
        'A ginger cat on a sunny windowsill, warm afternoon light, cinematic photography, soft shadows and a calm composition.'
      ),
    },
    {
      name: t('Anime illustration'),
      icon: Image01Icon,
      prompt: t(
        'A small seaside town at sunset, an anime illustration with glowing clouds, colorful rooftops and a gentle breeze.'
      ),
    },
    {
      name: t('3D product'),
      icon: CubeIcon,
      prompt: t(
        'A pastel ceramic coffee cup on a studio pedestal, a playful 3D product render with soft lighting and subtle reflections.'
      ),
    },
    {
      name: t('Ink painting'),
      icon: PaintBrush01Icon,
      prompt: t(
        'A mountain lake in morning mist, traditional ink painting, flowing brushwork and generous negative space.'
      ),
    },
  ]
  return (
    <section aria-label={t('Try an example')} className='studio-examples'>
      <h3 className='studio-examples-title'>
        <HugeiconsIcon
          icon={SparklesIcon}
          className='text-primary size-4'
          aria-hidden='true'
        />
        {t('Try an example')}
      </h3>
      <div className='studio-example-grid'>
        {examples.map((example, index) => (
          <button
            key={example.name}
            type='button'
            aria-label={example.name}
            title={example.prompt}
            disabled={
              props.disabled || example.prompt.length > props.maxCharacters
            }
            onClick={() => props.onChange(example.prompt)}
            className='studio-example-card'
          >
            <span className='studio-example-number' aria-hidden='true'>
              0{index + 1}
            </span>
            <HugeiconsIcon
              icon={example.icon}
              className='studio-example-icon size-5 shrink-0'
              aria-hidden='true'
            />
            <span className='grid min-w-0 gap-1'>
              <span className='text-xs font-medium'>{example.name}</span>
              <span className='text-muted-foreground line-clamp-2 text-[11px] leading-relaxed'>
                {example.prompt}
              </span>
            </span>
          </button>
        ))}
      </div>
    </section>
  )
}
