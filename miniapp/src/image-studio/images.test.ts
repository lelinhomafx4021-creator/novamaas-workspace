import Taro from '@tarojs/taro'
import { beforeEach, expect, it, vi } from 'vitest'

import { saveStudioImage } from './images'

const fs = vi.hoisted(() => ({ unlink: vi.fn() }))
vi.mock('@tarojs/taro', () => ({
  default: {
    getFileSystemManager: () => fs,
    downloadFile: vi.fn(),
    saveImageToPhotosAlbum: vi.fn(),
    getSetting: vi.fn(),
  },
}))
beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(Taro.downloadFile).mockResolvedValue({
    statusCode: 200, tempFilePath: '/temp/image.png', filePath: '', errMsg: 'ok',
  })
  vi.mocked(Taro.saveImageToPhotosAlbum).mockResolvedValue({ errMsg: 'ok' })
  vi.mocked(Taro.getSetting).mockResolvedValue({
    authSetting: {}, miniprogramAuthSetting: {},
    subscriptionsSetting: { mainSwitch: false, itemSettings: {} }, errMsg: 'ok',
  })
})

it('downloads the signed OSS image without credentials and releases the temporary file after saving', async () => {
  const url = 'https://oss.example/image.png?signature=valid'
  await saveStudioImage(url)
  expect(Taro.downloadFile).toHaveBeenCalledWith({ url })
  expect(Taro.saveImageToPhotosAlbum).toHaveBeenCalledWith({ filePath: '/temp/image.png' })
  expect(fs.unlink).toHaveBeenCalledWith(expect.objectContaining({ filePath: '/temp/image.png' }))
})

it('rejects failed OSS downloads before album writes and clears the downloaded error response', async () => {
  vi.mocked(Taro.downloadFile).mockResolvedValue({
    statusCode: 403, tempFilePath: '/temp/error.html', filePath: '', errMsg: 'ok',
  })
  await expect(saveStudioImage('https://oss.example/image.png')).rejects.toThrow('STUDIO_DOWNLOAD_FAILED')
  expect(Taro.saveImageToPhotosAlbum).not.toHaveBeenCalled()
  expect(fs.unlink).toHaveBeenCalledWith(expect.objectContaining({ filePath: '/temp/error.html' }))
})

it('reports denied album permission so the user can recover and still releases the temporary file', async () => {
  vi.mocked(Taro.saveImageToPhotosAlbum).mockRejectedValue({ errMsg: 'permission denied' })
  vi.mocked(Taro.getSetting).mockResolvedValue({
    authSetting: { 'scope.writePhotosAlbum': false }, miniprogramAuthSetting: {},
    subscriptionsSetting: { mainSwitch: false, itemSettings: {} }, errMsg: 'ok',
  })
  await expect(saveStudioImage('https://oss.example/image.png')).rejects.toThrow('STUDIO_ALBUM_PERMISSION')
  expect(fs.unlink).toHaveBeenCalledWith(expect.objectContaining({ filePath: '/temp/image.png' }))
})
