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
import { create } from 'zustand'

import {
  EMPTY_CANVAS,
  EMPTY_DRAFT,
  parseCanvas,
  parseDraft,
} from '@/features/video-studio/lib/draft'
import type {
  GenerationDraft,
  Project,
  ProjectCanvas,
  ProjectDraft,
} from '@/features/video-studio/types'

export type SaveState = 'saved' | 'dirty' | 'saving' | 'conflict' | 'error'

type VideoStudioState = {
  projectId: string
  name: string
  revision: number
  draft: ProjectDraft
  canvas: ProjectCanvas
  selectedNodeId: string | null
  saveState: SaveState
  load: (project: Project) => void
  setName: (name: string) => void
  addNode: (
    nodeId: string,
    draft: GenerationDraft,
    position: { x: number; y: number }
  ) => void
  updateNode: (nodeId: string, patch: Partial<GenerationDraft>) => void
  removeNode: (nodeId: string) => void
  moveNode: (nodeId: string, position: { x: number; y: number }) => void
  selectNode: (nodeId: string | null) => void
  markSaving: () => void
  markSaved: (revision: number) => void
  markFailed: (conflict: boolean) => void
}

export const useVideoStudioStore = create<VideoStudioState>((set) => ({
  projectId: '',
  name: '',
  revision: 0,
  draft: EMPTY_DRAFT,
  canvas: EMPTY_CANVAS,
  selectedNodeId: null,
  saveState: 'saved',
  load: (project) =>
    set({
      projectId: project.id,
      name: project.name,
      revision: project.revision,
      draft: parseDraft(project.draft),
      canvas: parseCanvas(project.canvas),
      selectedNodeId: null,
      saveState: 'saved',
    }),
  setName: (name) => set({ name, saveState: 'dirty' }),
  addNode: (nodeId, draft, position) =>
    set((state) => ({
      draft: {
        ...state.draft,
        nodes: { ...state.draft.nodes, [nodeId]: draft },
      },
      canvas: {
        ...state.canvas,
        positions: { ...state.canvas.positions, [nodeId]: position },
      },
      selectedNodeId: nodeId,
      saveState: 'dirty',
    })),
  updateNode: (nodeId, patch) =>
    set((state) => {
      const current = state.draft.nodes[nodeId]
      if (!current) return state
      return {
        draft: {
          ...state.draft,
          nodes: { ...state.draft.nodes, [nodeId]: { ...current, ...patch } },
        },
        saveState: 'dirty',
      }
    }),
  removeNode: (nodeId) =>
    set((state) => {
      const nodes = { ...state.draft.nodes }
      const positions = { ...state.canvas.positions }
      delete nodes[nodeId]
      delete positions[nodeId]
      return {
        draft: { ...state.draft, nodes },
        canvas: { ...state.canvas, positions },
        selectedNodeId:
          state.selectedNodeId === nodeId ? null : state.selectedNodeId,
        saveState: 'dirty',
      }
    }),
  moveNode: (nodeId, position) =>
    set((state) => ({
      canvas: {
        ...state.canvas,
        positions: { ...state.canvas.positions, [nodeId]: position },
      },
      saveState: 'dirty',
    })),
  selectNode: (selectedNodeId) => set({ selectedNodeId }),
  markSaving: () => set({ saveState: 'saving' }),
  markSaved: (revision) =>
    set((state) => ({
      revision,
      // Edits made while the request was in flight still need a save.
      saveState: state.saveState === 'saving' ? 'saved' : state.saveState,
    })),
  markFailed: (conflict) => set({ saveState: conflict ? 'conflict' : 'error' }),
}))
