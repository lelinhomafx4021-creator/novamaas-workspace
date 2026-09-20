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
type CostAccountingOperationInput = {
  start_timestamp: number
  end_timestamp: number
  channel_id: number
  cost_discount: string
  offset: number
  limit: number
  apply: boolean
  batch_id: string
}

export type CostAccountingBackfillInput = CostAccountingOperationInput

export type CostAccountingBackfillResult = {
  batch_id: string
  scanned: number
  ready: number
  existing: number
  unresolved: number
  applied: number
  next_offset: number
  has_more: boolean
  cost_quota: number
  revenue_quota: number
}

export type CostAccountingRepriceInput = CostAccountingOperationInput & {
  reason: string
}

export type CostAccountingRepriceResult = {
  batch_id: string
  scanned: number
  changed: number
  unchanged: number
  applied: number
  next_offset: number
  has_more: boolean
  current_cost_quota: number
  new_cost_quota: number
  delta_cost_quota: number
}

export type CostAccountingAPIResponse<T> = {
  success: boolean
  message?: string
  data: T
}
