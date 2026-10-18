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

import { API_ENDPOINTS } from './constants'

export async function fetchSupplierModels(payload: {
  base_url: string
  api_key: string
}): Promise<string[]> {
  const res = await api.post(API_ENDPOINTS.MODELS, payload, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  if (!res.data?.success) {
    throw new Error(String(res.data?.message || 'Failed to fetch models'))
  }
  return (res.data?.data ?? []) as string[]
}

export type QueryVideoTaskResult = {
  video_report?: import('./types').VideoRunReport
  success: boolean
  status_code?: number
  task_id?: string
  status?: string
  video_url?: string
  fail_reason?: string
  raw_response?: string
  message?: string
}

export async function querySupplierVideoTask(payload: {
  base_url: string
  api_key: string
  task_id: string
  custom_path?: string
}): Promise<QueryVideoTaskResult> {
  const res = await api.post(API_ENDPOINTS.VIDEO_QUERY, payload, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  return res.data as QueryVideoTaskResult
}

export async function getVideoRun(id: string) {
  const response = await api.get(
    `/api/supplier-test/video/runs/${encodeURIComponent(id)}`
  )
  return response.data.data as import('./types').VideoRunReport
}

export async function downloadVideoEvidence(id: string) {
  const response = await api.get(
    `/api/supplier-test/video/runs/${encodeURIComponent(id)}/evidence`,
    { responseType: 'blob' }
  )
  const url = URL.createObjectURL(response.data)
  const link = document.createElement('a')
  link.href = url
  link.download = `${id}-evidence.json`
  link.click()
  URL.revokeObjectURL(url)
}

export async function getVerificationSettings() {
  const response = await api.get(
    '/api/supplier-test/video/verification-settings'
  )
  return response.data.data as { configured: boolean; access_key_hint: string }
}

export async function saveVerificationSettings(
  accessKeyID: string,
  secret: string
) {
  const response = await api.put(
    '/api/supplier-test/video/verification-settings',
    { access_key_id: accessKeyID, secret_access_key: secret }
  )
  if (!response.data.success) {
    throw new Error(
      response.data.message || 'Unable to save verification credentials'
    )
  }
}

export async function queryVideoMaterials(
  baseURL: string,
  apiKey: string,
  search: string,
  page: number
) {
  const response = await api.post('/api/supplier-test/video/assets/query', {
    base_url: baseURL,
    api_key: apiKey,
    search,
    page,
  })
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to query materials')
  }
  return response.data.data as import('../assets/types').MediaAssetList
}

export async function previewVideoMaterial(
  baseURL: string,
  apiKey: string,
  assetId: string
) {
  const response = await api.post('/api/supplier-test/video/assets/query', {
    base_url: baseURL,
    api_key: apiKey,
    resource: 'preview',
    asset_id: assetId,
  })
  if (!response.data.success) throw new Error('Preview unavailable')
  return response.data.data as { url: string; expires_at: number }
}

export async function uploadVideoMaterial(form: FormData) {
  const response = await api.post(
    '/api/supplier-test/video/assets/upload',
    form
  )
  if (!response.data.success) {
    throw new Error(response.data.message || 'Unable to upload material')
  }
  return response.data.data as {
    asset: import('../assets/types').MediaAsset
    preparation_run_id: string
  }
}
