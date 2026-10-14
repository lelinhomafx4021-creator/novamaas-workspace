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
  ReactFlowProvider,
  type Edge,
  type Node,
  type NodeChange,
  type NodeTypes,
} from '@xyflow/react'
import { Clapperboard, Plus } from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { Canvas } from '@/components/ai-elements/canvas'
import { Button } from '@/components/ui/button'
import { useVideoStudioStore } from '@/stores/video-studio-store'

import { GenerationNode } from './generation-node'
import { ResultNode } from './result-node'

const NODE_TYPES: NodeTypes = { generation: GenerationNode, result: ResultNode }
// fitView would zoom a lone node to fill the screen; cap it at actual size.
const FIT_VIEW_OPTIONS = { maxZoom: 1, padding: 0.3 }

type StudioCanvasProps = {
  nodes: Node[]
  edges: Edge[]
  docked: boolean
  noModels: boolean
  onAddNode: () => void
}

export function StudioCanvas(props: StudioCanvasProps) {
  const { t } = useTranslation()

  const onNodesChange = useCallback((changes: NodeChange[]) => {
    const state = useVideoStudioStore.getState()
    for (const change of changes) {
      // Controlled nodes only follow the cursor if every drag frame is stored;
      // autosave is debounced, so this does not write per frame.
      if (change.type === 'position' && change.position) {
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
  }, [])

  return (
    <div className='relative min-w-0 flex-1'>
      <ReactFlowProvider key={props.docked ? 'wide' : 'narrow'}>
        <Canvas
          nodes={props.nodes}
          edges={props.edges}
          nodeTypes={NODE_TYPES}
          fitViewOptions={FIT_VIEW_OPTIONS}
          onNodesChange={onNodesChange}
          onPaneClick={() => useVideoStudioStore.getState().selectNode(null)}
          bgColor='transparent'
          nodesConnectable={false}
          panOnDrag
          proOptions={{ hideAttribution: true }}
        />
      </ReactFlowProvider>
      {props.nodes.length === 0 && (
        <div className='pointer-events-none absolute inset-0 flex items-center justify-center'>
          <div className='bg-card/80 pointer-events-auto flex max-w-sm flex-col items-center gap-3 rounded-xl border p-6 text-center shadow-[0_0_40px_-12px_var(--studio-glow)] backdrop-blur-xl'>
            <div className='bg-primary/10 text-primary rounded-full p-3 shadow-[0_0_20px_-4px_var(--studio-glow)]'>
              <Clapperboard className='size-6' />
            </div>
            <div className='space-y-1'>
              <p className='font-medium'>
                {props.noModels
                  ? t('No video models available')
                  : t('Start your first video')}
              </p>
              <p className='text-muted-foreground text-sm'>
                {props.noModels
                  ? t(
                      'Ask an administrator to enable a Seedance, Wan 3, or MiniMax-H3 channel for your group.'
                    )
                  : t(
                      'Add a generation node, write a prompt, and optionally attach reference images.'
                    )}
              </p>
            </div>
            <Button onClick={props.onAddNode}>
              <Plus />
              {t('Add generation node')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
