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
export type WebhookEventType =
  | 'asset.active'
  | 'asset.failed'
  | 'task.status_changed'

export type WebhookEndpointInput = {
  name: string
  url: string
  event_types: WebhookEventType[]
}

export type WebhookEndpoint = WebhookEndpointInput & {
  id: string
  object: 'webhook_endpoint'
  status: string
  created_at: number
  updated_at: number
}

export type WebhookResponse<T = unknown> = {
  success: boolean
  message?: string
  data: T
}

export type WebhookProbeResult = {
  reachable: boolean
  http_status: number
  duration_ms: number
  event_id: string
  webhook_id: string
  error?: string
}

export type SystemWebhookTopic = 'asset_library' | 'media_tasks'
export type SystemWebhookTarget = { enabled: boolean; url: string }
export type SystemWebhookSettings = Record<
  SystemWebhookTopic,
  SystemWebhookTarget
> & { manual_enabled: boolean }

export type WebhookCapabilities = {
  asset_library_enabled: boolean
  media_tasks_enabled: boolean
  manual_enabled: boolean
}
