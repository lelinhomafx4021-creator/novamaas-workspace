/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { BASIC_CHECKS, CACHE_CHECKS, VIDEO_CHECKS } from '../constants'
import type { SupplierTestRunRequest } from '../types'

const runMock = vi.hoisted(() => ({ start: vi.fn(), stop: vi.fn() }))

vi.mock('../hooks/use-supplier-test-run', () => ({
  useSupplierTestRun: () => ({
    runningModule: null,
    basicChecks: BASIC_CHECKS,
    cacheChecks: CACHE_CHECKS,
    videoChecks: VIDEO_CHECKS,
    streamText: '',
    basicStreamText: '',
    progress: { completed: 0, total: 0 },
    metrics: null,
    cacheMetrics: null,
    videoMetrics: null,
    summaries: { basic: '', cache: '', stress: '' },
    errorMessage: '',
    start: runMock.start,
    stop: runMock.stop,
  }),
}))

const { SupplierTest } = await import('../index')
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
})

afterEach(() => {
  queryClient.clear()
  sessionStorage.clear()
  runMock.start.mockClear()
})

test('polling an existing video task starts without a model', async () => {
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={queryClient}>
      <SupplierTest />
    </QueryClientProvider>
  )

  await user.type(screen.getByLabelText('Base URL'), 'https://api.example.com')
  await user.click(screen.getByRole('tab', { name: 'Doubao Video' }))
  await user.type(
    screen.getByPlaceholderText('Enter Task ID or generate by submitting'),
    'existing-task'
  )
  await user.click(screen.getByRole('button', { name: 'Poll Only' }))

  expect(runMock.start).toHaveBeenCalledWith(
    expect.objectContaining({
      model: '',
      modules: ['video'],
      video: expect.objectContaining({
        task_id: 'existing-task',
        checks: ['video_poll'],
      }),
    })
  )
})

test('video submission sends material and generation parameters once in the native request body', async () => {
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={queryClient}>
      <SupplierTest />
    </QueryClientProvider>
  )
  await user.type(screen.getByLabelText('Base URL'), 'https://api.example.com')
  await user.type(screen.getByLabelText('Model'), 'video-model')
  await user.click(screen.getByRole('tab', { name: 'Doubao Video' }))
  await user.type(screen.getByLabelText('Generation Prompt'), 'animate')
  await user.type(
    screen.getByRole('textbox', { name: 'Material URL or asset ID' }),
    'asset://reference-one'
  )
  await user.click(screen.getByRole('button', { name: 'Add material' }))
  await user.click(screen.getByRole('checkbox', { name: 'Watermark' }))
  await user.click(screen.getByRole('button', { name: 'Start Video Test' }))

  const request = runMock.start.mock.calls[0]?.[0] as SupplierTestRunRequest
  expect(request.video).toEqual({
    scenario: 'reference',
    platform_mode: false,
    verify_official: true,
    preparation_run_ids: [],
    checks: undefined,
    task_id: undefined,
    raw_payload: expect.any(String),
    custom_path: undefined,
  })
  expect(JSON.parse(request.video?.raw_payload || '{}')).toEqual({
    model: 'video-model',
    content: [
      { type: 'text', text: 'animate' },
      {
        type: 'image_url',
        image_url: { url: 'asset://reference-one' },
        role: 'reference_image',
      },
    ],
    watermark: false,
  })
})
