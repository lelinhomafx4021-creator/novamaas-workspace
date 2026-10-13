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

export async function listCapabilities(group?: string) {
  const response = await api.get<VideoStudioResponse<Capability[]>>(
    '/api/video-studio/models',
    { params: { group: group || undefined } }
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

export async function listReferences(page: number, pageSize: number) {
  const response = await api.get<VideoStudioResponse<Page<StudioReference>>>(
    '/api/video-studio/references',
    { params: { p: page, page_size: pageSize } }
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
