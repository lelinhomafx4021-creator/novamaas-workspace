import { Text, View } from '@tarojs/components'
import { useTranslation } from 'react-i18next'

import { usePageTitle } from '@/hooks/use-page-title'

import { PageShell } from './page-shell'

import './feature-page.scss'

interface FeaturePageProps {
  descriptionKey: string
  titleKey: string
}

export function FeaturePage(props: FeaturePageProps) {
  const { t } = useTranslation()
  usePageTitle(props.titleKey)

  return (
    <PageShell
      title={t(props.titleKey)}
      description={t(props.descriptionKey)}
      trailing={<Text className='feature-page__badge'>{t('common.planned')}</Text>}
    >
      <View className='feature-page__card'>
        <View className='feature-page__marker' />
        <Text className='feature-page__next'>{t('feature.nextStep')}</Text>
      </View>
    </PageShell>
  )
}
