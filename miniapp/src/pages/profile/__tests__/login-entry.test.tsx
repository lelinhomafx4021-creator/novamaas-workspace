// @vitest-environment jsdom
import type { ReactNode } from 'react'
import { act, cleanup, render, screen, fireEvent } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { clearMiniAuthSession } from '@/auth/session'

import ProfilePage from '../index'
vi.mock('@tarojs/components', async () => {
  const { createElement } = await import('react')
  const nativeElement = (tag: string) => (props: Record<string, unknown>) => {
    return createElement(
      tag,
      {
        className: props.className,
        disabled: props.disabled,
        onClick: props.onClick ?? (props.onGetPhoneNumber ? () => (props.onGetPhoneNumber as (event: unknown) => void)({ detail: { code: 'fresh-phone-code' } }) : undefined),
        placeholder: props.placeholder,
        type: props.type,
        defaultValue: props.value,
        'aria-label': props.ariaLabel,
        'data-open-type': props.openType,
      },
      props.children as ReactNode
    )
  }
  return {
    Button: nativeElement('button'),
    Checkbox: nativeElement('input'),
    Image: nativeElement('img'),
    Input: nativeElement('input'),
    Text: nativeElement('span'),
    View: nativeElement('div'),
  }
})

vi.mock('@tarojs/taro', () => ({
  default: {
    getStorageSync: () => null,
    getAccountInfoSync: () => ({ miniProgram: { appId: 'wx-test-app' } }),
    login: vi.fn().mockResolvedValue({ code: 'login-code' }),
    request: vi.fn().mockResolvedValue({ statusCode: 401, data: { success: false, code: 'MINI_AUTH_PHONE_CODE_INVALID', message: 'Unauthorized' } }),
    setNavigationBarTitle: vi.fn(),
    setStorageSync: vi.fn(),
    removeStorageSync: vi.fn(),
  },
  useDidShow: vi.fn(),
  usePullDownRefresh: vi.fn(),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: { resolvedLanguage: 'en' },
  }),
}))
afterEach(() => { cleanup(); clearMiniAuthSession(); vi.unstubAllGlobals() })
vi.mock('@/api/status', () => ({ getPlatformStatus: vi.fn().mockResolvedValue({ sms_login: true, wechat_miniapp_login: true }) }))

it('signed-out login offers WeChat phone and existing account login without an SMS login entry', async () => {
  clearMiniAuthSession()
  await act(async () => { render(<ProfilePage />) })
  expect(screen.getByRole('button', { name: 'auth.wechatPhoneLogin' })).toBeDefined()
  expect(screen.getByRole('button', { name: 'auth.existingWeChatLogin' })).toBeDefined()
  expect(screen.queryByRole('button', { name: 'phone.login' })).toBeNull()
})

it('a rejected WeChat phone authorization stays on the WeChat page and does not leak into password login', async () => {
  vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example')
  await act(async () => { render(<ProfilePage />) })
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'auth.wechatPhoneLogin' })) })
  expect(screen.getByText('auth.error.phoneCodeInvalid')).toBeDefined()
  expect(screen.getByRole('button', { name: 'auth.wechatPhoneLogin' })).toBeDefined()
  fireEvent.click(screen.getByRole('button', { name: 'auth.existingWeChatLogin' }))
  expect(screen.queryByText('auth.error.phoneCodeInvalid')).toBeNull()
  expect(screen.getByPlaceholderText('auth.password')).toBeDefined()
})
