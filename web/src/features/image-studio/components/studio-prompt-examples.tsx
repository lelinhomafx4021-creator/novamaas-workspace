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
    <section
      aria-label={t('Try an example')}
      className='flex flex-col items-start gap-2.5 sm:flex-row sm:flex-wrap sm:items-center'
    >
      <h3 className='text-muted-foreground mr-1 flex items-center gap-1.5 text-xs font-normal'>
        <HugeiconsIcon
          icon={SparklesIcon}
          className='text-primary size-4'
          aria-hidden='true'
        />
        {t('Try an example')}
      </h3>
      <div className='grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:flex-wrap'>
        {examples.map((example) => (
          <button
            key={example.name}
            type='button'
            aria-label={example.name}
            title={example.prompt}
            disabled={
              props.disabled || example.prompt.length > props.maxCharacters
            }
            onClick={() => props.onChange(example.prompt)}
            className='studio-example-card hover:border-ring hover:bg-muted focus-visible:outline-ring inline-flex min-h-9 items-center gap-2 rounded-lg border bg-(--studio-surface) px-3 py-2 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-3 disabled:cursor-not-allowed disabled:opacity-45'
          >
            <HugeiconsIcon
              icon={example.icon}
              className='studio-example-icon text-primary size-4 shrink-0'
              aria-hidden='true'
            />
            {example.name}
          </button>
        ))}
      </div>
    </section>
  )
}
