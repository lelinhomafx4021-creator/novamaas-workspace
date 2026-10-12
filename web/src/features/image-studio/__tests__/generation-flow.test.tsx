import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { useAuthStore } from '@/stores/auth-store'

import * as api from '../api'
import { ImageStudio } from '../image-studio'
import type { StudioRun } from '../types'

vi.mock('../api')
vi.mock('@/features/keys/api')

const completed: StudioRun = {
  id: 'run-1',
  owner_user_id: 7,
  status: 'completed',
  prompt: 'A mountain',
  created_at: 1,
  model: 'dall-e-3',
  quota: 0,
  results: [
    {
      id: 'result-1',
      index: 0,
      preview_url: 'https://example.test/original.png',
      thumbnail_url: 'https://example.test/thumbnail.png',
    },
  ],
}
const key = {
  id: 1,
  name: 'Image key',
  key: '***',
  status: 1,
  group: '',
} as ApiKey

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  useAuthStore
    .getState()
    .auth.setUser({ id: 7, username: 'artist', role: 1, group: 'default' })
  vi.mocked(api.getStudioSettings).mockResolvedValue({
    enabled: true,
    max_images: 4,
    max_prompt_characters: 1000,
  })
  vi.mocked(getApiKeys).mockResolvedValue({
    success: true,
    data: { items: [key], total: 1, page: 1, page_size: 100 },
  })
  vi.mocked(api.listStudioModels).mockImplementation(async (_id, kind) =>
    kind === 'text' ? ['text-model'] : ['dall-e-3']
  )
  vi.mocked(api.listStudioRuns).mockResolvedValue({ items: [], total: 0 })
})
afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.resetAllMocks()
})

async function generate() {
  render(
    <QueryClientProvider client={client}>
      <ImageStudio />
    </QueryClientProvider>
  )
  await screen.findByText('dall-e-3', { exact: true })
  const user = userEvent.setup()
  await user.type(
    screen.getByRole('textbox', { name: 'Describe your image' }),
    'A mountain'
  )
  await user.click(screen.getByRole('button', { name: 'Create images' }))
  return user
}

test('a failed submission retries with the same idempotency key and nothing is saved by default', async () => {
  vi.mocked(api.createStudioRun)
    .mockRejectedValueOnce(new Error('network'))
    .mockResolvedValueOnce({ run: completed, replay: false })
  const user = await generate()
  await screen.findByText('network')
  await user.click(screen.getByRole('button', { name: 'Create images' }))
  await screen.findByRole('img', { name: 'A mountain' })
  const calls = vi.mocked(api.createStudioRun).mock.calls
  expect(calls).toHaveLength(2)
  expect(calls[0][0]).toEqual({
    token_id: 1,
    idempotency_key: expect.any(String),
    request: { model: 'dall-e-3', prompt: 'A mountain' },
  })
  expect(calls[1][0].idempotency_key).toBe(calls[0][0].idempotency_key)
  expect(api.saveStudioResult).not.toHaveBeenCalled()
  expect(
    screen.getByRole('button', {
      name: 'Save to asset library (for video generation)',
    })
  ).toBeEnabled()
})

test('a replayed run that is still generating shows progress and can be refreshed', async () => {
  const running: StudioRun = { ...completed, status: 'running', results: [] }
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: running,
    replay: true,
  })
  vi.mocked(api.getStudioRun).mockResolvedValue(running)
  const user = await generate()
  const progress = await screen.findByText(
    'This generation is still in progress'
  )
  vi.mocked(api.getStudioRun).mockResolvedValue(completed)
  await user.click(within(progress).getByRole('button', { name: 'Refresh' }))
  await screen.findByRole('img', { name: 'A mountain' })
  await waitFor(() =>
    expect(
      screen.queryByText('This generation is still in progress')
    ).not.toBeInTheDocument()
  )
})
