import { apiRequest } from './request'

export interface ModelPricing {
  audio_completion_ratio?: number
  audio_ratio?: number
  billing_expr?: string
  billing_mode?: string
  cache_ratio?: number
  capabilities?: string[]
  completion_ratio?: number
  context_length?: number
  description?: string
  enable_groups: string[]
  icon?: string
  id: number
  input_modalities?: string[]
  max_output_tokens?: number
  model_name: string
  model_price?: number
  model_ratio?: number
  output_modalities?: string[]
  quota_type?: number
  supported_endpoint_types?: string[]
  tags?: string[]
  vendor_id?: number
  vendor_name?: string
}

export interface UserGroup {
  desc: string
  ratio: number | string
}

export function getPricingCatalog() {
  return apiRequest<ModelPricing[]>('/api/pricing')
}

export function getUserGroups() {
  return apiRequest<Record<string, UserGroup>>('/api/user/self/groups')
}

export function getUserModels(group?: string) {
  const query = group ? `?group=${encodeURIComponent(group)}` : ''
  return apiRequest<string[]>(`/api/user/models${query}`)
}
