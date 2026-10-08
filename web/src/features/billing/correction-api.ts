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

export interface CorrectionRow {
  source_entry_id: number
  model_name: string
  posted_at: number
  original_group: string
  original_rate: string
  original_quota: number
  corrected_quota: number
  delta: number
  blocked: string
}
export interface CorrectionBatch {
  id: string
  user_id: number
  created_by: number
  created_at: number
  expires_at: number
  start_at: number
  end_at: number
  models: string
  target_group: string
  target_rate: string
  reason: string
  status: 'preview' | 'applied' | 'reversed'
  can_apply: boolean
  charge_delta: number
  refund_delta: number
  net_delta: number
  sha256: string
  audit_logged_at: number
  reversal_audit_logged_at: number
  rows: CorrectionRow[]
}
export interface CorrectionInput {
  user_id: number
  start_at: number
  end_at: number
  models: string[]
  target_group: string
  reason: string
}
export interface CorrectionAction {
  action: 'apply' | 'reverse'
  sha256: string
  confirm_user_id: number
  confirm_net_delta: number
  reason: string
}

async function data<T>(
  request: Promise<{ data: { success: boolean; data: T; message?: string } }>
) {
  const response = (await request).data
  if (!response.success) throw new Error(response.message)
  return response.data
}
const path = '/api/billing/admin/corrections'
export function correctionGroups(userId: number) {
  return data(
    api.get<{ success: boolean; data: { group: string; rate: string }[] }>(
      `${path}/groups`,
      { params: { user_id: userId } }
    )
  )
}
export function listCorrections(userId: number) {
  return data(
    api.get<{ success: boolean; data: CorrectionBatch[] }>(path, {
      params: { user_id: userId },
    })
  )
}
export function getCorrection(id: string) {
  return data(
    api.get<{ success: boolean; data: CorrectionBatch }>(`${path}/${id}`)
  )
}
export function previewCorrection(input: CorrectionInput) {
  return data(
    api.post<{ success: boolean; data: CorrectionBatch }>(
      `${path}/preview`,
      input
    )
  )
}
export function actOnCorrection(
  id: string,
  input: CorrectionAction,
  proof?: string
) {
  return data(
    api.post<{ success: boolean; data: CorrectionBatch }>(
      `${path}/${id}/actions`,
      input,
      { headers: { 'X-Security-Proof': proof } }
    )
  )
}
