import Taro from '@tarojs/taro'

import { apiRequest } from '@/api/request'
import { clearConversation } from '@/playground/storage'

import {
  clearMiniAuthSession,
  getMiniAuthSession,
  saveMiniAuthSession,
  type MiniAuthSession,
  type MiniAuthUser,
} from './session'

interface MiniAuthBundleData {
  access_expires_at: number
  access_token: string
  binding_required: false
  refresh_token: string
  session: { sid: string }
  user: MiniAuthUser
}

export interface MiniBindingRequiredData {
  binding_required: true
  email_verification_enabled: boolean
  flow_expires_at: number
  flow_token: string
  password_login_enabled: boolean
  privacy_policy_enabled: boolean
  registration_enabled: boolean
  user_agreement_enabled: boolean
}

export type MiniLoginResult =
  | { kind: 'authenticated'; session: MiniAuthSession }
  | { kind: 'binding-required'; binding: MiniBindingRequiredData }

export interface MiniBindInput {
  acceptTerms: boolean
  flowToken: string
  password: string
  twoFactorCode?: string
  username: string
}

export interface MiniRegisterInput {
  acceptTerms: boolean
  affCode?: string
  email?: string
  flowToken: string
  password: string
  username: string
  verificationCode?: string
}

function persistAuthBundle(data: MiniAuthBundleData) {
  const session: MiniAuthSession = {
    accessToken: data.access_token,
    accessExpiresAt: data.access_expires_at,
    refreshToken: data.refresh_token,
    sid: data.session.sid,
    user: data.user,
  }
  saveMiniAuthSession(session)
  return session
}

export async function loginWithWeChat(): Promise<MiniLoginResult> {
  const login = await Taro.login()
  const data = await apiRequest<MiniAuthBundleData | MiniBindingRequiredData>(
    '/api/mini/auth/login',
    { auth: false, retryAuth: false, method: 'POST', data: { code: login.code } }
  )
  if (data.binding_required) {
    return { kind: 'binding-required', binding: data }
  }
  return { kind: 'authenticated', session: persistAuthBundle(data) }
}

export async function bindMiniAppAccount(input: MiniBindInput) {
  const data = await apiRequest<MiniAuthBundleData>('/api/mini/auth/bind', {
    auth: false,
    retryAuth: false,
    method: 'POST',
    data: {
      flow_token: input.flowToken,
      username: input.username,
      password: input.password,
      two_factor_code: input.twoFactorCode,
      accept_terms: input.acceptTerms,
    },
  })
  return persistAuthBundle(data)
}

export async function registerMiniAppAccount(input: MiniRegisterInput) {
  const data = await apiRequest<MiniAuthBundleData>('/api/mini/auth/register', {
    auth: false,
    retryAuth: false,
    method: 'POST',
    data: {
      flow_token: input.flowToken,
      username: input.username,
      password: input.password,
      email: input.email,
      verification_code: input.verificationCode,
      aff_code: input.affCode,
      accept_terms: input.acceptTerms,
    },
  })
  return persistAuthBundle(data)
}

export async function sendMiniAppEmailVerification(flowToken: string, email: string) {
  await apiRequest('/api/mini/auth/verification', {
    auth: false,
    retryAuth: false,
    method: 'POST',
    data: { flow_token: flowToken, email },
  })
}

export async function logoutMiniApp() {
  const session = getMiniAuthSession()
  if (!session) {
    return
  }
  try {
    await apiRequest('/api/mini/auth/logout', {
      retryAuth: false,
      method: 'POST',
      data: { refresh_token: session.refreshToken, sid: session.sid },
    })
  } finally {
    clearMiniAuthSession()
    clearConversation()
  }
}
