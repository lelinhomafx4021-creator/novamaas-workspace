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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { listMediaAssets } from '@/features/assets/api'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { listReferences, saveReference, uploadReference } from '../api'
import { studioErrorMessage } from '../lib/errors'
import type { NodeReference, ReferenceRole, StudioReference } from '../types'
import { ReferenceThumbnail } from './reference-thumbnail'

const LIBRARY_QUERY_KEY = ['video-studio', 'references'] as const

type ReferencePanelProps = {
  supportsAssets: boolean
  defaultRole: ReferenceRole | undefined
}

function formatExpiry(expiresAt: number) {
  return new Date(expiresAt * 1000).toLocaleString()
}

/** Left panel: uploads, the user's saved library, and Seedance assets. */
export function ReferencePanel(props: ReferencePanelProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const inputRef = useRef<HTMLInputElement>(null)
  const [uploads, setUploads] = useState<StudioReference[]>([])
  const selectedNodeId = useVideoStudioStore((state) => state.selectedNodeId)
  const selectedDraft = useVideoStudioStore((state) =>
    state.selectedNodeId ? state.draft.nodes[state.selectedNodeId] : undefined
  )
  const updateNode = useVideoStudioStore((state) => state.updateNode)

  const library = useQuery({
    queryKey: LIBRARY_QUERY_KEY,
    queryFn: async () => {
      const response = await listReferences(1, 60)
      if (!response.success) throw new Error(response.message)
      return response.data.items
    },
  })
  const assets = useQuery({
    queryKey: ['video-studio', 'seedance-assets'],
    enabled: props.supportsAssets,
    queryFn: async () => {
      const response = await listMediaAssets({ page: 1, pageSize: 60 })
      if (!response.success) throw new Error(response.message)
      return response.data.items.filter(
        (asset) => asset.type === 'image' && asset.status === 'ready'
      )
    },
  })

  const upload = useMutation({
    mutationFn: async (files: File[]) => {
      const uploaded: StudioReference[] = []
      for (const file of files) {
        const response = await uploadReference(file)
        if (!response.success) throw new Error(response.message)
        uploaded.push(response.data)
      }
      return uploaded
    },
    onSuccess: (uploaded) => {
      setUploads((current) => [
        ...uploaded,
        ...current.filter((item) => !uploaded.some((u) => u.id === item.id)),
      ])
      if (selectedNodeId && props.defaultRole) {
        for (const reference of uploaded) {
          attach({
            source: 'storage',
            id: reference.id,
            label: reference.file_name,
          })
        }
      }
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  const save = useMutation({
    mutationFn: async (id: string) => {
      const response = await saveReference(id)
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    onSuccess: async (saved) => {
      setUploads((current) =>
        current.map((item) => (item.id === saved.id ? saved : item))
      )
      await queryClient.invalidateQueries({ queryKey: LIBRARY_QUERY_KEY })
      toast.success(t('Saved to your library'))
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  const attach = (reference: Omit<NodeReference, 'role'>) => {
    const state = useVideoStudioStore.getState()
    const nodeId = state.selectedNodeId
    const draft = nodeId ? state.draft.nodes[nodeId] : undefined
    if (!nodeId || !draft || !props.defaultRole) {
      toast.info(t('Select a generation node first'))
      return
    }
    if (draft.references.some((item) => item.id === reference.id)) return
    updateNode(nodeId, {
      references: [
        ...draft.references,
        { ...reference, role: props.defaultRole },
      ],
    })
  }

  const renderReference = (reference: StudioReference) => (
    <div key={reference.id} className='flex flex-col gap-1'>
      <button
        type='button'
        className='focus-visible:ring-ring rounded-md focus-visible:ring-2 focus-visible:outline-none'
        onClick={() =>
          attach({
            source: 'storage',
            id: reference.id,
            label: reference.file_name,
          })
        }
        aria-label={t('Add {{name}} to the selected node', {
          name: reference.file_name,
        })}
      >
        <ReferenceThumbnail
          source='storage'
          id={reference.id}
          label={reference.file_name}
        />
      </button>
      {reference.saved ? (
        <Badge variant='secondary'>{t('Saved')}</Badge>
      ) : (
        <Button
          size='xs'
          variant='outline'
          disabled={save.isPending}
          title={t('Expires at {{time}}', {
            time: formatExpiry(reference.expires_at),
          })}
          onClick={() => save.mutate(reference.id)}
        >
          {t('Save to library')}
        </Button>
      )}
    </div>
  )

  return (
    <aside className='bg-background flex w-64 shrink-0 flex-col border-r'>
      <div className='flex items-center justify-between gap-2 border-b p-3'>
        <span className='font-mono text-xs tracking-wider uppercase'>
          {t('References')}
        </span>
        {upload.isPending && (
          <span className='text-muted-foreground font-mono text-[10px]'>
            {t('Uploading...')}
          </span>
        )}
        <input
          ref={inputRef}
          type='file'
          accept='image/jpeg,image/png,image/webp'
          multiple
          hidden
          onChange={(event) => {
            const files = [...(event.target.files ?? [])]
            event.target.value = ''
            if (files.length > 0) upload.mutate(files)
          }}
        />
      </div>
      <button
        type='button'
        disabled={upload.isPending}
        onClick={() => inputRef.current?.click()}
        className='border-border text-muted-foreground hover:border-primary/60 hover:text-primary focus-visible:ring-ring mx-3 mt-3 flex flex-col items-center gap-1 rounded-lg border border-dashed px-3 py-5 text-xs transition-colors focus-visible:ring-2 focus-visible:outline-none disabled:opacity-50'
      >
        <Upload className='size-5' />
        {t('Drop or click to upload images')}
        <span className='text-[10px] opacity-70'>JPEG / PNG / WebP</span>
      </button>
      {!selectedDraft && (
        <p className='text-muted-foreground px-3 pt-3 text-[11px]'>
          {t('Select a generation node, then click an image to attach it.')}
        </p>
      )}
      <Tabs defaultValue='uploads' className='min-h-0 flex-1 p-3'>
        <TabsList className='w-full'>
          <TabsTrigger value='uploads'>{t('Recent')}</TabsTrigger>
          <TabsTrigger value='library'>{t('Library')}</TabsTrigger>
          {props.supportsAssets && (
            <TabsTrigger value='assets'>{t('Seedance')}</TabsTrigger>
          )}
        </TabsList>
        <ScrollArea className='min-h-0 flex-1'>
          <TabsContent value='uploads' className='grid grid-cols-2 gap-2 pt-2'>
            {uploads.length === 0 && (
              <p className='text-muted-foreground col-span-2 text-xs'>
                {t('Uploaded images are kept temporarily until you save them.')}
              </p>
            )}
            {uploads.map(renderReference)}
          </TabsContent>
          <TabsContent value='library' className='grid grid-cols-2 gap-2 pt-2'>
            {library.data?.length === 0 && (
              <p className='text-muted-foreground col-span-2 text-xs'>
                {t('No saved references yet.')}
              </p>
            )}
            {library.data?.map(renderReference)}
          </TabsContent>
          {props.supportsAssets && (
            <TabsContent value='assets' className='grid grid-cols-2 gap-2 pt-2'>
              {assets.data?.length === 0 && (
                <p className='text-muted-foreground col-span-2 text-xs'>
                  {t('No ready Seedance image assets.')}
                </p>
              )}
              {assets.data?.map((asset) => (
                <button
                  key={asset.id}
                  type='button'
                  className='focus-visible:ring-ring flex flex-col gap-1 rounded-md text-left focus-visible:ring-2 focus-visible:outline-none'
                  onClick={() =>
                    attach({
                      source: 'seedance_asset',
                      id: asset.id,
                      label: asset.name,
                    })
                  }
                  aria-label={t('Add {{name}} to the selected node', {
                    name: asset.name,
                  })}
                >
                  <ReferenceThumbnail
                    source='seedance_asset'
                    id={asset.id}
                    label={asset.name}
                  />
                  <span className='truncate text-xs'>{asset.name}</span>
                </button>
              ))}
            </TabsContent>
          )}
        </ScrollArea>
      </Tabs>
    </aside>
  )
}
