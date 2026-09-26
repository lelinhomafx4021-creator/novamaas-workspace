import Taro from '@tarojs/taro'

import {
  clearMiniAuthSession,
  getMiniAuthSession,
  saveMiniAuthSession,
  type MiniAuthSession,
  type MiniAuthUser,
} from '@/auth/session'
import { clearConversation } from '@/playground/storage'

import { buildApiUrl, normalizeApiBaseUrl } from './url'

interface ApiEnvelope<T> {
  code?: string
  data: T
  message?: string
  success: boolean
}

export interface RequestOptions {
  auth?: boolean
  data?: unknown
  header?: Record<string, string>
  method?: keyof Taro.request.Method
  retryAuth?: boolean
}

interface MiniAuthBundleData {
  access_expires_at: number
  access_token: string
  refresh_token: string
  session: { sid: string }
  user: MiniAuthUser
}

let refreshInFlight: Promise<MiniAuthSession> | undefined

export class ApiRequestError extends Error {
  readonly code?: string
  readonly statusCode?: number

  constructor(message: string, statusCode?: number, code?: string) {
    super(message)
    this.name = 'ApiRequestError'
    this.statusCode = statusCode
    this.code = code
  }
}

export function getConfiguredApiBaseUrl() {
  const configuredValue =
    typeof MINIAPP_API_BASE_URL === 'string' ? MINIAPP_API_BASE_URL : ''
  return normalizeApiBaseUrl(configuredValue)
}

async function requestOnce<T>(
  path: string,
  options: RequestOptions,
  session: MiniAuthSession | null
) {
  const header: Record<string, string> = {
    Accept: 'application/json',
    ...options.header,
  }
  if (options.auth !== false && session?.accessToken) {
    header.Authorization = `Bearer ${session.accessToken}`
  }
  const response = await Taro.request<ApiEnvelope<T>>({
    url: buildApiUrl(getConfiguredApiBaseUrl(), path),
    method: options.method ?? 'GET',
    data: options.data,
    header,
  })

  if (response.statusCode < 200 || response.statusCode >= 300) {
    throw new ApiRequestError(
      response.data?.message || `Request failed with status ${response.statusCode}`,
      response.statusCode,
      response.data?.code
    )
  }
  if (!response.data?.success) {
    throw new ApiRequestError(
      response.data?.message || 'Request failed',
      response.statusCode,
      response.data?.code
    )
  }

  return response.data.data
}

async function rotateMiniAuthSession(session: MiniAuthSession) {
  refreshInFlight ??= requestOnce<MiniAuthBundleData>(
    '/api/mini/auth/refresh',
    {
      auth: false,
      retryAuth: false,
      method: 'POST',
      data: { refresh_token: session.refreshToken, sid: session.sid },
    },
    null
  )
    .then((data) => {
      const nextSession: MiniAuthSession = {
        accessToken: data.access_token,
        accessExpiresAt: data.access_expires_at,
        refreshToken: data.refresh_token,
        sid: data.session.sid,
        user: data.user,
      }
      saveMiniAuthSession(nextSession)
      return nextSession
    })
    .catch((error) => {
      clearMiniAuthSession()
      clearConversation()
      throw error
    })
    .finally(() => {
      refreshInFlight = undefined
    })
  return refreshInFlight
}

export async function getAuthorizedMiniSession() {
  const session = getMiniAuthSession()
  if (!session?.refreshToken) {
    throw new ApiRequestError('Authentication required', 401, 'MINI_AUTH_REQUIRED')
  }
  if (session.accessExpiresAt > Math.floor(Date.now() / 1000) + 30) {
    return session
  }
  return rotateMiniAuthSession(session)
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}) {
  const session = options.auth === false ? null : getMiniAuthSession()
  try {
    return await requestOnce<T>(path, options, session)
  } catch (error) {
    if (
      !(error instanceof ApiRequestError) ||
      error.statusCode !== 401 ||
      options.auth === false ||
      options.retryAuth === false ||
      !session?.refreshToken
    ) {
      throw error
    }
    const nextSession = await rotateMiniAuthSession(session)
    return requestOnce<T>(path, { ...options, retryAuth: false }, nextSession)
  }
}
