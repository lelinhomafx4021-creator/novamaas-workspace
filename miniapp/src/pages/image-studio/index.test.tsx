// @vitest-environment jsdom
import type { ReactNode } from 'react'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import Taro, { useDidShow } from '@tarojs/taro'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { getAccountSummary } from '@/api/dashboard'
import {
  assistStudioPrompt,
  createStudioRun,
  getStudioKeys,
  getStudioModels,
  getStudioRun,
  getStudioSettings,
  listStudioRuns,
  type StudioRun,
} from '@/api/image-studio'
import { ApiRequestError } from '@/api/request'
import { getMiniAuthSession } from '@/auth/session'
import { saveStudioImage } from '@/image-studio/images'

import ImageStudioPage from './index'

vi.mock('@tarojs/components', async () => {
  const { createElement } = await import('react')
  const element = (tag: string) => (props: Record<string, unknown>) =>
    createElement(
      tag,
      { className: props.className, style: props.style, id: props.id, onClick: props.onClick },
      props.children as ReactNode
    )
  const input = (tag: string) => (props: Record<string, unknown>) =>
    createElement(tag, {
      value: props.value,
      disabled: props.disabled,
      placeholder: props.placeholder,
      onChange: (event: { target: { value: string } }) =>
        (props.onInput as (event: unknown) => void)?.({ detail: { value: event.target.value } }),
    })
  return {
    View: element('div'),
    Text: element('span'),
    ScrollView: element('div'),
    Input: input('input'),
    Textarea: input('textarea'),
    Button: (props: Record<string, unknown>) =>
      createElement(
        'button',
        {
          className: props.className,
          'aria-pressed': props['aria-pressed'],
          'aria-expanded': props['aria-expanded'],
          disabled: props.disabled,
          onClick: props.onClick,
        },
        props.children as ReactNode
      ),
    Image: (props: Record<string, unknown>) =>
      createElement('img', {
        src: props.src,
        className: props.className,
        'data-mode': props.mode,
        onClick: props.onClick,
        onLoad: (event: { nativeEvent: CustomEvent }) =>
          (props.onLoad as (event: unknown) => void)?.({ detail: event.nativeEvent.detail }),
      }),
    Picker: (props: Record<string, unknown>) =>
      createElement(
        'select',
        {
          disabled: props.disabled,
          value: props.value,
          onChange: (event: { target: { value: string } }) =>
            (props.onChange as (event: unknown) => void)({ detail: { value: event.target.value } }),
        },
        (props.range as string[]).map((text, index) => createElement('option', { key: index, value: index }, text))
      ),
  }
})
vi.mock('@tarojs/taro', () => ({
  default: {
    setNavigationBarTitle: vi.fn(),
    switchTab: vi.fn(),
    showToast: vi.fn(),
    previewImage: vi.fn().mockResolvedValue({}),
    pageScrollTo: vi.fn().mockResolvedValue({}),
    openSetting: vi.fn().mockResolvedValue({}),
  },
  useDidShow: vi.fn(),
  useDidHide: vi.fn(),
}))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, values?: Record<string, unknown>) => (values?.id ? `${key}:${values.id}` : key),
    i18n: { resolvedLanguage: 'en' },
  }),
}))
vi.mock('@/api/image-studio', () => ({
  getStudioSettings: vi.fn(),
  getStudioKeys: vi.fn(),
  getStudioModels: vi.fn(),
  listStudioRuns: vi.fn(),
  createStudioRun: vi.fn(),
  assistStudioPrompt: vi.fn(),
  getStudioRun: vi.fn(),
}))
vi.mock('@/api/dashboard', () => ({ getAccountSummary: vi.fn() }))
vi.mock('@/auth/session', () => ({ getMiniAuthSession: vi.fn() }))
vi.mock('@/image-studio/images', () => ({
  saveStudioImage: vi.fn(),
}))

