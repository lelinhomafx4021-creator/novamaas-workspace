import Taro from '@tarojs/taro'
import { describe, expect, it, vi } from 'vitest'

import { imageDataUrl } from './playground-media'

vi.mock('@tarojs/taro', () => ({ default: { getFileSystemManager: vi.fn() } }))

describe('playground image attachment', () => {
  it('uses a supported local image format in an OpenAI image data URL', async () => {
    vi.mocked(Taro.getFileSystemManager).mockReturnValue({
      readFile: vi.fn(({ success }) => success({ data: '/9j/' })),
    } as unknown as ReturnType<typeof Taro.getFileSystemManager>)
    await expect(imageDataUrl('wxfile://saved/image.jpg')).resolves.toBe('data:image/jpeg;base64,/9j/')
  })

  it('rejects unsupported or oversized image data before sending', async () => {
    vi.mocked(Taro.getFileSystemManager).mockReturnValue({
      readFile: vi.fn(({ success }) => success({ data: 'R0lG' })),
    } as unknown as ReturnType<typeof Taro.getFileSystemManager>)
    await expect(imageDataUrl('wxfile://saved/unknown')).rejects.toThrow('IMAGE_INVALID')

    vi.mocked(Taro.getFileSystemManager).mockReturnValue({
      readFile: vi.fn(({ success }) => success({ data: `/9j/${'A'.repeat(1_398_104)}` })),
    } as unknown as ReturnType<typeof Taro.getFileSystemManager>)
    await expect(imageDataUrl('wxfile://saved/too-large')).rejects.toThrow('IMAGE_INVALID')
  })
})
