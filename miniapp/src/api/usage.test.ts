import Taro from '@tarojs/taro'
import { describe, expect, it, vi } from 'vitest'

import { apiRequest } from './request'
import { getDrawingTasks, getTaskInformation, getTaskPollHistory, getTaskRequestSnapshots, getTasks } from './usage'

vi.mock('@tarojs/taro', () => ({ default: { request: vi.fn() } }))
vi.mock('./request', () => ({
  ApiRequestError: class extends Error {},
  apiRequest: vi.fn(),
  getAuthorizedMiniSession: vi.fn(async () => ({ accessToken: 'session-token' })),
  getConfiguredApiBaseUrl: vi.fn(() => 'https://api.example'),
}))

describe('media task time filters', () => {
  it('sends seconds for standard tasks and inclusive milliseconds for drawing tasks', () => {
    const startTimestamp = Date.parse('2026-09-29T16:00:00Z') / 1000
    const endTimestamp = Date.parse('2026-09-30T15:59:59Z') / 1000

    getTasks({ startTimestamp, endTimestamp })
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toContain(`start_timestamp=${startTimestamp}`)
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toContain(`end_timestamp=${endTimestamp}`)

    getDrawingTasks({ startTimestamp, endTimestamp })
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toContain(`start_timestamp=${startTimestamp * 1000}`)
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toContain(`end_timestamp=${endTimestamp * 1000 + 999}`)
  })
})

describe('administrator task inspection endpoints', () => {
  it('uses task-specific request snapshots and paginated polling history', () => {
    getTaskRequestSnapshots('task/one')
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toBe('/api/task/task%2Fone/request-snapshots')

    getTaskPollHistory('task/one', 42)
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toBe('/api/task/task%2Fone/poll-history?before_id=42&limit=50')
  })

  it('retrieves supported task information with the user session', async () => {
    vi.mocked(Taro.request).mockResolvedValue({ statusCode: 200, data: { id: 'task-one' } } as Awaited<ReturnType<typeof Taro.request>>)
    await expect(getTaskInformation('task/one', '54')).resolves.toEqual({ id: 'task-one' })
    expect(Taro.request).toHaveBeenCalledWith({
      url: 'https://api.example/v1/video/generations/task%2Fone',
      method: 'GET',
      header: { Authorization: 'Bearer session-token', Accept: 'application/json' },
    })
    await expect(getTaskInformation('task/one', 'suno')).rejects.toThrow('TASK_INFORMATION_UNSUPPORTED')
  })
})
