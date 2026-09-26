import { useEffect, useState } from 'react'

import { Button, RichText, Text, View } from '@tarojs/components'
import { useRouter } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getPrivacyPolicy, getUserAgreement } from '@/api/legal'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'

import './index.scss'

type LegalState =
  | { phase: 'loading' }
  | { phase: 'ready'; content: string }
  | { phase: 'error' }

export default function LegalPage() {
  const router = useRouter()
  const { t } = useTranslation()
  const [reloadCount, setReloadCount] = useState(0)
  const [state, setState] = useState<LegalState>({ phase: 'loading' })
  const privacy = router.params.kind === 'privacy'
  const titleKey = privacy ? 'legal.titlePrivacy' : 'legal.titleAgreement'

  usePageTitle(titleKey)

  useEffect(() => {
    let active = true
    setState({ phase: 'loading' })
    const request = privacy ? getPrivacyPolicy() : getUserAgreement()
    request
      .then((content) => {
        if (active) {
          setState({ phase: 'ready', content })
        }
      })
      .catch(() => {
        if (active) {
          setState({ phase: 'error' })
        }
      })
    return () => {
      active = false
    }
  }, [privacy, reloadCount])

  return (
    <PageShell title={t(titleKey)} description={t('legal.description')}>
      <View className='legal-card'>
        {state.phase === 'loading' ? (
          <Text className='legal-message'>{t('legal.loading')}</Text>
        ) : null}
        {state.phase === 'error' ? (
          <>
            <Text className='legal-message legal-message--error'>
              {t('legal.error')}
            </Text>
            <Button
              className='legal-retry'
              onClick={() => setReloadCount((count) => count + 1)}
            >
              {t('common.retry')}
            </Button>
          </>
        ) : null}
        {state.phase === 'ready' && state.content ? (
          <RichText className='legal-content' nodes={state.content} />
        ) : null}
        {state.phase === 'ready' && !state.content ? (
          <Text className='legal-message'>{t('legal.empty')}</Text>
        ) : null}
      </View>
    </PageShell>
  )
}
