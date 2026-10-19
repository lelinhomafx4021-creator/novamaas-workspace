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
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { getFreshAuthHeaders } from '@/lib/api'

import type { SupplierTestRunRequest } from '../../types'
import { useSupplierTestRun } from '../use-supplier-test-run'

const mockStream = vi.hoisted(() => ({
  created: 0,
  closed: 0,
  listeners: new Map<
    string,
    (event: Event & { responseCode?: number }) => void
  >(),
}))

vi.mock('@/lib/api', () => ({ getFreshAuthHeaders: vi.fn() }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
vi.mock('sse.js', () => ({
  SSE: class {
    constructor() {
      mockStream.created += 1
    }
    addEventListener(
      type: string,
      listener: (event: Event & { responseCode?: number }) => void
    ) {
      mockStream.listeners.set(type, listener)
    }
    close() {
      mockStream.closed += 1
    }
    stream() {}
  },
}))

const payload = {
  modules: ['basic'],
  basic: { checks: [] },
} as unknown as SupplierTestRunRequest

beforeEach(() => {
  mockStream.created = 0
  mockStream.closed = 0
  mockStream.listeners.clear()
})

test('leaving the page closes an active supplier test stream', async () => {
  vi.mocked(getFreshAuthHeaders).mockResolvedValue({})
  const { result, unmount } = renderHook(() => useSupplierTestRun())

  await act(async () => {
    await result.current.start(payload)
  })
  expect(mockStream.created).toBe(1)

  unmount()
  expect(mockStream.closed).toBe(1)
})

test('stop before authentication completes prevents starting the stream', async () => {
  let resolveHeaders!: (headers: Record<string, string>) => void
  vi.mocked(getFreshAuthHeaders).mockReturnValue(
    new Promise((resolve) => {
      resolveHeaders = resolve
    })
  )
  const { result } = renderHook(() => useSupplierTestRun())
  let startPromise!: Promise<void>
  act(() => {
    startPromise = result.current.start(payload)
  })
  act(() => result.current.stop())
  await act(async () => {
    resolveHeaders({})
    await startPromise
  })
  expect(mockStream.created).toBe(0)
  expect(result.current.runningModule).toBeNull()
})

test('an HTTP 200 stream closing without done reports an error', async () => {
  vi.mocked(getFreshAuthHeaders).mockResolvedValue({})
  const { result } = renderHook(() => useSupplierTestRun())
  await act(async () => {
    await result.current.start(payload)
  })
  expect(mockStream.created).toBe(1)
  act(() =>
    mockStream.listeners.get('error')?.(
      Object.assign(new Event('error'), { responseCode: 200 })
    )
  )
  await waitFor(() => {
    expect(result.current.errorMessage).toBe('Test stream ended unexpectedly')
    expect(result.current.runningModule).toBeNull()
  })
})

test('manual video query keeps a translatable status and its task state separate', () => {
  const { result } = renderHook(() => useSupplierTestRun())

  act(() => {
    result.current.updateVideoMetricsWithQueryResult({
      success: true,
      task_id: 'video-42',
      status: 'queued',
    })
  })

  const pollCheck = result.current.videoChecks.find(
    (check) => check.id === 'video_poll'
  )
  expect(pollCheck).toMatchObject({
    status: 'running',
    message: 'Current status: {{status}}',
    messageArgs: { status: 'queued' },
  })
})

test('four model/resolution runs accumulate immutable snapshots, supplementary queries update one record, and changing supplier resets the report', async () => {
  vi.mocked(getFreshAuthHeaders).mockResolvedValue({})
  const { result } = renderHook(() => useSupplierTestRun())
  let number = 0
  for (const model of ['model-a', 'model-b']) {
    for (const resolution of ['720p', '1080p']) {
      const request = {
        ...payload,
        base_url: 'https://supplier.example',
        model,
        modules: ['video'],
        video: { prompt: 'animate' },
      } as SupplierTestRunRequest
      await act(async () => {
        await result.current.start(request)
      })
      const task = `task-${number++}`
      act(() => {
        mockStream.listeners.get('message')?.(
          Object.assign(new Event('message'), {
            data: JSON.stringify({
              type: 'check',
              module: 'video',
              check_id: 'video_result',
              status: 'pass',
              video: {
                task_id: task,
                status: 'succeeded',
                elapsed_ms: 100,
                video_url: `https://cdn.example/${task}.mp4`,
                poll_count: 1,
                raw_request_json: JSON.stringify({ model, resolution }),
                submit: {
                  method: 'POST',
                  url: 'https://supplier.example/tasks',
                  headers: '',
                  request: '{}',
                  response: task,
                  status: 200,
                  request_id: 'submit',
                  elapsed_ms: 1,
                },
                final: {
                  method: 'GET',
                  url: `https://supplier.example/tasks/${task}`,
                  headers: '',
                  request: '',
                  response: '{"status":"succeeded"}',
                  status: 200,
                  request_id: 'final',
                  elapsed_ms: 1,
                },
              },
            }),
          })
        )
        mockStream.listeners.get('message')?.(
          Object.assign(new Event('message'), { data: '[DONE]' })
        )
      })
    }
  }
  expect(result.current.videoRuns).toHaveLength(4)
  expect(
    new Set(result.current.videoRuns.map((record) => record.id)).size
  ).toBe(4)
  expect(result.current.videoRuns.map((record) => record.outcome)).toEqual([
    'Succeeded',
    'Succeeded',
    'Succeeded',
    'Succeeded',
  ])
  const original = result.current.videoRuns[0].metrics.submit
  act(() =>
    result.current.updateVideoMetricsWithQueryResult({
      base_url: 'https://supplier.example',
      success: true,
      task_id: 'task-0',
      status: 'succeeded',
      video_url: 'https://cdn.example/updated.mp4',
      exchange: {
        method: 'GET',
        url: 'https://supplier.example/tasks/task-0',
        headers: '',
        request: '',
        response: '{}',
        status: 200,
        request_id: 'supplement',
        elapsed_ms: 1,
      },
    })
  )
  expect(result.current.videoRuns).toHaveLength(4)
  expect(result.current.videoRuns[0].metrics.submit).toEqual(original)
  expect(result.current.videoRuns[0].metrics.video_url).toContain('updated')
  expect(result.current.videoRuns[1].metrics.task_id).toBe('task-1')
  act(() =>
    result.current.updateVideoVerification(result.current.videoRuns[1].id, {
      attempted: true,
      queryId: 'only-case-1',
      count: 1,
      elapsed: 1,
    })
  )
  expect(result.current.videoRuns[0].verification).toBeUndefined()
  expect(result.current.videoRuns[1].verification?.queryId).toBe('only-case-1')
  await act(async () => {
    await result.current.start({
      ...payload,
      modules: ['video'],
      base_url: 'https://other.example',
      model: 'model',
      video: { prompt: 'animate' },
    } as SupplierTestRunRequest)
  })
  expect(result.current.videoRuns).toHaveLength(1)
  expect(result.current.videoRuns[0].baseUrl).toBe('https://other.example')
  act(() => result.current.stop())
  expect(result.current.videoRuns[0].outcome).toBe('Cancelled')
  act(() => result.current.clearVideoReport())
  expect(result.current.videoRuns).toEqual([])
})

test('a failed manual query for a different task cannot reuse a previous video URL or submit receipt', () => {
  const { result } = renderHook(() => useSupplierTestRun())
  act(() =>
    result.current.updateVideoMetricsWithQueryResult({
      base_url: 'https://supplier.example',
      success: true,
      task_id: 'old',
      status: 'succeeded',
      video_url: 'https://cdn.example/old.mp4',
    })
  )
  act(() =>
    result.current.updateVideoMetricsWithQueryResult({
      base_url: 'https://supplier.example',
      success: false,
      task_id: 'new',
      message: 'Not found',
    })
  )
  expect(result.current.videoMetrics?.video_url).toBeUndefined()
  expect(result.current.videoRuns).toHaveLength(2)
  expect(result.current.videoRuns[1].metrics.submit).toBeUndefined()
  expect(result.current.videoRuns[1].outcome).toBe('Failed')
})
