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
export type AssetGroup = {
  id: string
  owner_user_id: number
  owner_name?: string
  name: string
  description: string
  status: string
  created_at: number
  updated_at: number
}

export type MediaAsset = {
  id: string
  group_id: string
  owner_user_id: number
  owner_name: string
  name: string
  type: 'image' | 'video' | 'audio'
  content_type: string
  size: number
  sha256: string
  status: string
  created_at: number
  updated_at: number
}

export type MediaAssetList = {
  items: MediaAsset[]
  total: number
  page: number
  page_size: number
}

export type AssetAccessKey = {
  id: number
  name: string
  access_key_id: string
  secret_hint: string
  status: string
  last_used_at: number
  created_at: number
}

export type CreatedAssetAccessKey = AssetAccessKey & {
  secret_access_key: string
}

export type AssetSyncJob = {
  id: number
  asset_id: string
  asset_name: string
  asset_type: string
  group_id: string
  group_name: string
  owner_user_id: number
  owner_name: string
  channel_id: number
  channel_name: string
  operation: string
  status: string
  progress: number
  attempts: number
  last_error?: string
  last_synced_at: number
  updated_at: number
}

export type AssetSyncSummary = {
  total: number
  pending: number
  processing: number
  active: number
  failed: number
}

export type AssetSyncJobList = {
  items: AssetSyncJob[]
  total: number
  page: number
  page_size: number
  summary: AssetSyncSummary
}

export type AssetLibraryResponse<T = undefined> = {
  success: boolean
  message?: string
  data: T
}
