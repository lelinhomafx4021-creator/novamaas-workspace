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
  ResourceMonitorPeriod,
  ResourceMonitorReport,
  SystemInstanceDeleteResponse,
  SystemInstanceListResponse,
} from './types'

export async function getResourceMonitor(
  minutes: ResourceMonitorPeriod,
  signal?: AbortSignal
): Promise<ResourceMonitorReport> {
  const res = await api.get<{
    success: boolean
    message?: string
    data?: ResourceMonitorReport
  }>('/api/system-info/monitor', {
    params: { minutes },
    signal,
    // React Query owns observer cancellation and request deduplication here.
    disableDuplicate: true,
    skipErrorHandler: true,
    skipBusinessError: true,
  })
  if (!res.data.success || !res.data.data) {
    throw new Error(res.data.message || 'Unable to load resource monitoring')
  }
  return res.data.data
}

export async function listSystemInstances() {
  const res = await api.get<SystemInstanceListResponse>(
    '/api/system-info/instances'
  )
  return res.data
}

export async function deleteStaleSystemInstances() {
  const res = await api.delete<SystemInstanceDeleteResponse>(
    '/api/system-info/stale-instances'
  )
  return res.data
}

export async function deleteStaleSystemInstance(nodeName: string) {
  const res = await api.delete<SystemInstanceDeleteResponse>(
    `/api/system-info/instances/${encodeURIComponent(nodeName)}`
  )
  return res.data
}
