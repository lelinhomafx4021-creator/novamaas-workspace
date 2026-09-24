import Taro from '@tarojs/taro'

const AUTH_STORAGE_KEY = 'miniapp:auth:v1'

export interface MiniAuthUser {
  display_name: string
  id: number
  username: string
}

export interface MiniAuthSession {
  accessExpiresAt: number
  accessToken: string
  refreshToken: string
  sid: string
  user: MiniAuthUser
}

let cachedSession: MiniAuthSession | null | undefined

function isMiniAuthSession(value: unknown): value is MiniAuthSession {
  if (!value || typeof value !== 'object') {
    return false
  }
  const candidate = value as Partial<MiniAuthSession>
  return (
    typeof candidate.accessToken === 'string' &&
    candidate.accessToken !== '' &&
    typeof candidate.refreshToken === 'string' &&
    candidate.refreshToken !== '' &&
    typeof candidate.sid === 'string' &&
    candidate.sid !== '' &&
    typeof candidate.accessExpiresAt === 'number' &&
    !!candidate.user &&
    typeof candidate.user.id === 'number' &&
    typeof candidate.user.username === 'string'
  )
}

export function getMiniAuthSession() {
  if (cachedSession !== undefined) {
    return cachedSession
  }
  try {
    const stored = Taro.getStorageSync(AUTH_STORAGE_KEY)
    cachedSession = isMiniAuthSession(stored) ? stored : null
  } catch {
    cachedSession = null
  }
  return cachedSession
}

export function saveMiniAuthSession(session: MiniAuthSession) {
  cachedSession = session
  try {
    Taro.setStorageSync(AUTH_STORAGE_KEY, session)
  } catch {
    // Keep the in-memory session usable when persistent storage is unavailable.
  }
}

export function clearMiniAuthSession() {
  cachedSession = null
  try {
    Taro.removeStorageSync(AUTH_STORAGE_KEY)
  } catch {
    // Logout must still clear the in-memory credentials.
  }
}
