// @vitest-environment jsdom
import type { ReactNode } from 'react'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import Taro from '@tarojs/taro'
import { renderToStaticMarkup } from 'react-dom/server'
import { afterEach, expect, it, vi } from 'vitest'

import { clearMiniAuthSession, saveMiniAuthSession } from '@/auth/session'

import HomePage from './index'
import { getPlatformStatus } from '@/api/status'
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
        onClick: props.onClick,
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
    setNavigationBarTitle: vi.fn(),
    navigateTo: vi.fn(),
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
vi.mock('@/api/status', () => ({ getPlatformStatus: vi.fn().mockResolvedValue({ setup: true }) }))
vi.mock('@/api/dashboard', () => ({
  getAccountSummary: vi.fn().mockResolvedValue({ username: 'owner', group: 'default' }),
  getPlatformNotice: vi.fn().mockResolvedValue(''),
  getSubscriptionSummary: vi.fn().mockResolvedValue({ subscriptions: [] }),
  getUsageStats: vi.fn().mockResolvedValue({ quota: 0, requests: 0 }),
}))
afterEach(() => {
  cleanup()
  clearMiniAuthSession()
  vi.unstubAllGlobals()
})
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

it('shows the image entry only while the shared feature switch is enabled', async () => {
  vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example.com')
  saveMiniAuthSession({
    accessToken: 'access',
    refreshToken: 'refresh',
    sid: 'sid',
    accessExpiresAt: 9999999999,
    user: { id: 1, username: 'owner', display_name: 'Owner' },
  })
  vi.mocked(getPlatformStatus).mockResolvedValue({ ImageStudioEnabled: true, setup: true, system_name: 'New API', version: 'test' })
  await act(async () => {
    render(<HomePage />)
  })
  fireEvent.click(screen.getByRole('button', { name: 'studio.start' }))
  expect(Taro.navigateTo).toHaveBeenCalledWith({ url: '/pages/image-studio/index' })
  cleanup()
  vi.mocked(getPlatformStatus).mockResolvedValue({ ImageStudioEnabled: false, setup: true, system_name: 'New API', version: 'test' })
  await act(async () => {
    render(<HomePage />)
  })
  expect(screen.queryByRole('button', { name: 'studio.start' })).toBeNull()
})
