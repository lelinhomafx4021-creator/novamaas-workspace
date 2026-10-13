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
import type { Edge, Node } from '@xyflow/react'

import type { ProjectCanvas, ProjectDraft, Run } from '../types'

const RESULT_OFFSET_X = 360
const RESULT_SPACING_Y = 320

/**
 * Derives the canvas graph: each generation node is followed by one result
 * node per run, newest first. Result positions are not persisted; they are
 * laid out next to their generation node.
 */
export function buildFlowGraph(
  draft: ProjectDraft,
  canvas: ProjectCanvas,
  runs: Run[],
  selectedNodeId: string | null,
  onRegenerate: (run: Run) => void
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = []
  const edges: Edge[] = []
  const runsByNode = new Map<string, Run[]>()
  for (const run of runs) {
    const list = runsByNode.get(run.node_id) ?? []
    list.push(run)
    runsByNode.set(run.node_id, list)
  }
  for (const nodeId of Object.keys(draft.nodes)) {
    const position = canvas.positions[nodeId] ?? { x: 0, y: 0 }
    nodes.push({
      id: nodeId,
      type: 'generation',
      position,
      data: {},
      selected: nodeId === selectedNodeId,
    })
    const nodeRuns = runsByNode.get(nodeId) ?? []
    nodeRuns.forEach((run, index) => {
      const resultId = `result-${run.id}`
      nodes.push({
        id: resultId,
        type: 'result',
        position: canvas.positions[resultId] ?? {
          x: position.x + RESULT_OFFSET_X,
          y: position.y + index * RESULT_SPACING_Y,
        },
        data: { run, onRegenerate },
        selectable: false,
      })
      edges.push({ id: `edge-${run.id}`, source: nodeId, target: resultId })
    })
  }
  return { nodes, edges }
}
