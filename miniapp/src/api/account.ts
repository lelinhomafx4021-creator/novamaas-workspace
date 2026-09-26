import { apiRequest } from './request'

export interface AccountProfile {
  aff_code: string
  aff_count: number
  aff_history_quota: number
  aff_quota: number
  display_name: string
  email: string
  group: string
  id: number
  quota: number
  role: number
  setting: string
  username: string
}

export interface LoginSession {
  created_at: number
  current: boolean
  expires_at: number
  ip: string
  last_active_at: number
  login_method: string
  sid: string
  user_agent: string
}

export interface CheckinStats {
  checked_in_today: boolean
  checkin_count: number
  records: Array<{ checkin_date: string; quota_awarded: number }>
  total_checkins: number
  total_quota: number
}

export interface CheckinStatus {
  enabled: boolean
  max_quota: number
  min_quota: number
  stats: CheckinStats
}

export function getAccountProfile() {
  return apiRequest<AccountProfile>('/api/user/self')
}

export function updateAccountProfile(displayName: string) {
  return apiRequest<null>('/api/user/self', {
    data: { display_name: displayName },
    method: 'PUT',
  })
}

export function updateAccountLanguage(language: string) {
  return apiRequest<null>('/api/user/self', {
    data: { language },
    method: 'PUT',
  })
}

export function getLoginSessions() {
  return apiRequest<LoginSession[]>('/api/user/sessions')
}

export function revokeLoginSession(sid: string) {
  return apiRequest<{ current: boolean; revoked_sid: string }>(
    `/api/user/sessions/${encodeURIComponent(sid)}`,
    { method: 'DELETE' }
  )
}

export function revokeOtherLoginSessions() {
  return apiRequest<{ revoked_count: number }>('/api/user/sessions/revoke-others', {
    method: 'POST',
  })
}

export function getCheckinStatus(month: string) {
  return apiRequest<CheckinStatus>(`/api/user/checkin?month=${encodeURIComponent(month)}`)
}

export function checkin() {
  return apiRequest<{ checkin_date: string; quota_awarded: number }>(
    '/api/user/checkin',
    { method: 'POST' }
  )
}

export function getAffiliateCode() {
  return apiRequest<string>('/api/user/aff')
}

export function transferAffiliateQuota(quota: number) {
  return apiRequest<null>('/api/user/aff_transfer', {
    data: { quota },
    method: 'POST',
  })
}

export function deleteAccount() {
  return apiRequest<null>('/api/user/self', { method: 'DELETE' })
}