const run: StudioRun = {
  id: 'run-own',
  status: 'completed',
  prompt: 'A lake',
  model: 'image-a',
  results: [
    { id: 'result-own', preview_url: 'https://oss.example/image.png?signature=old', thumbnail_url: 'https://oss.example/thumb.png?signature=old' },
  ],
}
let onShow: () => void
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(useDidShow).mockImplementation(callback => {
    onShow = callback
  })
  vi.mocked(getMiniAuthSession).mockReturnValue({
    accessToken: 'access',
    refreshToken: 'refresh',
    accessExpiresAt: 9999999999,
    sid: 'sid',
    user: { id: 1, username: 'owner', display_name: 'Owner' },
  })
  vi.mocked(getAccountSummary).mockResolvedValue({ group: 'default' } as Awaited<ReturnType<typeof getAccountSummary>>)
  vi.mocked(getStudioSettings).mockResolvedValue({
    enabled: true,
    max_images: 2,
    max_prompt_characters: 32000,
  })
  vi.mocked(getStudioKeys).mockResolvedValue([
    { id: 7, name: 'key-a', group: '' },
    { id: 8, name: 'key-b', group: 'vip' },
  ] as Awaited<ReturnType<typeof getStudioKeys>>)
  vi.mocked(getStudioModels).mockImplementation(async (_id, kind) =>
    kind === 'image' ? ['image-a', 'image-b'] : ['text-a', 'text-b']
  )
  vi.mocked(listStudioRuns).mockResolvedValue({ items: [], total: 0 })
  vi.mocked(createStudioRun).mockResolvedValue(run)
  vi.mocked(getStudioRun).mockResolvedValue(run)
  vi.mocked(assistStudioPrompt).mockResolvedValue({ suggested_prompt: 'A cinematic lake' })
  vi.mocked(saveStudioImage).mockResolvedValue(undefined)
})
afterEach(() => cleanup())

async function ready() {
  await act(async () => {
    render(<ImageStudioPage />)
  })
  fireEvent.change(screen.getByPlaceholderText('studio.placeholder'), { target: { value: 'A lake' } })
  await waitFor(() =>
    expect((screen.getByRole('button', { name: 'studio.generate' }) as HTMLButtonElement).disabled).toBe(false)
  )
}

it('generates with an owned key, optimizes inline with undo and saves to the phone album', async () => {
  const renewed = { ...run, results: [{ ...run.results[0], preview_url: 'https://oss.example/image.png?signature=renewed', download_url: 'https://oss.example/image.png?signature=attachment' }] }
  vi.mocked(getStudioRun).mockResolvedValue(renewed)
  await ready()
  fireEvent.click(screen.getByRole('button', { name: 'studio.optimize' }))
  await waitFor(() =>
    expect((screen.getByPlaceholderText('studio.placeholder') as HTMLTextAreaElement).value).toBe('A cinematic lake')
  )
  fireEvent.click(screen.getByRole('button', { name: 'studio.undo' }))
  expect((screen.getByPlaceholderText('studio.placeholder') as HTMLTextAreaElement).value).toBe('A lake')
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('studio.result')
  expect(createStudioRun).toHaveBeenCalledWith(
    expect.objectContaining({ token_id: 7, request: { model: 'image-a', prompt: 'A lake' } })
  )
  expect(saveStudioImage).not.toHaveBeenCalled()
  await waitFor(() =>
    expect((screen.getByRole('button', { name: 'studio.save' }) as HTMLButtonElement).disabled).toBe(false)
  )
  expect(Taro.pageScrollTo).toHaveBeenCalledWith({ selector: '#studio-result', duration: 200 })
  fireEvent.click(screen.getByRole('button', { name: 'studio.preview' }))
  await waitFor(() => expect(Taro.previewImage).toHaveBeenCalledWith({ current: 'https://oss.example/image.png?signature=renewed', urls: ['https://oss.example/image.png?signature=renewed'] }))
  fireEvent.click(screen.getByRole('button', { name: 'studio.save' }))
  await waitFor(() => expect(saveStudioImage).toHaveBeenCalledWith('https://oss.example/image.png?signature=attachment'))
  expect(Taro.showToast).toHaveBeenCalledWith({ title: 'studio.saved', icon: 'success' })
})

it('optimizes with the selected text model when no image model is available', async () => {
  vi.mocked(getStudioModels).mockImplementation(async (_id, kind) => kind === 'text' ? ['text-a'] : [])
  await act(async () => { render(<ImageStudioPage />) })
  fireEvent.change(screen.getByPlaceholderText('studio.placeholder'), { target: { value: 'A lake' } })
  await waitFor(() => expect((screen.getByRole('button', { name: 'studio.optimize' }) as HTMLButtonElement).disabled).toBe(false))
  fireEvent.click(screen.getByRole('button', { name: 'studio.optimize' }))
  await waitFor(() => expect(assistStudioPrompt).toHaveBeenCalledWith(7, 'text-a', 'A lake'))
  expect((screen.getByRole('button', { name: 'studio.generate' }) as HTMLButtonElement).disabled).toBe(true)
  expect(createStudioRun).not.toHaveBeenCalled()
})

