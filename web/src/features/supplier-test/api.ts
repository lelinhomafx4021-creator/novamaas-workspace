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
  base_url?: string
  exchange?: import('./types').VideoExchange
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

export async function verifySupplierVideo(
  input: {
    action: 'create' | 'query'
    video_url?: string
    query_id?: string
    access_key: string
    secret_key: string
  },
  signal: AbortSignal
): Promise<import('./types').VideoExchange> {
  const response = await api.post('/api/supplier-test/video/verify', input, {
    signal,
  })
  if (!response.data.success) {
    throw new Error('Official verification request failed')
  }
  return response.data.data
}
