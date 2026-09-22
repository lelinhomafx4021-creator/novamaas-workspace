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
  AudioWaveformIcon,
  Delete02Icon,
  FileVideoIcon,
  Image01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { getMediaAssetPreview } from '../api'
import { assertAssetSuccess, formatAssetBytes } from '../asset-utils'
import type { MediaAsset } from '../types'

function AssetPreview(props: { asset: MediaAsset }) {
  const { t } = useTranslation()
  const previewQuery = useQuery({
    queryKey: ['asset-library', 'preview', props.asset.id],
    queryFn: async () =>
      assertAssetSuccess(await getMediaAssetPreview(props.asset.id)),
    staleTime: 5 * 60 * 1000,
  })

  if (previewQuery.isLoading) {
    return <Skeleton className='h-36 w-full rounded-none' />
  }
  if (!previewQuery.data?.url) {
    return (
      <div className='bg-muted text-muted-foreground flex h-36 items-center justify-center text-xs'>
        {t('Preview unavailable')}
      </div>
    )
  }
  if (props.asset.type === 'image') {
    return (
      <img
        src={previewQuery.data.url}
        alt={props.asset.name}
        loading='lazy'
        decoding='async'
        className='bg-muted/30 h-36 w-full object-contain p-2'
      />
    )
  }
  if (props.asset.type === 'video') {
    return (
      <video
        src={previewQuery.data.url}
        controls
        preload='metadata'
        className='bg-muted h-36 w-full object-contain'
      />
    )
  }
  return (
    <div className='bg-muted/40 flex h-36 flex-col items-center justify-center gap-3 px-3'>
      <HugeiconsIcon
        icon={AudioWaveformIcon}
        className='text-muted-foreground size-8'
      />
      <audio
        src={previewQuery.data.url}
        controls
        preload='metadata'
        className='h-8 w-full'
      />
    </div>
  )
}

function assetTypeIcon(type: MediaAsset['type']) {
  if (type === 'video') return FileVideoIcon
  if (type === 'audio') return AudioWaveformIcon
  return Image01Icon
}

function assetTypeLabelKey(type: MediaAsset['type']) {
  if (type === 'video') return 'Video'
  if (type === 'audio') return 'Audio'
  return 'Image'
}

export function AssetCard(props: {
  asset: MediaAsset
  groupName: string
  onDelete: (asset: MediaAsset) => void
}) {
  const { t } = useTranslation()
  const reference = `asset://${props.asset.id}`
  const deleteButton = (
    <Button
      type='button'
      size='icon-sm'
      variant='ghost'
      aria-label={t('Delete asset')}
      onClick={() => props.onDelete(props.asset)}
    >
      <HugeiconsIcon icon={Delete02Icon} />
    </Button>
  )

  return (
    <Card
      size='sm'
      className='overflow-hidden [contain-intrinsic-size:320px] [content-visibility:auto]'
    >
      <AssetPreview asset={props.asset} />
      <CardHeader className='gap-1.5'>
        <div className='flex items-start justify-between gap-2'>
          <CardTitle className='min-w-0 truncate'>{props.asset.name}</CardTitle>
          <Badge variant='outline' className='shrink-0'>
            <HugeiconsIcon icon={assetTypeIcon(props.asset.type)} />
            {t(assetTypeLabelKey(props.asset.type))}
          </Badge>
        </div>
        <p className='text-muted-foreground truncate text-xs'>
          {props.groupName} · {formatAssetBytes(props.asset.size)}
        </p>
      </CardHeader>
      <CardContent>
        <code className='bg-muted block truncate rounded-md px-2 py-1.5 text-[11px]'>
          {reference}
        </code>
      </CardContent>
      <CardFooter className='justify-end gap-1'>
        <CopyButton
          value={reference}
          size='icon'
          className='size-7'
          tooltip={t('Copy asset reference')}
          successTooltip={t('Asset reference copied')}
        />
        <Tooltip>
          <TooltipTrigger render={deleteButton} />
          <TooltipContent>{t('Delete asset')}</TooltipContent>
        </Tooltip>
      </CardFooter>
    </Card>
  )
}
