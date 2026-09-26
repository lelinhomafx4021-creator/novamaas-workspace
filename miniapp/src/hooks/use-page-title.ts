import { useEffect } from 'react'

import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

export function usePageTitle(translationKey: string) {
  const { t, i18n } = useTranslation()

  useEffect(() => {
    void Taro.setNavigationBarTitle({ title: t(translationKey) })
  }, [i18n.resolvedLanguage, t, translationKey])
}
