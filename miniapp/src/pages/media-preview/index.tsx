import { Button, Image, Text, Video, View } from '@tarojs/components'
import Taro, { useRouter } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { usePageTitle } from '@/hooks/use-page-title'
import { readMediaPreviewRoute } from '@/usage/media-preview-route'

import './index.scss'

export default function MediaPreviewPage() {
  const { t } = useTranslation()
  const router = useRouter()
  const media = readMediaPreviewRoute(router.params)
  usePageTitle('usage.preview')

  const close = async () => {
    try {
      await Taro.navigateBack({ delta: 1 })
    } catch {
      await Taro.switchTab({ url: '/pages/usage/index' })
    }
  }

  return (
    <View className='media-preview-page'>
      <Button className='media-preview-page__back' onClick={close}>
        {t('usage.closePreview')}
      </Button>
      {media?.kind === 'image' ? (
        <Image className='media-preview-page__image' mode='aspectFit' src={media.filePath} />
      ) : null}
      {media?.kind === 'video' ? (
        <Video className='media-preview-page__video' src={media.filePath} controls />
      ) : null}
      {!media ? <Text className='mobile-error'>{t('usage.error')}</Text> : null}
    </View>
  )
}
