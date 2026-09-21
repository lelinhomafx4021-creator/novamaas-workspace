import { apiRequest } from './request'

export function getUserAgreement() {
  return apiRequest<string>('/api/user-agreement', { auth: false })
}

export function getPrivacyPolicy() {
  return apiRequest<string>('/api/privacy-policy', { auth: false })
}
