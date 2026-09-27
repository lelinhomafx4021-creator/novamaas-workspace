import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { clearMiniAuthSession, getMiniAuthSession } from './session'
import { loginWithPassword } from './client'

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn(),
    login: vi.fn(),
    removeStorageSync: vi.fn(),
    request: vi.fn(),
    setStorageSync: vi.fn(),
  },
}))

describe('mini program password login', () => {
  beforeEach(() => {
    vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example.com')
    vi.mocked(Taro.login).mockReset()
    vi.mocked(Taro.request).mockReset()
    clearMiniAuthSession()
  })

  it('signs in without requesting a WeChat code and preserves the 2FA code', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        message: '',
        data: {
          access_token: 'access',
          access_expires_at: 1000,
          refresh_token: 'refresh',
          session: { sid: 'session-1' },
          user: { id: 7, username: 'existing', display_name: 'Existing' },
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })

    const session = await loginWithPassword({
      username: '  13800001234  ',
      password: 'password123',
      twoFactorCode: '123456',
    })

    expect(Taro.login).not.toHaveBeenCalled()
    expect(Taro.request).toHaveBeenCalledWith(expect.objectContaining({
      url: 'https://api.example.com/api/mini/auth/password',
      method: 'POST',
      data: {
        username: '13800001234',
        password: 'password123',
        two_factor_code: '123456',
      },
    }))
    expect(session).toMatchObject({ accessToken: 'access', sid: 'session-1' })
    expect(getMiniAuthSession()).toMatchObject({ accessToken: 'access', sid: 'session-1' })
  })
})
