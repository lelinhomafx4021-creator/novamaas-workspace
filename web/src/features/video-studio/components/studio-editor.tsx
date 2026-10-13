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
import { Link } from '@tanstack/react-router'
import {
  ReactFlowProvider,
  type NodeChange,
  type NodeTypes,
} from '@xyflow/react'
import { ArrowLeft, Plus } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Canvas } from '@/components/ai-elements/canvas'
import { Panel } from '@/components/ai-elements/panel'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getUserGroups } from '@/features/playground/api'
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
import { GenerationNode } from './generation-node'
import { PropertiesPanel } from './properties-panel'
import { ReferencePanel } from './reference-panel'
import { ResultNode } from './result-node'

const NODE_TYPES: NodeTypes = { generation: GenerationNode, result: ResultNode }
const RUN_POLL_MS = 5000

const SAVE_LABELS = {
  saved: 'Saved',
  dirty: 'Unsaved changes',
  saving: 'Saving...',
  conflict: 'Edited elsewhere',
  error: 'Save failed',
} as const

function newNodeId() {
  return `gen_${crypto.randomUUID().slice(0, 8)}`
}

export function StudioEditor(props: { projectId: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const store = useVideoStudioStore
  const loadedProjectId = useVideoStudioStore((state) => state.projectId)
  const name = useVideoStudioStore((state) => state.name)
  const draft = useVideoStudioStore((state) => state.draft)
  const canvas = useVideoStudioStore((state) => state.canvas)
  const selectedNodeId = useVideoStudioStore((state) => state.selectedNodeId)
  const saveState = useVideoStudioStore((state) => state.saveState)
  // One request ID per explicit "Generate" click; network retries reuse it.
  const [pendingRequests, setPendingRequests] = useState<
    Record<string, string>
  >({})

  useProjectAutosave()

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
      return { nodeId, run: response.data }
    },
    onSuccess: async ({ nodeId }) => {
      setPendingRequests((current) => {
        const next = { ...current }
        delete next[nodeId]
        return next
      })
      await queryClient.invalidateQueries({ queryKey: runsQueryKey })
    },
    onError: async (error, nodeId) => {
      // The run was recorded as failed; the next click starts a new request.
      setPendingRequests((current) => {
        const next = { ...current }
        delete next[nodeId]
        return next
      })
      toast.error(studioErrorMessage(error))
      await queryClient.invalidateQueries({ queryKey: runsQueryKey })
    },
  })

  const addNode = useCallback(
    (initial = newGenerationDraft(capabilities.data?.[0])) => {
      const state = store.getState()
      const count = Object.keys(state.draft.nodes).length
      state.addNode(newNodeId(), initial, { x: 0, y: count * 360 })
    },
    [capabilities.data, store]
  )

  const regenerate = useCallback(
    (run: Run) => {
      const current = store.getState().draft.nodes[run.node_id]
      const restored = draftFromRun(run, current ?? newGenerationDraft())
      addNode(restored)
    },
    [addNode, store]
  )

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

  const onNodesChange = useCallback(
    (changes: NodeChange[]) => {
      const state = store.getState()
      for (const change of changes) {
        // Persist only when a drag ends, not on every intermediate frame.
        if (
          change.type === 'position' &&
          change.position &&
          change.dragging === false
        ) {
          state.moveNode(change.id, change.position)
        }
        if (change.type === 'select' && state.draft.nodes[change.id]) {
          if (change.selected) state.selectNode(change.id)
          else if (state.selectedNodeId === change.id) state.selectNode(null)
        }
        if (change.type === 'remove' && state.draft.nodes[change.id]) {
          state.removeNode(change.id)
        }
      }
    },
    [store]
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
    <div className='flex h-full min-h-0 flex-col'>
      <header className='flex items-center gap-3 border-b px-4 py-2'>
        <Button
          size='icon-sm'
          variant='ghost'
          aria-label={t('Back to projects')}
          render={<Link to='/video-studio' />}
        >
          <ArrowLeft />
        </Button>
        <Input
          aria-label={t('Project name')}
          className='h-8 max-w-64'
          value={name}
          maxLength={128}
          onChange={(event) => store.getState().setName(event.target.value)}
        />
        <span
          className='text-muted-foreground text-xs'
          role='status'
          aria-live='polite'
        >
          {t(SAVE_LABELS[saveState])}
        </span>
        {saveState === 'conflict' && (
          <Button
            size='sm'
            variant='outline'
            onClick={async () => {
              // Discards local edits in favour of the stored version.
              const latest = await project.refetch()
              if (latest.data) store.getState().load(latest.data)
            }}
          >
            {t('Reload latest version')}
          </Button>
        )}
      </header>
      {capabilities.data?.length === 0 && (
        <Alert className='m-3 w-auto'>
          <AlertTitle>{t('No video models available')}</AlertTitle>
          <AlertDescription>
            {t(
              'Ask an administrator to enable a Seedance, Wan 3, or MiniMax-H3 channel for your group.'
            )}
          </AlertDescription>
        </Alert>
      )}
      <div className='flex min-h-0 flex-1'>
        <ReferencePanel
          supportsAssets={selectedCapability?.asset_library ?? false}
          defaultRole={selectedCapability?.roles[0]}
        />
        <div className='min-w-0 flex-1'>
          <ReactFlowProvider>
            <Canvas
              nodes={graph.nodes}
              edges={graph.edges}
              nodeTypes={NODE_TYPES}
              onNodesChange={onNodesChange}
              onPaneClick={() => store.getState().selectNode(null)}
              nodesConnectable={false}
              panOnDrag
            >
              <Panel position='top-left'>
                <Button size='sm' onClick={() => addNode()}>
                  <Plus />
                  {t('Add generation node')}
                </Button>
              </Panel>
            </Canvas>
          </ReactFlowProvider>
        </div>
        <PropertiesPanel
          capabilities={capabilities.data ?? []}
          groups={groups.data ?? []}
          submitting={submit.isPending}
          onSubmit={(nodeId) => submit.mutate(nodeId)}
        />
      </div>
    </div>
  )
}
