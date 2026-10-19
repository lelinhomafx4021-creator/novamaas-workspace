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
import { Image01Icon, Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import type { StudioReference } from '../types'

export function StudioReferenceImages(props: {
  references: StudioReference[]
  uploading: boolean
  disabled: boolean
  maxImages: number
  onUpload: (files: File[]) => void
  onRemove: (id: string) => void
}) {
  const { t } = useTranslation()
  const input = useRef<HTMLInputElement>(null)

  return (
    <section aria-label={t('Reference images')} className='grid gap-2'>
      <input
        ref={input}
        type='file'
        accept='image/png,image/jpeg,image/webp'
        multiple
        className='sr-only'
        tabIndex={-1}
        aria-label={t('Upload reference images')}
        disabled={props.disabled || props.references.length >= props.maxImages}
        onChange={(event) => {
          props.onUpload([...(event.currentTarget.files || [])])
          event.currentTarget.value = ''
        }}
      />
      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={
            props.disabled || props.references.length >= props.maxImages
          }
          aria-busy={props.uploading}
          onClick={() => input.current?.click()}
        >
          {props.uploading ? (
            <Spinner />
          ) : (
            <HugeiconsIcon icon={Image01Icon} aria-hidden='true' />
          )}
          {props.uploading ? t('Uploading...') : t('Add reference images')}
        </Button>
        <span className='text-muted-foreground text-[10px]'>
          {t('Drop images here. Uploads are kept for 7 days.')}
        </span>
      </div>
      {props.references.length > 0 && (
        <div className='flex flex-wrap gap-2'>
          {props.references.map((reference, index) => (
            <div
              key={reference.id}
              className='bg-background relative size-16 overflow-hidden rounded-lg border'
            >
              <img
                src={reference.url}
                alt={t('Reference image {{index}}', { index: index + 1 })}
                className='size-full object-cover'
              />
              <Button
                type='button'
                variant='secondary'
                size='icon-xs'
                className='absolute top-0.5 right-0.5'
                aria-label={t('Remove reference image {{index}}', {
                  index: index + 1,
                })}
                disabled={props.disabled}
                onClick={() => props.onRemove(reference.id)}
              >
                <HugeiconsIcon icon={Cancel01Icon} aria-hidden='true' />
              </Button>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
