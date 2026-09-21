import { describe, expect, it } from 'vitest'

import type { DrawingTask, PlatformTask, UsageLog } from '@/api/usage'

import {
  getCacheTokens,
  getDrawingMedia,
  getEffectiveGroupRatio,
  getTaskMedia,
  getTaskStatusKey,
  getUsageLogTypeKey,
  isTerminalTaskStatus,
  parseUsageLogOther,
  showsUsageCost,
} from './presentation'

describe('usage presentation', () => {
  it('maps log and task states to stable translation keys', () => {
    expect(getUsageLogTypeKey(7)).toBe('usage.logType.login')
    expect(getUsageLogTypeKey(999)).toBe('usage.logType.unknown')
    expect(getTaskStatusKey('IN_PROGRESS')).toBe('usage.taskStatus.processing')
    expect(isTerminalTaskStatus('SUCCESS')).toBe(true)
    expect(isTerminalTaskStatus('QUEUED')).toBe(false)
    expect(showsUsageCost({ type: 5 } as UsageLog)).toBe(true)
    expect(showsUsageCost({ type: 7 } as UsageLog)).toBe(false)
  })

  it('parses user-visible log metadata and split cache tokens', () => {
    const other = parseUsageLogOther(
      JSON.stringify({ cache_tokens: 12, cache_creation_tokens_5m: 5, cache_creation_tokens_1h: 7 })
    )
    expect(getCacheTokens(other)).toEqual({ read: 12, write: 12 })
    expect(parseUsageLogOther('{invalid')).toEqual({})
    expect(getEffectiveGroupRatio({ group_ratio: 1.2, user_group_ratio: -1 })).toBe(1.2)
    expect(getEffectiveGroupRatio({ group_ratio: 1.2, user_group_ratio: 0.8 })).toBe(0.8)
  })

  it('collects deduplicated task media without exposing unrelated URLs', () => {
    const task = {
      data: [
        { audio_url: 'https://media.example/song.mp3', source_url: 'https://private.example/raw' },
        { image_url: 'https://media.example/cover.png' },
      ],
      result_url: 'https://media.example/video.mp4',
    } as PlatformTask
    expect(getTaskMedia(task)).toEqual([
      { kind: 'video', url: 'https://media.example/video.mp4' },
      { kind: 'audio', url: 'https://media.example/song.mp3' },
      { kind: 'image', url: 'https://media.example/cover.png' },
    ])

    const drawing = {
      image_url: 'https://media.example/output.webp',
      video_url: '',
      video_urls: JSON.stringify([{ url: 'https://media.example/proxy/clip' }]),
    } as DrawingTask
    expect(getDrawingMedia(drawing)).toEqual([
      { kind: 'image', url: 'https://media.example/output.webp' },
      { kind: 'video', url: 'https://media.example/proxy/clip' },
    ])

    const audioTask = {
      action: 'song',
      data: [],
      platform: 'suno',
      result_url: 'https://media.example/proxy/audio',
    } as PlatformTask
    expect(getTaskMedia(audioTask)).toEqual([
      { kind: 'audio', url: 'https://media.example/proxy/audio' },
    ])
  })

  it('keeps the API log contract fields used by details rendering', () => {
    const log = {
      channel: 2,
      type: 2,
      upstream_request_id: 'upstream-1',
      username: 'member',
    } as UsageLog
    expect(log).toMatchObject({ channel: 2, upstream_request_id: 'upstream-1' })
  })
})
