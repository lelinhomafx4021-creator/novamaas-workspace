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
import type {
  Capability,
  GenerationDraft,
  NodeReference,
  ProjectCanvas,
  ProjectDraft,
  Run,
  SubmitParameters,
  SubmitRequest,
} from '../types'

export const EMPTY_DRAFT: ProjectDraft = { version: 1, nodes: {} }
export const EMPTY_CANVAS: ProjectCanvas = { version: 1, positions: {} }

function parseObject<T extends { version: 1 }>(raw: string, empty: T): T {
  try {
    const parsed = JSON.parse(raw) as Partial<T>
    if (parsed && parsed.version === 1) return { ...empty, ...parsed }
  } catch {
    // A corrupt or legacy payload falls back to an empty project.
  }
  return empty
}

export function parseDraft(raw: string): ProjectDraft {
  return parseObject(raw, EMPTY_DRAFT)
}

export function parseCanvas(raw: string): ProjectCanvas {
  return parseObject(raw, EMPTY_CANVAS)
}

export function newGenerationDraft(capability?: Capability): GenerationDraft {
  return {
    model: capability?.model ?? '',
    token_id: 0,
    prompt: '',
    references: [],
    parameters: capability ? { ...capability.defaults } : {},
  }
}

/**
 * Lists every reason the draft cannot be submitted to the selected model.
 * Switching models keeps the inputs; unsupported ones are reported here
 * instead of being dropped silently.
 */
export function draftIssues(
  draft: GenerationDraft,
  capability: Capability | undefined
): string[] {
  if (!capability) return ['Select a model']
  const issues: string[] = []
  if (draft.prompt.trim() === '') issues.push('Enter a prompt')
  if (draft.references.length > capability.max_references) {
    issues.push('Too many reference images for this model')
  }
  const roles = draft.references.map((reference) => reference.role)
  if (roles.some((role) => !capability.roles.includes(role))) {
    issues.push('This model does not support some reference roles')
  }
  if (
    !capability.asset_library &&
    draft.references.some((reference) => reference.source === 'seedance_asset')
  ) {
    issues.push('Seedance assets only work with Seedance models')
  }
  if (roles.includes('last_frame') && !roles.includes('first_frame')) {
    issues.push('A last frame requires a first frame')
  }
  if (
    roles.includes('reference_image') &&
    (roles.includes('first_frame') || roles.includes('last_frame'))
  ) {
    issues.push('Reference images cannot be combined with first or last frames')
  }
  if (draft.parameters.generate_audio !== undefined && !capability.audio) {
    issues.push('This model does not support audio generation')
  }
  if (draft.parameters.seed !== undefined && !capability.seed) {
    issues.push('This model does not support a fixed seed')
  }
  const duration = draft.parameters.duration
  if (
    duration !== undefined &&
    (duration < capability.min_duration || duration > capability.max_duration)
  ) {
    issues.push('Duration is out of range for this model')
  }
  return issues
}

export function buildSubmitRequest(
  projectId: string,
  nodeId: string,
  draft: GenerationDraft,
  clientRequestId: string
): SubmitRequest {
  return {
    project_id: projectId,
    node_id: nodeId,
    client_request_id: clientRequestId,
    model: draft.model,
    token_id: draft.token_id || undefined,
    prompt: draft.prompt.trim(),
    references: draft.references.map(({ source, id, role }) => ({
      source,
      id,
      role,
    })),
    parameters: draft.parameters,
  }
}

/** Restores a run's input snapshot into an editable draft ("generate again"). */
export function draftFromRun(
  run: Run,
  fallback: GenerationDraft
): GenerationDraft {
  try {
    const snapshot = JSON.parse(run.snapshot) as Partial<SubmitRequest>
    const labels = new Map(
      fallback.references.map((reference) => [reference.id, reference.label])
    )
    const references: NodeReference[] = (snapshot.references ?? []).map(
      (reference) => ({
        ...reference,
        label: labels.get(reference.id) ?? reference.id,
      })
    )
    return {
      model: snapshot.model ?? fallback.model,
      token_id: snapshot.token_id ?? fallback.token_id,
      prompt: snapshot.prompt ?? fallback.prompt,
      references,
      parameters: snapshot.parameters ?? fallback.parameters,
    }
  } catch {
    return fallback
  }
}

export function isRunActive(run: Run): boolean {
  return (
    run.status === 'submitting' ||
    run.status === 'queued' ||
    run.status === 'in_progress'
  )
}

/**
 * Parameters to carry over when switching to another model: the model's
 * defaults, overridden by the current values the new model also supports.
 */
export function parametersForModel(
  parameters: SubmitParameters,
  capability: Capability
): SubmitParameters {
  const kept: SubmitParameters = {}
  if (capability.audio && parameters.generate_audio !== undefined) {
    kept.generate_audio = parameters.generate_audio
  }
  if (capability.seed && parameters.seed !== undefined) {
    kept.seed = parameters.seed
  }
  if (
    parameters.resolution &&
    capability.resolutions.includes(parameters.resolution)
  ) {
    kept.resolution = parameters.resolution
  }
  if (parameters.ratio && capability.ratios.includes(parameters.ratio)) {
    kept.ratio = parameters.ratio
  }
  if (
    parameters.duration !== undefined &&
    parameters.duration >= capability.min_duration &&
    parameters.duration <= capability.max_duration
  ) {
    kept.duration = parameters.duration
  }
  return { ...capability.defaults, ...kept }
}
