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
import type { TFunction } from 'i18next'

import { api } from '@/lib/api'
import type { AuthBundle } from '@/stores/auth-store'

export interface PhoneStatus {
  phone: string
  verified: { phone: string; source: string; verified_at: string } | null
  sms_enabled: boolean
}

export interface SmsChallenge {
  challenge_token: string
  expires_in: number
  retry_after: number
}

interface Response<T> {
  success: boolean
  message?: string
  data: T
}

export async function getPhoneStatus(userId?: number): Promise<PhoneStatus> {
  const response = await api.get<Response<PhoneStatus>>(
    userId ? `/api/user/${userId}/phone` : '/api/user/self/phone'
  )
  return response.data.data
}

export async function verifyAccountSecurity(input: {
  password?: string
  two_factor_code?: string
  challenge_token?: string
  code?: string
  scope: 'phone.manage' | 'wechat.manage'
}): Promise<string> {
  const response = await api.post<Response<{ proof_token: string }>>(
    '/api/user/self/phone/security/verify',
    input
  )
  return response.data.data.proof_token
}

export async function sendSecuritySMS(): Promise<SmsChallenge> {
  const response = await api.post<Response<SmsChallenge>>(
    '/api/user/self/phone/security/code'
  )
  return response.data.data
}

export async function sendBindingSMS(
  phone: string,
  proof: string,
  userId?: number
): Promise<SmsChallenge> {
  const response = await api.post<Response<SmsChallenge>>(
    userId ? `/api/user/${userId}/phone/code` : '/api/user/self/phone/code',
    { phone },
    { headers: { 'X-Security-Proof': proof } }
  )
  return response.data.data
}

export async function confirmPhoneBinding(
  token: string,
  code: string,
  proof: string,
  userId?: number
): Promise<void> {
  await api.post(
    userId ? `/api/user/${userId}/phone/bind` : '/api/user/self/phone/bind',
    { challenge_token: token, code },
    { headers: { 'X-Security-Proof': proof }, acceptAuthRotation: !userId }
  )
}

export async function sendLoginSMS(input: {
  phone?: string
  flow_token?: string
  turnstile?: string
}): Promise<SmsChallenge> {
  const path = input.flow_token
    ? '/api/user/login/sms/mfa/code'
    : '/api/user/login/sms/code'
  const response = await api.post<Response<SmsChallenge>>(
    `${path}?turnstile=${encodeURIComponent(input.turnstile ?? '')}`,
    input,
    { skipAuthRefresh: true }
  )
  return response.data.data
}

export async function loginWithSMS(
  token: string,
  code: string,
  flowToken?: string
): Promise<AuthBundle> {
  const response = await api.post<Response<AuthBundle>>(
    flowToken ? '/api/user/login/sms/mfa' : '/api/user/login/sms',
    { challenge_token: token, code, flow_token: flowToken },
    { skipAuthRefresh: true }
  )
  return response.data.data
}

export function phoneErrorMessage(error: unknown, t: TFunction): string {
  const code = (error as { response?: { data?: { code?: string } } })?.response
    ?.data?.code
  if (code === 'PHONE_RATE_LIMITED') {
    return t('Too many SMS requests. Please try again later.')
  }
  if (code === 'PHONE_SMS_UNAVAILABLE') {
    return t('SMS service is unavailable. Use another sign-in method.')
  }
  if (code === 'PHONE_NUMBER_INVALID') {
    return t('Enter a valid mainland China phone number.')
  }
  if (code === 'PHONE_BINDING_PROTECTED') {
    return t('Use phone verification to change a bound phone number')
  }
  if (code === 'PHONE_ALREADY_BOUND') {
    return t('This phone number is bound to another account.')
  }
  if (code?.startsWith('SECURITY_PROOF_')) {
    return t('Security verification expired. Please verify again.')
  }
  return t('Phone verification failed. Check the code or request a new one.')
}

export async function sendUserMutationSMS(
  input: { phone: string; user_id?: number; username: string },
  proof: string
): Promise<SmsChallenge> {
  const response = await api.post<Response<SmsChallenge>>(
    '/api/user/phone/code',
    input,
    { headers: { 'X-Security-Proof': proof } }
  )
  return response.data.data
}
