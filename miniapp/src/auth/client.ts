import Taro from '@tarojs/taro'

import { ApiRequestError, apiRequest } from '@/api/request'
import { readWeChatAvatar } from '@/api/wechat'
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
  confirmation_available?: boolean
  account_hint?: string
  matched_account?: {
    username: string
    display_name: string
    phone_hint: string
  }
  two_factor_required?: boolean
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
  password?: string
  twoFactorCode?: string
  username?: string
  confirmOnly?: boolean
  nickname?: string
  avatarPath?: string
}

export interface MiniPasswordLoginInput {
  password: string
  twoFactorCode?: string
  username: string
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

export async function loginWithWeChatPhone(
  phoneCode: string
): Promise<MiniLoginResult> {
  const authorizationCode = phoneCode.trim()
  if (!authorizationCode) {
    throw new ApiRequestError(
      'Phone authorization code is required',
      400,
      'MINI_AUTH_CODE_REQUIRED'
    )
  }
  let appId = ''
  try {
    appId = Taro.getAccountInfoSync().miniProgram.appId.trim()
  } catch {
    // Do not proceed without the native app identity in the WeChat runtime.
  }
  if (!appId) {
    throw new ApiRequestError(
      'WeChat app identity is unavailable',
      400,
      'MINI_AUTH_NOT_CONFIGURED'
    )
  }
  const login = await Taro.login()
  const loginCode = login.code?.trim()
  if (!loginCode) {
    throw new ApiRequestError(
      'WeChat login code is required',
      400,
      'MINI_AUTH_CODE_REQUIRED'
    )
  }
  const data = await apiRequest<MiniAuthBundleData | MiniBindingRequiredData>(
    '/api/mini/auth/phone',
    {
      auth: false,
      retryAuth: false,
      method: 'POST',
      data: { app_id: appId, code: loginCode, phone_code: authorizationCode },
    }
  )
  if (data.binding_required) {
    if (!data.matched_account?.username) {
      throw new ApiRequestError(
        'No verified platform account matches this phone',
        404,
        'MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND'
      )
    }
    return { kind: 'binding-required', binding: data }
  }
  return { kind: 'authenticated', session: persistAuthBundle(data) }
}

export async function loginWithPassword(input: MiniPasswordLoginInput) {
  const data = await apiRequest<MiniAuthBundleData>('/api/mini/auth/password', {
    auth: false,
    retryAuth: false,
    method: 'POST',
    data: {
      username: input.username.trim(),
      password: input.password,
      two_factor_code: input.twoFactorCode,
    },
  })
  return persistAuthBundle(data)
}

export async function bindMiniAppAccount(input: MiniBindInput) {
  const avatarBase64 = input.avatarPath
    ? await readWeChatAvatar(input.avatarPath)
    : undefined
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
      confirm_only: input.confirmOnly,
      nickname: input.nickname,
      avatar_base64: avatarBase64,
    },
  })
  return persistAuthBundle(data)
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

export interface SmsChallenge {
  challenge_token: string
  expires_in: number
  retry_after: number
}

export async function sendMiniSMS(phone: string, flowToken?: string) {
  const code = flowToken ? undefined : (await Taro.login()).code
  return apiRequest<SmsChallenge>(
    flowToken ? '/api/mini/auth/sms/mfa/code' : '/api/mini/auth/sms/code',
    {
      auth: false,
      retryAuth: false,
      method: 'POST',
      data: { phone, wx_code: code, flow_token: flowToken },
    }
  )
}

export async function loginWithMiniSMS(
  token: string,
  code: string,
  flowToken?: string
) {
  const data = await apiRequest<MiniAuthBundleData>(
    flowToken ? '/api/mini/auth/sms/mfa' : '/api/mini/auth/sms',
    {
      auth: false,
      retryAuth: false,
      method: 'POST',
      data: { challenge_token: token, code, flow_token: flowToken },
    }
  )
  return persistAuthBundle(data)
}
