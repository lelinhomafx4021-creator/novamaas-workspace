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
import { useQuery } from '@tanstack/react-query'
import { ImageOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { getMediaAssetPreview } from '@/features/assets/api'
import { cn } from '@/lib/utils'

import { getReferencePreview } from '../api'
import type { ReferenceSource } from '../types'

type ReferenceThumbnailProps = {
  source: ReferenceSource
  id: string
  label: string
  className?: string
}

/** Signed preview URLs expire, so they are fetched on demand and cached briefly. */
export function ReferenceThumbnail(props: ReferenceThumbnailProps) {
  const { t } = useTranslation()
  const preview = useQuery({
    queryKey: ['video-studio', 'preview', props.source, props.id],
    queryFn: async () => {
      const response =
        props.source === 'seedance_asset'
          ? await getMediaAssetPreview(props.id, 'thumbnail')
          : await getReferencePreview(props.id)
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    staleTime: (query) =>
      Math.max(
        0,
        Math.min(
          10 * 60 * 1000,
          (query.state.data?.expires_at ?? 0) * 1000 - Date.now() - 30_000
        )
      ),
    refetchInterval: (query) =>
      query.state.status !== 'error' && query.state.data
        ? Math.max(
            30_000,
            query.state.data.expires_at * 1000 - Date.now() - 60_000
          )
        : false,
    retry: false,
  })

  return (
    <div
      className={cn(
        'bg-muted flex aspect-square items-center justify-center overflow-hidden rounded-md',
        props.className
      )}
    >
      {preview.data && !preview.isError ? (
        <img
          src={preview.data.url}
          alt={props.label}
          className='size-full object-cover'
          loading='lazy'
        />
      ) : (
        <ImageOff
          className='text-muted-foreground size-5'
          aria-label={preview.isError ? t('Preview expired') : props.label}
        />
      )}
    </div>
  )
}
