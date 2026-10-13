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
import type { NodeProps as FlowNodeProps } from '@xyflow/react'
import { Clapperboard, Trash2 } from 'lucide-react'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Node,
  NodeAction,
  NodeContent,
  NodeDescription,
  NodeHeader,
  NodeTitle,
} from '@/components/ai-elements/node'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { ReferenceThumbnail } from './reference-thumbnail'

/** Subscribes to its own draft only, so editing one node never re-renders others. */
export const GenerationNode = memo(function GenerationNode(
  props: FlowNodeProps
) {
  const { t } = useTranslation()
  const draft = useVideoStudioStore((state) => state.draft.nodes[props.id])
  const removeNode = useVideoStudioStore((state) => state.removeNode)
  if (!draft) return null

  return (
    <Node
      handles={{ target: false, source: true }}
      className={cn('w-72', props.selected && 'ring-primary ring-2')}
    >
      <NodeHeader>
        <NodeTitle className='flex items-center gap-1.5 text-sm'>
          <Clapperboard className='size-4' />
          {t('Generate video')}
        </NodeTitle>
        <NodeDescription className='truncate text-xs'>
          {draft.model || t('No model selected')}
        </NodeDescription>
        <NodeAction>
          <Button
            size='icon-xs'
            variant='ghost'
            className='nodrag'
            aria-label={t('Delete node')}
            onClick={(event) => {
              event.stopPropagation()
              removeNode(props.id)
            }}
          >
            <Trash2 />
          </Button>
        </NodeAction>
      </NodeHeader>
      <NodeContent className='grid gap-2'>
        <p className='text-muted-foreground line-clamp-3 text-xs'>
          {draft.prompt || t('No prompt yet')}
        </p>
        {draft.references.length > 0 && (
          <div className='flex gap-1'>
            {draft.references.map((reference, index) => (
              <div key={reference.id} className='relative'>
                <ReferenceThumbnail
                  source={reference.source}
                  id={reference.id}
                  label={reference.label}
                  className='size-10'
                />
                <span className='bg-background/80 absolute top-0 left-0 rounded-br px-1 text-[10px]'>
                  {index + 1}
                </span>
              </div>
            ))}
          </div>
        )}
      </NodeContent>
    </Node>
  )
})
