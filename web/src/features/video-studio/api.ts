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
import { getApiKeys } from '@/features/keys/api'
import { sendChatCompletion } from '@/features/playground/api'
import { api } from '@/lib/api'

import type {
  Capability,
  Page,
  Project,
  ProjectSummary,
  Run,
  StudioReference,
  SubmitRequest,
  VideoStudioResponse,
} from './types'

const quiet = { skipBusinessError: true, skipErrorHandler: true }

export async function listCapabilities(tokenId?: number) {
  const response = await api.get<VideoStudioResponse<Capability[]>>(
    '/api/video-studio/models',
    { params: { token_id: tokenId || undefined } }
  )
  return response.data
}

/** Non-video models the key can reach, for prompt optimization. */
export async function listTextModels(tokenId: number) {
  const response = await api.get<VideoStudioResponse<string[]>>(
    '/api/video-studio/models',
    { params: { token_id: tokenId, kind: 'text' } }
  )
  return response.data
}

export async function listProjects(page: number, pageSize: number) {
  const response = await api.get<VideoStudioResponse<Page<ProjectSummary>>>(
    '/api/video-studio/projects',
    { params: { p: page, page_size: pageSize } }
  )
  return response.data
}

export async function createProject(name: string) {
  const response = await api.post<VideoStudioResponse<Project>>(
    '/api/video-studio/projects',
    { name },
    quiet
  )
  return response.data
}

export async function getProject(id: string) {
  const response = await api.get<VideoStudioResponse<Project>>(
    `/api/video-studio/projects/${encodeURIComponent(id)}`,
    { disableDuplicate: true }
  )
  return response.data
}

export async function saveProject(
  id: string,
  input: { revision: number; name?: string; draft?: string; canvas?: string }
) {
  const response = await api.put<VideoStudioResponse<Project>>(
    `/api/video-studio/projects/${encodeURIComponent(id)}`,
    input,
    quiet
  )
  return response.data
}

export async function deleteProject(id: string) {
  const response = await api.delete<VideoStudioResponse>(
    `/api/video-studio/projects/${encodeURIComponent(id)}`,
    quiet
  )
  return response.data
}

export async function listRuns(projectId: string) {
  const response = await api.get<VideoStudioResponse<Page<Run>>>(
    `/api/video-studio/projects/${encodeURIComponent(projectId)}/runs`,
    { params: { p: 1, page_size: 100 }, disableDuplicate: true }
  )
  return response.data
}

export async function uploadReference(file: File) {
  const form = new FormData()
  form.append('file', file)
  const response = await api.post<VideoStudioResponse<StudioReference>>(
    '/api/video-studio/references',
    form,
    quiet
  )
  return response.data
}

export async function listReferences(
  page: number,
  pageSize: number,
  includeTemporary = false
) {
  const response = await api.get<VideoStudioResponse<Page<StudioReference>>>(
    '/api/video-studio/references',
    {
      params: {
        p: page,
        page_size: pageSize,
        include_temporary: includeTemporary || undefined,
      },
    }
  )
  return response.data
}

export async function getReferencePreview(id: string) {
  const response = await api.get<
    VideoStudioResponse<{ url: string; expires_at: number }>
  >(`/api/video-studio/references/${encodeURIComponent(id)}/preview`, {
    params: { variant: 'thumbnail' },
  })
  return response.data
}

export async function saveReference(id: string) {
  const response = await api.put<VideoStudioResponse<StudioReference>>(
    `/api/video-studio/references/${encodeURIComponent(id)}/retention`,
    {},
    quiet
  )
  return response.data
}

export async function submitVideo(input: SubmitRequest) {
  const response = await api.post<VideoStudioResponse<Run>>(
    '/pg/videos',
    input,
    quiet
  )
  return response.data
}

const PROMPT_INSTRUCTION =
  'You rewrite prompts for an AI video generation model. Keep the original intent and language. Add concrete subject, scene, camera movement, lighting and style details. Reply with the rewritten prompt only, under 800 characters, without quotes or explanations.'

/** Rewrites a prompt through the administrator-chosen text model. */
export async function optimizePrompt(
  model: string,
  group: string | undefined,
  tokenId: number,
  prompt: string,
  signal?: AbortSignal
) {
  const response = await sendChatCompletion(
    {
      model,
      token_id: tokenId,
      stream: false,
      temperature: 0.7,
      group,
      messages: [
        { role: 'system', content: PROMPT_INSTRUCTION },
        { role: 'user', content: prompt },
      ],
    },
    signal
  )
  const text = response.choices[0]?.message.content.trim()
  if (!text) throw new Error('The model returned an empty prompt')
  return text
}

/** Enabled keys the user can run studio generations under. */
export async function listStudioKeys() {
  const response = await getApiKeys({ p: 1, size: 100 })
  const now = Date.now() / 1000
  return (response.data?.items ?? []).filter(
    (key) =>
      key.status === 1 && (key.expired_time === -1 || key.expired_time > now)
  )
}
