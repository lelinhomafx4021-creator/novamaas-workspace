// @vitest-environment jsdom
import { createElement, type ReactNode } from 'react'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import Taro from '@tarojs/taro'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { clearMiniAuthSession, saveMiniAuthSession } from '@/auth/session'
import { initializeI18n } from '@/i18n/config'
import { resources } from '@/i18n/resources'
import { getTodayRange, getYesterdayRange } from '@/utils/format'
import UsagePage from '../index'

vi.mock('@tarojs/components', () => {
  const element = (tag: string) => (props: Record<string, unknown>) => createElement(tag, {
    className: props.className,
    onClick: props.onClick,
    disabled: props.disabled,
  }, props.children as ReactNode)
  return { Button: element('button'), Image: element('img'), Input: element('input'), Picker: element('div'), Text: element('span'), View: element('div') }
})
vi.mock('@tarojs/taro', () => ({
  default: {
    request: vi.fn(), getStorageSync: vi.fn(), setStorageSync: vi.fn(), removeStorageSync: vi.fn(),
    getAppBaseInfo: () => ({ language: 'en' }), setNavigationBarTitle: vi.fn(),
  },
  useDidShow: vi.fn(), useDidHide: vi.fn(),
}))
const label = (key: keyof typeof resources.en.translation) => resources.en.translation[key]
const fixedNow = new Date(2026, 9, 7, 12).getTime()

beforeEach(async () => {
  vi.stubGlobal('MINIAPP_API_BASE_URL', 'https://api.example')
  vi.spyOn(Date, 'now').mockReturnValue(fixedNow)
  saveMiniAuthSession({ accessToken: 'access', accessExpiresAt: fixedNow / 1000 + 600, refreshToken: 'refresh', sid: 'sid', user: { id: 1, username: 'admin', display_name: 'Admin' } })
  vi.mocked(Taro.request).mockImplementation((options) => {
    const path = new URL(options.url).pathname
    const data = path === '/api/user/self' ? { role: 10 } : path.endsWith('/stat') ? { quota: 0, rpm: 0, tpm: 0 } : { items: [], total: 0, page: 1, page_size: 20 }
    return Promise.resolve({ statusCode: 200, data: { success: true, data } }) as ReturnType<typeof Taro.request>
  })
  await initializeI18n()
})
afterEach(() => { cleanup(); clearMiniAuthSession(); vi.restoreAllMocks(); vi.clearAllMocks(); vi.unstubAllGlobals() })

function requestedRange(path: string) {
  const call = [...vi.mocked(Taro.request).mock.calls].reverse().find(([options]) => new URL(options.url).pathname.replace(/\/$/, '') === path)
  if (!call) return null
  const query = new URL(call[0].url).searchParams
  return { startTimestamp: Number(query.get('start_timestamp')), endTimestamp: Number(query.get('end_timestamp')) }
}

test('usage and media tasks initially request today instead of the last thirty days', async () => {
  render(<UsagePage />)
  await waitFor(() => expect(requestedRange('/api/log/self')).toEqual(getTodayRange(fixedNow)))
  fireEvent.click(screen.getByRole('button', { name: label('usage.tasks') }))
  await waitFor(() => expect(requestedRange('/api/task/self')).toEqual(getTodayRange(fixedNow)))
})

test('a selected yesterday range survives switching data scope and media tabs', async () => {
  render(<UsagePage />)
  await waitFor(() => expect(screen.getByRole('button', { name: label('usage.scopePersonal') })).toBeDefined())
  fireEvent.click(screen.getByRole('button', { name: label('common.yesterday') }))
  await waitFor(() => expect(requestedRange('/api/log/self')).toEqual(getYesterdayRange(fixedNow)))
  fireEvent.click(screen.getByRole('button', { name: label('usage.tasks') }))
  await waitFor(() => expect(requestedRange('/api/task/self')).toEqual(getYesterdayRange(fixedNow)))
  fireEvent.click(screen.getByRole('button', { name: label('usage.scopePlatform') }))
  await waitFor(() => expect(requestedRange('/api/task')).toEqual(getYesterdayRange(fixedNow)))
  fireEvent.click(screen.getByRole('button', { name: label('usage.logs') }))
  await waitFor(() => expect(requestedRange('/api/log')).toEqual(getYesterdayRange(fixedNow)))
})
