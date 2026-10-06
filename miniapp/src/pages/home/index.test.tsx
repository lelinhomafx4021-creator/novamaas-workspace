import type { ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, expect, it, vi } from 'vitest'

import { clearMiniAuthSession, saveMiniAuthSession } from '@/auth/session'

import HomePage from './index'
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
afterEach(() => clearMiniAuthSession())
it('signed-in home keeps account features without showing WeChat binding information', () => {
  saveMiniAuthSession({
    accessToken: 'test-access',
    refreshToken: 'test-refresh',
    sid: 'test-session',
    accessExpiresAt: 2000,
    user: { id: 1, username: 'fixture-user', display_name: 'Fixture' },
  })
  const markup = renderToStaticMarkup(<HomePage />)
  expect(markup).toContain('home.statusTitle')
  expect(markup).not.toContain('wechat.title')
  expect(markup).not.toContain('wechat-binding-panel')
})
