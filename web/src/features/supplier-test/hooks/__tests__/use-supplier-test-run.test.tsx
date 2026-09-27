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
