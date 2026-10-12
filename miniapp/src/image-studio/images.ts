import Taro from '@tarojs/taro'

export async function saveStudioImage(url: string) {
  // OSS signatures authorize this download; do not send the platform access token.
  const download = await Taro.downloadFile({ url })
  try {
    if (download.statusCode !== 200 || !download.tempFilePath) throw new Error('STUDIO_DOWNLOAD_FAILED')
    await Taro.saveImageToPhotosAlbum({ filePath: download.tempFilePath })
  } catch (error) {
    const setting = await Taro.getSetting().catch(() => null)
    if (setting?.authSetting['scope.writePhotosAlbum'] === false) throw new Error('STUDIO_ALBUM_PERMISSION')
    throw error
  } finally {
    if (download.tempFilePath) {
      Taro.getFileSystemManager().unlink({ filePath: download.tempFilePath, fail: () => {} })
    }
  }
}