it('blocks repeat submission and reuses the same idempotency key after a network failure', async () => {
  let reject!: (error: Error) => void
  vi.mocked(createStudioRun).mockReturnValueOnce(
    new Promise((_resolve, fail) => {
      reject = fail
    })
  )
  await ready()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  fireEvent.click(screen.getByRole('button', { name: 'studio.generating' }))
  expect(createStudioRun).toHaveBeenCalledTimes(1)
  await act(async () => reject(new Error('network timeout')))
  expect(screen.queryByText('studio.result')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('studio.result')
  expect(vi.mocked(createStudioRun).mock.calls[1][0].idempotency_key).toBe(
    vi.mocked(createStudioRun).mock.calls[0][0].idempotency_key
  )
})

it('blocks disabled studio routes and shows retained own history after reopening', async () => {
  vi.mocked(getStudioSettings).mockResolvedValue({ enabled: false, max_images: 1, max_prompt_characters: 4000 })
  await act(async () => { render(<ImageStudioPage />) })
  expect(Taro.switchTab).toHaveBeenCalledWith({ url: '/pages/home/index' })
  expect(screen.queryByPlaceholderText('studio.placeholder')).toBeNull()
  expect(getStudioModels).not.toHaveBeenCalled()
  expect(listStudioRuns).not.toHaveBeenCalled()
  expect(createStudioRun).not.toHaveBeenCalled()
  vi.mocked(getStudioSettings).mockResolvedValue({ enabled: true, max_images: 1, max_prompt_characters: 4000 })
  vi.mocked(listStudioRuns).mockResolvedValue({ items: [run], total: 1 })
  await act(async () => { onShow(); onShow() })
  await screen.findByText('A lake')
  expect(screen.queryByText('studio.result')).toBeNull()
  expect(createStudioRun).not.toHaveBeenCalled()
})

it('drops a late generation response when the account changes', async () => {
  let complete!: (value: StudioRun) => void
  vi.mocked(createStudioRun).mockReturnValueOnce(
    new Promise(resolve => {
      complete = resolve
    })
  )
  await ready()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  vi.mocked(getMiniAuthSession).mockReturnValue(null)
  await act(async () => {
    onShow()
    complete(run)
  })
  expect(screen.getByText('studio.login')).toBeDefined()
  expect(screen.queryByText('studio.result')).toBeNull()
})

it('shows the sanitized server failure without adding a generated image', async () => {
  vi.mocked(createStudioRun).mockRejectedValue(new ApiRequestError('provider rejected this prompt', 400))
  await ready()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('provider rejected this prompt')
  expect(screen.queryByText('studio.result')).toBeNull()
  expect(saveStudioImage).not.toHaveBeenCalled()
})

it('keeps image and text keys independent and preserves the chosen models after success', async () => {
  await ready()
  expect(screen.queryByText('studio.textModel')).toBeNull()
  expect(screen.getByRole('button', { name: 'studio.configure' }).getAttribute('aria-expanded')).toBe('false')
  fireEvent.click(screen.getByRole('button', { name: 'studio.configure' }))
  expect(screen.getByRole('button', { name: 'studio.configure' }).getAttribute('aria-expanded')).toBe('true')
  const fields = screen.getAllByRole('combobox')
  expect(fields[0].textContent).toContain('key-a · default')
  fireEvent.change(fields[1], { target: { value: '1' } })
  fireEvent.change(fields[2], { target: { value: '1' } })
  await waitFor(() => expect(getStudioModels).toHaveBeenCalledWith(8, 'text'))
  fireEvent.click(screen.getByRole('button', { name: 'studio.configure' }))
  expect(screen.queryByText('studio.textModel')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'studio.optimize' }))
  await waitFor(() => expect(assistStudioPrompt).toHaveBeenCalledWith(8, 'text-a', 'A lake'))
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('studio.result')
  expect(createStudioRun).toHaveBeenCalledWith(
    expect.objectContaining({ token_id: 7, request: expect.objectContaining({ model: 'image-b' }) })
  )
  expect(screen.queryByText('studio.textModel')).toBeNull()
  await waitFor(() =>
    expect((screen.getByRole('button', { name: 'studio.configure' }) as HTMLButtonElement).disabled).toBe(false)
  )
  fireEvent.click(screen.getByRole('button', { name: 'studio.configure' }))
  expect(screen.getAllByRole('combobox')[2].textContent).toContain('key-b · vip')
  await waitFor(() => expect((screen.getAllByRole('combobox')[1] as HTMLSelectElement).value).toBe('1'))
})

