import { apiRequest } from './request'
import { getApiTokens, type ApiToken } from './tokens'

export interface StudioSettings {
  enabled: boolean
  max_images: number
  max_prompt_characters: number
}

export interface StudioResult {
  id: string
  preview_url?: string
  thumbnail_url?: string
  download_url?: string
  description?: string
}

export interface StudioRun {
  id: string
  status: 'running' | 'completed' | 'failed'
  prompt?: string
  model: string
  error_message?: string
  results: StudioResult[]
}

export interface StudioRunPage {
  items: StudioRun[]
  total: number
}

export function getStudioSettings() {
  return apiRequest<StudioSettings>('/api/image-studio/settings')
}

export async function getStudioKeys() {
  const keys: ApiToken[] = []
  for (let page = 1; ; page++) {
    const response = await getApiTokens(page, 100)
    keys.push(...response.items)
    if (!response.items.length || keys.length >= response.total) break
  }
  return keys.filter(key => key.status === 1)
}

export function getStudioModels(tokenId: number, kind: 'image' | 'text') {
  return apiRequest<string[]>(`/api/image-studio/models?token_id=${tokenId}&kind=${kind}`)
}

export function listStudioRuns(page = 1) {
  return apiRequest<StudioRunPage>(`/api/image-studio/runs?p=${page}&page_size=8`)
}

export function getStudioRun(id: string) {
  return apiRequest<StudioRun>(`/api/image-studio/runs/${encodeURIComponent(id)}`)
}

export function createStudioRun(input: {
  token_id: number
  idempotency_key: string
  request: { model: string; prompt: string }
}) {
  return apiRequest<StudioRun>('/api/image-studio/runs', {
    method: 'POST',
    timeout: 300_000,
    header: { 'Idempotency-Key': input.idempotency_key },
    data: input,
  })
}

export function assistStudioPrompt(tokenId: number, model: string, prompt: string) {
  return apiRequest<{ suggested_prompt: string }>('/api/image-studio/prompt-assist', {
    method: 'POST',
    timeout: 150_000,
    data: { token_id: tokenId, model, prompt },
  })
}
