import { Image, Text, View } from '@tarojs/components'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

export function StudioThumbnail(props: { source?: string }) {
  const { t } = useTranslation()
  const [failedSource, setFailedSource] = useState('')
  return props.source && props.source !== failedSource ? (
    <Image className='studio-thumb' src={props.source} mode='aspectFit' onError={() => setFailedSource(props.source || '')} />
  ) : (
    <Text className='studio-thumb studio-thumb--placeholder'>{failedSource ? '—' : t('common.loading')}</Text>
  )
}

export function StudioImagePreview(props: { path: string; onPreview: () => void; onError: () => void }) {
  const [ratio, setRatio] = useState(1)
  return (
    <View className='studio-result-frame' style={{ aspectRatio: String(ratio) }}>
      <Image
        className='studio-result'
        src={props.path}
        mode='aspectFit'
        onClick={props.onPreview}
        onError={props.onError}
        onLoad={event => {
          const width = Number(event.detail.width)
          const height = Number(event.detail.height)
          if (width > 0 && height > 0 && Number.isFinite(width / height)) setRatio(width / height)
        }}
      />
    </View>
  )
}
