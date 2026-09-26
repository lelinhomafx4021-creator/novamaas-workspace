import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  clearMiniAuthSession,
  getMiniAuthSession,
  saveMiniAuthSession,
} from '@/auth/session'

import { apiRequest } from '../request'

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn(),
    removeStorageSync: vi.fn(),
    request: vi.fn(),
    setStorageSync: vi.fn(),
  },
}))

describe('API request envelope', () => {
  beforeEach(() => {
    vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example.com')
    vi.mocked(Taro.request).mockReset()
    clearMiniAuthSession()
  })

  it('returns data from a successful platform response', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        message: '',
        data: { version: '1.0.0' },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })

    await expect(apiRequest<{ version: string }>('/api/status')).resolves.toEqual(
      { version: '1.0.0' }
    )
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({ url: 'https://api.example.com/api/status' })
    )
  })

  it('surfaces an unsuccessful platform envelope', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: { success: false, message: 'setup required', data: null },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })

    await expect(apiRequest('/api/status')).rejects.toMatchObject({
      name: 'ApiRequestError',
      message: 'setup required',
    })
  })

  it('performs one refresh for concurrent 401 responses and retries with the rotated token', async () => {
    saveMiniAuthSession({
      accessToken: 'old-access',
      accessExpiresAt: 100,
      refreshToken: 'old-refresh',
      sid: 'sid-1',
      user: { id: 7, username: 'mini-user', display_name: 'Mini User' },
    })
    let protectedRequests = 0
    let refreshRequests = 0
    const requestMock = async (options: {
      header?: Record<string, string>
      url: string
    }) => {
      if (options.url.endsWith('/api/mini/auth/refresh')) {
        refreshRequests += 1
        return {
          statusCode: 200,
          data: {
            success: true,
            message: '',
            data: {
              access_token: 'new-access',
              access_expires_at: 200,
              refresh_token: 'new-refresh',
              session: { sid: 'sid-1' },
              user: { id: 7, username: 'mini-user', display_name: 'Mini User' },
            },
          },
          header: {},
          cookies: [],
          errMsg: 'request:ok',
        } as never
      }

      protectedRequests += 1
      if (protectedRequests <= 2) {
        return {
          statusCode: 401,
          data: {
            success: false,
            code: 'AUTH_TOKEN_EXPIRED',
            message: 'expired',
            data: null,
          },
          header: {},
          cookies: [],
          errMsg: 'request:ok',
        } as never
      }
      return {
        statusCode: 200,
        data: { success: true, message: '', data: { ok: true } },
        header: {},
        cookies: [],
        errMsg: 'request:ok',
      } as never
    }
    vi.mocked(Taro.request).mockImplementation(requestMock as never)

    await expect(
      Promise.all([
        apiRequest<{ ok: boolean }>('/api/private'),
        apiRequest<{ ok: boolean }>('/api/private'),
      ])
    ).resolves.toEqual([{ ok: true }, { ok: true }])

    expect(refreshRequests).toBe(1)
    expect(protectedRequests).toBe(4)
    expect(getMiniAuthSession()).toMatchObject({
      accessToken: 'new-access',
      refreshToken: 'new-refresh',
    })
    const retryCalls = vi.mocked(Taro.request).mock.calls.filter(
      ([options]) =>
        options.url.endsWith('/api/private') &&
        options.header?.Authorization === 'Bearer new-access'
    )
    expect(retryCalls).toHaveLength(2)
  })

  it('clears the local session when refresh fails', async () => {
    saveMiniAuthSession({
      accessToken: 'expired-access',
      accessExpiresAt: 100,
      refreshToken: 'expired-refresh',
      sid: 'sid-2',
      user: { id: 8, username: 'expired-user', display_name: 'Expired User' },
    })
    vi.mocked(Taro.request)
      .mockResolvedValueOnce({
        statusCode: 401,
        data: { success: false, message: 'expired', data: null },
        header: {},
        cookies: [],
        errMsg: 'request:ok',
      })
      .mockResolvedValueOnce({
        statusCode: 401,
        data: { success: false, message: 'refresh rejected', data: null },
        header: {},
        cookies: [],
        errMsg: 'request:ok',
      })

    await expect(apiRequest('/api/private')).rejects.toMatchObject({
      statusCode: 401,
    })
    expect(getMiniAuthSession()).toBeNull()
  })
})
