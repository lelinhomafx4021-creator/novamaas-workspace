// @vitest-environment jsdom
import type { ReactNode } from 'react'
import { act, cleanup, render, screen } from '@testing-library/react'
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
afterEach(() => { cleanup(); clearMiniAuthSession() })
vi.mock('@/api/status', () => ({ getPlatformStatus: vi.fn().mockResolvedValue({ sms_login: true, wechat_miniapp_login: true }) }))

it('signed-out login offers WeChat phone and existing account login without an SMS login entry', async () => {
  clearMiniAuthSession()
  await act(async () => { render(<ProfilePage />) })
  expect(screen.getByRole('button', { name: 'auth.wechatPhoneLogin' })).toBeDefined()
  expect(screen.getByRole('button', { name: 'auth.existingWeChatLogin' })).toBeDefined()
  expect(screen.queryByRole('button', { name: 'phone.login' })).toBeNull()
})
