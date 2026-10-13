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
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { getUserGroups } from '@/features/playground/api'
import { useMediaQuery } from '@/hooks/use-media-query'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { getProject, listCapabilities, listRuns, submitVideo } from '../api'
import { useProjectAutosave } from '../hooks/use-project-autosave'
import {
  buildSubmitRequest,
  draftFromRun,
  isRunActive,
  newGenerationDraft,
} from '../lib/draft'
import { studioErrorMessage } from '../lib/errors'
import { buildFlowGraph } from '../lib/flow'
import type { Run } from '../types'
import { PropertiesPanel } from './properties-panel'
import { ReferencePanel } from './reference-panel'
import { StudioCanvas } from './studio-canvas'
import { StudioDock, STUDIO_DOCK_QUERY } from './studio-dock'
import { StudioToolbar } from './studio-toolbar'

const RUN_POLL_MS = 5000

function newNodeId() {
  return `gen_${crypto.randomUUID().slice(0, 8)}`
}

export function StudioEditor(props: { projectId: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const store = useVideoStudioStore
  const loadedProjectId = useVideoStudioStore((state) => state.projectId)
  const draft = useVideoStudioStore((state) => state.draft)
  const canvas = useVideoStudioStore((state) => state.canvas)
  const selectedNodeId = useVideoStudioStore((state) => state.selectedNodeId)
  // One request ID per explicit "Generate" click; network retries reuse it.
  const [pendingRequests, setPendingRequests] = useState<
    Record<string, string>
  >({})

  useProjectAutosave()
  const docked = useMediaQuery(STUDIO_DOCK_QUERY)
  const [referencesOpen, setReferencesOpen] = useState(false)
  const [propertiesOpen, setPropertiesOpen] = useState(false)

  const project = useQuery({
    queryKey: ['video-studio', 'project', props.projectId],
    queryFn: async () => {
      const response = await getProject(props.projectId)
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    refetchOnWindowFocus: false,
  })
  useEffect(() => {
    if (project.data) store.getState().load(project.data)
  }, [project.data, store])

  const capabilities = useQuery({
    queryKey: ['video-studio', 'capabilities'],
    queryFn: async () => {
      const response = await listCapabilities()
      if (!response.success) throw new Error(response.message)
      return response.data
    },
  })
  const groups = useQuery({
    queryKey: ['video-studio', 'groups'],
    queryFn: getUserGroups,
  })

  const runsQueryKey = ['video-studio', 'runs', props.projectId]
  const runs = useQuery({
    queryKey: runsQueryKey,
    queryFn: async () => {
      const response = await listRuns(props.projectId)
      if (!response.success) throw new Error(response.message)
      return response.data.items
    },
    // Poll only while some run is unfinished; the server tracks the task.
    refetchInterval: (query) =>
      query.state.data?.some(isRunActive) ? RUN_POLL_MS : false,
    refetchIntervalInBackground: false,
  })

  const clearPending = (nodeId: string) =>
    setPendingRequests((current) => {
      const next = { ...current }
      delete next[nodeId]
      return next
    })

  const submit = useMutation({
    mutationFn: async (nodeId: string) => {
      const nodeDraft = store.getState().draft.nodes[nodeId]
      const clientRequestId = pendingRequests[nodeId] ?? crypto.randomUUID()
      setPendingRequests((current) => ({
        ...current,
        [nodeId]: clientRequestId,
      }))
      const response = await submitVideo(
        buildSubmitRequest(props.projectId, nodeId, nodeDraft, clientRequestId)
      )
      if (!response.success) throw new Error(response.message)
      return nodeId
    },
    onSuccess: async (nodeId) => {
      clearPending(nodeId)
      await queryClient.invalidateQueries({ queryKey: runsQueryKey })
    },
    onError: async (error, nodeId) => {
      // The run was recorded as failed; the next click starts a new request.
      clearPending(nodeId)
      toast.error(studioErrorMessage(error))
      await queryClient.invalidateQueries({ queryKey: runsQueryKey })
    },
  })

  const addNode = useCallback(
    (initial = newGenerationDraft(capabilities.data?.[0])) => {
      const state = store.getState()
      const count = Object.keys(state.draft.nodes).length
      state.addNode(newNodeId(), initial, { x: 0, y: count * 190 })
    },
    [capabilities.data, store]
  )

  const regenerate = useCallback(
    (run: Run) => {
      const current = store.getState().draft.nodes[run.node_id]
      addNode(draftFromRun(run, current ?? newGenerationDraft()))
    },
    [addNode, store]
  )

  useEffect(() => {
    if (!docked && selectedNodeId) setPropertiesOpen(true)
  }, [docked, selectedNodeId])

  const graph = useMemo(
    () =>
      buildFlowGraph(
        draft,
        canvas,
        runs.data ?? [],
        selectedNodeId,
        regenerate
      ),
    [draft, canvas, runs.data, selectedNodeId, regenerate]
  )

  if (project.isError) {
    return (
      <div className='p-6'>
        <Alert variant='destructive'>
          <AlertTitle>{t('Project could not be opened')}</AlertTitle>
          <AlertDescription>
            {studioErrorMessage(project.error)}
          </AlertDescription>
        </Alert>
      </div>
    )
  }
  if (!project.data || loadedProjectId !== props.projectId) {
    return (
      <div className='text-muted-foreground p-6 text-sm'>{t('Loading...')}</div>
    )
  }

  const selectedModel = selectedNodeId
    ? draft.nodes[selectedNodeId]?.model
    : undefined
  const selectedCapability = capabilities.data?.find(
    (item) => item.model === selectedModel
  )

  return (
    <div className='bg-muted/30 flex h-full min-h-0 flex-col'>
      <StudioToolbar
        docked={docked}
        onOpenReferences={() => setReferencesOpen(true)}
        onOpenProperties={() => setPropertiesOpen(true)}
        onAddNode={() => addNode()}
        onReload={async () => {
          // Discards local edits in favour of the stored version.
          const latest = await project.refetch()
          if (latest.data) store.getState().load(latest.data)
        }}
      />
      <div className='flex min-h-0 flex-1'>
        <StudioDock
          docked={docked}
          side='left'
          title='References'
          open={referencesOpen}
          onOpenChange={setReferencesOpen}
        >
          <ReferencePanel
            supportsAssets={selectedCapability?.asset_library ?? false}
            defaultRole={selectedCapability?.roles[0]}
          />
        </StudioDock>
        <StudioCanvas
          nodes={graph.nodes}
          edges={graph.edges}
          noModels={capabilities.data?.length === 0}
          onAddNode={() => addNode()}
        />
        <StudioDock
          docked={docked}
          side='right'
          title='Node settings'
          open={propertiesOpen}
          onOpenChange={setPropertiesOpen}
        >
          <PropertiesPanel
            capabilities={capabilities.data ?? []}
            groups={groups.data ?? []}
            submitting={submit.isPending}
            onSubmit={(nodeId) => {
              setPropertiesOpen(false)
              submit.mutate(nodeId)
            }}
          />
        </StudioDock>
      </div>
    </div>
  )
}
