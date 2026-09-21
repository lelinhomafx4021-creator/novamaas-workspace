import { apiRequest } from './request'

export interface ApiToken {
  accessed_time: number
  allow_ips: string
  auto_groups: string[]
  created_time: number
  cross_group_retry: boolean
  expired_time: number
  group: string
  id: number
  key: string
  model_limits: string
  model_limits_enabled: boolean
  name: string
  remain_quota: number
  status: number
  unlimited_quota: boolean
  used_quota: number
}

interface TokenPage {
  items: ApiToken[]
  page: number
  page_size: number
  total: number
}

export function getApiTokens(page = 1, size = 20) {
  return apiRequest<TokenPage>(`/api/token/?p=${page}&size=${size}`)
}

export function createApiToken(name: string, group: string) {
  return apiRequest<unknown>('/api/token/', {
    method: 'POST',
    data: {
      allow_ips: '',
      auto_groups: [],
      cross_group_retry: false,
      expired_time: -1,
      group,
      model_limits: '',
      model_limits_enabled: false,
      name,
      remain_quota: 0,
      unlimited_quota: true,
    },
  })
}

export function setApiTokenEnabled(token: ApiToken, enabled: boolean) {
  return apiRequest<ApiToken>('/api/token/?status_only=true', {
    method: 'PUT',
    data: { ...token, status: enabled ? 1 : 2 },
  })
}

export function deleteApiToken(id: number) {
  return apiRequest<unknown>(`/api/token/${id}`, { method: 'DELETE' })
}

export function verifyApiTokenAccess(password: string, twoFA: string) {
  return apiRequest<{ expires_at: number; proof_token: string }>(
    '/api/mini/security/verify',
    { method: 'POST', data: { password, two_fa: twoFA } }
  )
}

export function revealApiToken(id: number, proofToken: string) {
  return apiRequest<{ key: string }>(`/api/mini/token/${id}/key`, {
    method: 'POST',
    header: { 'X-Security-Proof': proofToken },
  })
}