it('uses model defaults without image count controls or extra request parameters', async () => {
  vi.mocked(getStudioModels).mockImplementation(async (_id, kind) =>
    kind === 'image' ? ['gpt-image-1', 'dall-e-3'] : ['text-a']
  )
  await ready()
  expect(screen.getByText('studio.modelDefault')).toBeTruthy()
  expect(screen.getByText('studio.proportionsHint')).toBeTruthy()
  expect(screen.queryByText('studio.quality')).toBeNull()
  expect(screen.queryByText('studio.count')).toBeNull()
  expect(screen.queryByRole('combobox')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('studio.result')
  expect(createStudioRun).toHaveBeenCalledWith(expect.objectContaining({
    request: { model: 'gpt-image-1', prompt: 'A lake' },
  }))
  await waitFor(() => expect((screen.getByRole('button', { name: 'studio.configure' }) as HTMLButtonElement).disabled).toBe(false))
  fireEvent.click(screen.getByRole('button', { name: 'studio.configure' }))
  fireEvent.change(screen.getAllByRole('combobox')[1], { target: { value: '1' } })
  fireEvent.click(screen.getByRole('button', { name: 'studio.configure' }))
  expect(screen.queryByRole('combobox')).toBeNull()
  expect(screen.queryByText('studio.singleImage')).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await waitFor(() => expect(createStudioRun).toHaveBeenCalledTimes(2))
  expect(createStudioRun).toHaveBeenLastCalledWith(expect.objectContaining({
    request: { model: 'dall-e-3', prompt: 'A lake' },
  }))
})

it('reuses the selected creation prompt without generation or album writes', async () => {
  await ready()
  fireEvent.click(screen.getByRole('button', { name: 'studio.generate' }))
  await screen.findByText('studio.result')
  fireEvent.change(screen.getByPlaceholderText('studio.placeholder'), { target: { value: 'Another idea' } })
  fireEvent.click(screen.getByRole('button', { name: 'studio.reusePrompt' }))
  expect((screen.getByPlaceholderText('studio.placeholder') as HTMLTextAreaElement).value).toBe('A lake')
  expect(createStudioRun).toHaveBeenCalledTimes(1)
  expect(saveStudioImage).not.toHaveBeenCalled()
  expect(Taro.pageScrollTo).toHaveBeenCalledWith({ selector: '#studio-composer', duration: 200 })
})

it('preserves original proportions and lets each image in a batch select its own main preview', async () => {
  const batch = {
    ...run,
    results: [
      { id: 'first', thumbnail_url: 'thumb-first', preview_url: 'original-first', description: 'First lake' },
      { id: 'second', thumbnail_url: 'thumb-second', preview_url: 'original-second', description: 'Second lake' },
    ],
  }
  vi.mocked(listStudioRuns).mockResolvedValue({ items: [batch], total: 1 })
  vi.mocked(getStudioRun).mockResolvedValue(batch)
  await ready()
  const second = await screen.findByRole('button', { name: /Second lake/ })
  expect(second.querySelector('img')?.getAttribute('data-mode')).toBe('aspectFit')
  fireEvent.click(second)
  await waitFor(() =>
    expect(document.querySelector('.studio-result')?.getAttribute('src')).toBe('original-second')
  )
  expect(second.getAttribute('aria-pressed')).toBe('true')
  const main = document.querySelector('.studio-result') as HTMLImageElement
  expect(main.getAttribute('data-mode')).toBe('aspectFit')
  fireEvent(main, new CustomEvent('load', { detail: { width: 600, height: 1200 } }))
  expect(Number.parseFloat(main.parentElement?.style.aspectRatio || '')).toBe(0.5)
  fireEvent.click(screen.getByRole('button', { name: /First lake/ }))
  await waitFor(() =>
    expect(document.querySelector('.studio-result')?.getAttribute('src')).toBe('original-first')
  )
  expect(Number.parseFloat((document.querySelector('.studio-result-frame') as HTMLElement).style.aspectRatio)).toBe(1)
  fireEvent(
    document.querySelector('.studio-result') as HTMLImageElement,
    new CustomEvent('load', { detail: { width: 1200, height: 600 } })
  )
  expect(Number.parseFloat((document.querySelector('.studio-result-frame') as HTMLElement).style.aspectRatio)).toBe(2)
  expect(createStudioRun).not.toHaveBeenCalled()
  expect(saveStudioImage).not.toHaveBeenCalled()
})
