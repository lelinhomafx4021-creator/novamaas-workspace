import { beforeEach, expect, it, vi } from 'vitest'

import { createStudioRun, getStudioKeys, getStudioModels, listStudioRuns } from './image-studio'
import { apiRequest } from './request'
import { getApiTokens } from './tokens'

vi.mock('./request', () => ({ apiRequest: vi.fn().mockResolvedValue({}) }))
vi.mock('./tokens', () => ({ getApiTokens: vi.fn() }))
beforeEach(() => vi.clearAllMocks())

it('uses existing authenticated generation and history without asset writes or review scope', async () => {
  await createStudioRun({
    token_id: 7,
    idempotency_key: 'same-request',
    request: { model: 'image-model', prompt: 'A lake' },
  })
  expect(apiRequest).toHaveBeenCalledWith('/api/image-studio/runs', {
    method: 'POST',
    timeout: 300_000,
    header: { 'Idempotency-Key': 'same-request' },
    data: {
      token_id: 7,
      idempotency_key: 'same-request',
      request: {
        model: 'image-model',
        prompt: 'A lake',
      },
    },
  })
  await listStudioRuns(2)
  expect(apiRequest).toHaveBeenLastCalledWith('/api/image-studio/runs?p=2&page_size=8')
  await getStudioModels(7, 'text')
  expect(apiRequest).toHaveBeenLastCalledWith('/api/image-studio/models?token_id=7&kind=text')
})

it('finds enabled own keys across every page', async () => {
  vi.mocked(getApiTokens)
    .mockResolvedValueOnce({ total: 2, items: [{ id: 1, status: 2 }] } as Awaited<ReturnType<typeof getApiTokens>>)
    .mockResolvedValueOnce({ total: 2, items: [{ id: 2, status: 1 }] } as Awaited<ReturnType<typeof getApiTokens>>)
  await expect(getStudioKeys()).resolves.toEqual([{ id: 2, status: 1 }])
  expect(getApiTokens).toHaveBeenNthCalledWith(2, 2, 100)
})
