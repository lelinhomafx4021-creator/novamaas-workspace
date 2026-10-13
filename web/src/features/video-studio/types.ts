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
export type VideoStudioResponse<T = undefined> = {
  success: boolean
  message?: string
  data: T
}

export type Page<T> = {
  items: T[]
  total: number
  page: number
  page_size: number
}

export type ReferenceRole = 'first_frame' | 'last_frame' | 'reference_image'
export type ReferenceSource = 'storage' | 'seedance_asset'

export type Capability = {
  model: string
  protocol: 'seedance' | 'wan3' | 'openai_video'
  roles: ReferenceRole[]
  max_references: number
  min_duration: number
  max_duration: number
  resolutions: string[]
  ratios: string[]
  audio: boolean
  seed: boolean
  asset_library: boolean
  defaults: { duration: number; resolution: string; ratio: string }
}

export type StudioReference = {
  id: string
  file_name: string
  content_type: string
  size: number
  sha256: string
  saved: boolean
  expires_at: number
  created_at: number
}

export type ProjectSummary = {
  id: string
  name: string
  revision: number
  created_at: number
  updated_at: number
}

export type Project = ProjectSummary & {
  draft: string
  canvas: string
}

export type SubmitReference = {
  source: ReferenceSource
  id: string
  role: ReferenceRole
}

export type SubmitParameters = {
  duration?: number
  resolution?: string
  ratio?: string
  generate_audio?: boolean
  seed?: number
}

export type SubmitRequest = {
  project_id: string
  node_id: string
  client_request_id: string
  model: string
  token_id?: number
  prompt: string
  references: SubmitReference[]
  parameters: SubmitParameters
}

export type RunStatus =
  | 'submitting'
  | 'queued'
  | 'in_progress'
  | 'completed'
  | 'failed'
  | 'unknown'

export type Run = {
  id: string
  node_id: string
  client_request_id: string
  snapshot: string
  task_id: string
  created_at: number
  status: RunStatus
  progress?: string
  fail_reason?: string
  quota: number
  finished_at?: number
}

/** A reference attached to a generation node, in submission order. */
export type NodeReference = SubmitReference & {
  label: string
}

/** Persisted generation node fields; layout lives in the canvas JSON. */
export type GenerationDraft = {
  model: string
  /** API key the run is billed to; 0 uses the account's default group. */
  token_id: number
  /** Key and model used to optimize the prompt; 0 or empty turns it off. */
  text_token_id: number
  text_model: string
  prompt: string
  references: NodeReference[]
  parameters: SubmitParameters
}

export type ProjectDraft = {
  version: 1
  nodes: Record<string, GenerationDraft>
}

export type ProjectCanvas = {
  version: 1
  positions: Record<string, { x: number; y: number }>
  viewport?: { x: number; y: number; zoom: number }
}
