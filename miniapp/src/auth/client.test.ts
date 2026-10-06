import Taro from '@tarojs/taro'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiRequestError } from '@/api/request'

import {
  bindMiniAppAccount,
  loginWithMiniSMS,
  loginWithPassword,
  loginWithWeChatPhone,
  sendMiniSMS,
} from './client'
import { clearMiniAuthSession, getMiniAuthSession } from './session'

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: vi.fn(),
    getAccountInfoSync: vi.fn(),
    getFileInfo: vi.fn(),
    getFileSystemManager: vi.fn(),
    login: vi.fn(),
    removeStorageSync: vi.fn(),
    request: vi.fn(),
    setStorageSync: vi.fn(),
  },
}))

describe('mini program password login', () => {
  beforeEach(() => {
    vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example.com')
    vi.mocked(Taro.getAccountInfoSync).mockReturnValue({
      miniProgram: {
        appId: 'wx-native-app',
        envVersion: 'develop',
        version: '',
      },
      plugin: { appId: '', version: '' },
    })
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
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.example.com/api/mini/auth/password',
        method: 'POST',
        data: {
          username: '13800001234',
          password: 'password123',
          two_factor_code: '123456',
        },
      })
    )
    expect(session).toMatchObject({ accessToken: 'access', sid: 'session-1' })
    expect(getMiniAuthSession()).toMatchObject({
      accessToken: 'access',
      sid: 'session-1',
    })
  })

  it('requests SMS with a fresh native code without establishing an account connection', async () => {
    vi.mocked(Taro.login).mockResolvedValue({
      code: 'native-code',
      errMsg: 'login:ok',
    })
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          challenge_token: 'challenge',
          expires_in: 300,
          retry_after: 60,
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    expect(await sendMiniSMS('13800138000')).toMatchObject({
      challenge_token: 'challenge',
    })
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.example.com/api/mini/auth/sms/code',
        data: {
          phone: '13800138000',
          wx_code: 'native-code',
          flow_token: undefined,
        },
      })
    )
    expect(getMiniAuthSession()).toBeNull()
  })

  it('uses the existing password flow for SMS and retains the renewable mobile session', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          access_token: 'sms-access',
          access_expires_at: 2000,
          refresh_token: 'sms-refresh',
          session: { sid: 'sms-session' },
          user: { id: 7, username: 'existing', display_name: 'Existing' },
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    const result = await loginWithMiniSMS(
      'challenge',
      '123456',
      'password-flow'
    )
    expect(Taro.login).not.toHaveBeenCalled()
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.example.com/api/mini/auth/sms/mfa',
        data: {
          challenge_token: 'challenge',
          code: '123456',
          flow_token: 'password-flow',
        },
      })
    )
    expect(result).toMatchObject({
      refreshToken: 'sms-refresh',
      sid: 'sms-session',
    })
  })

  it('does not save a session before explicit WeChat confirmation', async () => {
    vi.mocked(Taro.login).mockResolvedValue({
      code: 'wx-confirm',
      errMsg: 'login:ok',
    })
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          binding_required: true,
          flow_token: 'binding-flow',
          confirmation_available: true,
          account_hint: 'Existing',
          matched_account: {
            username: 'existing',
            display_name: 'Existing',
            phone_hint: '+86 138****1234',
          },
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    const result = await loginWithWeChatPhone('phone-code')
    expect(result).toMatchObject({
      kind: 'binding-required',
      binding: { confirmation_available: true, account_hint: 'Existing' },
    })
    expect(getMiniAuthSession()).toBeNull()
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        data: {
          app_id: 'wx-native-app',
          code: 'wx-confirm',
          phone_code: 'phone-code',
        },
      })
    )
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          access_token: 'confirmed',
          access_expires_at: 2000,
          refresh_token: 'refresh',
          session: { sid: 'confirmed-session' },
          user: { id: 7, username: 'existing', display_name: 'Existing' },
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    await bindMiniAppAccount({
      flowToken: 'binding-flow',
      confirmOnly: true,
      acceptTerms: true,
      username: '',
      password: '',
    })
    expect(Taro.request).toHaveBeenLastCalledWith(
      expect.objectContaining({
        data: expect.objectContaining({
          flow_token: 'binding-flow',
          confirm_only: true,
          accept_terms: true,
        }),
      })
    )
    expect(getMiniAuthSession()).toMatchObject({ accessToken: 'confirmed' })
  })

  it('preserves the SMS alternative returned with an authenticator challenge', async () => {
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 403,
      data: {
        success: false,
        code: 'MINI_AUTH_2FA_REQUIRED',
        data: { flow_token: 'password-flow', sms_available: true },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    const request = loginWithPassword({
      username: 'existing',
      password: 'password123',
    })
    await expect(request).rejects.toBeInstanceOf(ApiRequestError)
    await expect(request).rejects.toMatchObject({
      details: { flow_token: 'password-flow', sms_available: true },
    })
    expect(getMiniAuthSession()).toBeNull()
  })

  it('stops an unmatched phone response without exposing an account binding flow', async () => {
    vi.mocked(Taro.login).mockResolvedValue({
      code: 'native-code',
      errMsg: 'login:ok',
    })
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          binding_required: true,
          flow_token: 'legacy-unmatched-flow',
          registration_enabled: true,
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    await expect(loginWithWeChatPhone('phone-code')).rejects.toMatchObject({
      code: 'MINI_AUTH_PHONE_ACCOUNT_NOT_FOUND',
    })
    expect(Taro.request).toHaveBeenCalledOnce()
    expect(getMiniAuthSession()).toBeNull()
  })

  it('includes the selected profile only in explicit confirmation and rejects oversized avatars first', async () => {
    vi.mocked(Taro.getFileInfo).mockResolvedValue({
      size: 2 * 1024 * 1024 + 1,
      errMsg: 'getFileInfo:ok',
    })
    const input = {
      flowToken: 'matched-flow',
      confirmOnly: true,
      username: 'existing',
      password: '',
      acceptTerms: true,
      nickname: 'Chosen name',
      avatarPath: '/tmp/chosen-avatar.jpg',
    }
    await expect(bindMiniAppAccount(input)).rejects.toMatchObject({
      code: 'MINI_AUTH_PROFILE_INVALID',
    })
    expect(Taro.request).not.toHaveBeenCalled()
    expect(getMiniAuthSession()).toBeNull()
    vi.mocked(Taro.getFileInfo).mockResolvedValue({
      size: 100,
      errMsg: 'getFileInfo:ok',
    })
    vi.mocked(Taro.getFileSystemManager).mockReturnValue({
      readFile: ({
        success,
      }: {
        success: (result: { data: string }) => void
      }) => success({ data: 'avatar-base64' }),
    } as unknown as ReturnType<typeof Taro.getFileSystemManager>)
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 200,
      data: {
        success: true,
        data: {
          access_token: 'confirmed',
          access_expires_at: 2000,
          refresh_token: 'refresh',
          session: { sid: 'confirmed-session' },
          user: { id: 7, username: 'existing', display_name: 'Existing' },
        },
      },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    await bindMiniAppAccount(input)
    expect(Taro.request).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.example.com/api/mini/auth/bind',
        data: expect.objectContaining({
          flow_token: 'matched-flow',
          confirm_only: true,
          nickname: 'Chosen name',
          avatar_base64: 'avatar-base64',
        }),
      })
    )
    expect(getMiniAuthSession()).toMatchObject({ accessToken: 'confirmed' })
  })
  it('does not send a phone authorization request without a native app identity', async () => {
    vi.mocked(Taro.getAccountInfoSync).mockImplementationOnce(() => {
      throw new Error('native API unavailable')
    })
    await expect(loginWithWeChatPhone('phone-code')).rejects.toMatchObject({
      code: 'MINI_AUTH_NOT_CONFIGURED',
    })
    expect(Taro.login).not.toHaveBeenCalled()
    expect(Taro.request).not.toHaveBeenCalled()
  })

  it('does not submit empty login or phone authorization codes', async () => {
    await expect(loginWithWeChatPhone(' ')).rejects.toMatchObject({
      code: 'MINI_AUTH_CODE_REQUIRED',
    })
    expect(Taro.login).not.toHaveBeenCalled()
    vi.mocked(Taro.login).mockResolvedValue({ code: '', errMsg: 'login:ok' })
    await expect(loginWithWeChatPhone('phone-code')).rejects.toMatchObject({
      code: 'MINI_AUTH_CODE_REQUIRED',
    })
    expect(Taro.request).not.toHaveBeenCalled()
  })

  it('does not retry one-time WeChat codes or persist a session on rejected code exchange', async () => {
    vi.mocked(Taro.login).mockResolvedValue({
      code: 'native-code',
      errMsg: 'login:ok',
    })
    vi.mocked(Taro.request).mockResolvedValue({
      statusCode: 401,
      data: { success: false, code: 'MINI_AUTH_CODE_INVALID' },
      header: {},
      cookies: [],
      errMsg: 'request:ok',
    })
    await expect(loginWithWeChatPhone('phone-code')).rejects.toMatchObject({
      code: 'MINI_AUTH_CODE_INVALID',
    })
    expect(Taro.request).toHaveBeenCalledOnce()
    expect(Taro.login).toHaveBeenCalledOnce()
    expect(getMiniAuthSession()).toBeNull()
  })
})
