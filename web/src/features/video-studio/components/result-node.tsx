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
import { useMutation, useQuery } from '@tanstack/react-query'
import type {
  Node as FlowNode,
  NodeProps as FlowNodeProps,
} from '@xyflow/react'
import { Download, RotateCcw } from 'lucide-react'
import { memo, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  Node,
  NodeContent,
  NodeDescription,
  NodeFooter,
  NodeHeader,
  NodeTitle,
} from '@/components/ai-elements/node'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  downloadTaskVideo,
  getTaskVideoContentInfo,
} from '@/features/usage-logs/task-content-api'

import { studioErrorMessage } from '../lib/errors'
import type { Run } from '../types'

export type ResultNodeData = {
  run: Run
  onRegenerate: (run: Run) => void
}

export type ResultFlowNode = FlowNode<ResultNodeData, 'result'>

const STATUS_LABELS: Record<Run['status'], string> = {
  submitting: 'Submitting',
  queued: 'Queued',
  in_progress: 'Generating',
  completed: 'Completed',
  failed: 'Failed',
  unknown: 'Unknown',
}

/** Content may be proxied (blob) or redirected (signed URL). */
async function resolveVideoURL(taskId: string) {
  const info = await getTaskVideoContentInfo(taskId)
  if (!info.success || !info.data) throw new Error(info.message)
  if (info.data.delivery_mode === 'redirect' && info.data.url) {
    return { url: info.data.url, revoke: false }
  }
  const response = await downloadTaskVideo(taskId)
  return { url: URL.createObjectURL(response.data), revoke: true }
}

export const ResultNode = memo(function ResultNode(
  props: FlowNodeProps<ResultFlowNode>
) {
  const { t } = useTranslation()
  const run = props.data.run
  const completed = run.status === 'completed'

  const video = useQuery({
    queryKey: ['video-studio', 'video', run.task_id],
    enabled: completed,
    staleTime: Infinity,
    queryFn: () => resolveVideoURL(run.task_id),
  })
  useEffect(() => {
    const source = video.data
    return () => {
      if (source?.revoke) URL.revokeObjectURL(source.url)
    }
  }, [video.data])

  const download = useMutation({
    mutationFn: async () => {
      const source = video.data ?? (await resolveVideoURL(run.task_id))
      const anchor = document.createElement('a')
      anchor.href = source.url
      anchor.download = `${run.task_id}.mp4`
      anchor.rel = 'noopener'
      document.body.append(anchor)
      anchor.click()
      anchor.remove()
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  return (
    <Node handles={{ target: true, source: false }} className='w-80'>
      <NodeHeader>
        <NodeTitle className='flex items-center justify-between gap-2 text-sm'>
          {t('Result')}
          <Badge
            variant={run.status === 'failed' ? 'destructive' : 'secondary'}
          >
            {t(STATUS_LABELS[run.status])}
          </Badge>
        </NodeTitle>
        <NodeDescription className='text-xs'>
          {new Date(run.created_at * 1000).toLocaleString()}
        </NodeDescription>
      </NodeHeader>
      <NodeContent className='grid gap-2'>
        {completed && video.data && (
          <video
            className='nodrag aspect-video w-full rounded-md bg-black'
            src={video.data.url}
            controls
            preload='metadata'
          />
        )}
        {completed && video.isError && (
          <p className='text-destructive text-xs'>
            {t('Video is not available yet.')}
          </p>
        )}
        {(run.status === 'queued' || run.status === 'in_progress') && (
          <p className='text-muted-foreground text-xs'>
            {t(
              'The task is running on the provider. This updates automatically.'
            )}
          </p>
        )}
        {run.status === 'failed' && (
          <p className='text-destructive text-xs break-words'>
            {run.fail_reason || t('Generation failed')}
          </p>
        )}
      </NodeContent>
      <NodeFooter className='nodrag flex justify-end gap-2 p-2'>
        <Button
          size='sm'
          variant='outline'
          onClick={() => props.data.onRegenerate(run)}
        >
          <RotateCcw />
          {t('Generate again')}
        </Button>
        {completed && (
          <Button
            size='sm'
            disabled={download.isPending}
            onClick={() => download.mutate()}
          >
            <Download />
            {t('Download')}
          </Button>
        )}
      </NodeFooter>
    </Node>
  )
})
