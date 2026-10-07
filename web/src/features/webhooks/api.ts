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
  WebhookEndpoint,
  WebhookEndpointInput,
  WebhookResponse,
  WebhookProbeResult,
  SystemWebhookSettings,
  SystemWebhookTopic,
  WebhookCapabilities,
} from './types'

const options = { skipBusinessError: true, skipErrorHandler: true }

export async function listWebhookEndpoints() {
  const response = await api.get<WebhookResponse<WebhookEndpoint[]>>(
    '/api/webhook-endpoints',
    options
  )
  return response.data
}

export async function saveWebhookEndpoint(
  input: WebhookEndpointInput,
  id?: string
) {
  if (id) {
    const response = await api.put<WebhookResponse<WebhookEndpoint>>(
      `/api/webhook-endpoints/${encodeURIComponent(id)}`,
      input,
      options
    )
    return response.data
  }
  const response = await api.post<WebhookResponse<WebhookEndpoint>>(
    '/api/webhook-endpoints',
    input,
    options
  )
  return response.data
}

export async function deleteWebhookEndpoint(id: string) {
  const response = await api.delete<WebhookResponse>(
    `/api/webhook-endpoints/${encodeURIComponent(id)}`,
    options
  )
  return response.data
}

export async function testWebhookEndpoint(id: string) {
  const response = await api.post<WebhookResponse<WebhookProbeResult>>(
    `/api/webhook-endpoints/${encodeURIComponent(id)}/test`,
    undefined,
    options
  )
  return response.data
}

export async function getSystemWebhookSettings() {
  const response = await api.get<WebhookResponse<SystemWebhookSettings>>(
    '/api/webhooks/system',
    options
  )
  return response.data
}

export async function saveSystemWebhookSettings(input: SystemWebhookSettings) {
  const response = await api.put<WebhookResponse<SystemWebhookSettings>>(
    '/api/webhooks/system',
    input,
    options
  )
  return response.data
}

export async function testSystemWebhook(
  topic: SystemWebhookTopic,
  url: string
) {
  const response = await api.post<WebhookResponse<WebhookProbeResult>>(
    `/api/webhooks/system/${topic}/test`,
    { url },
    options
  )
  return response.data
}

export async function getWebhookCapabilities() {
  const response = await api.get<WebhookResponse<WebhookCapabilities>>(
    '/api/webhooks/capabilities',
    options
  )
  return response.data
}

export async function downloadWebhookManual(scope: SystemWebhookTopic) {
  const response = await api.get<Blob>('/api/webhooks/manual.pdf', {
    responseType: 'blob',
    params: { scope },
  })
  return response.data
}
