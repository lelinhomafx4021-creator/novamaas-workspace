import { useEffect, useState } from 'react'

import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { deleteAccount, getAccountProfile, type AccountProfile } from '@/api/account'
import { clearMiniAuthSession } from '@/auth/session'
import { isAccountDeletionConfirmed } from '@/account/preferences'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import { clearConversation } from '@/playground/storage'

import './index.scss'

export default function PrivacyPage() {
  const { t } = useTranslation()
  const [profile, setProfile] = useState<AccountProfile | null>(null)
  const [confirmation, setConfirmation] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)

  usePageTitle('privacy.title')

  useEffect(() => {
    void getAccountProfile().then(setProfile).catch(() => setError(true))
  }, [])

  const removeAccount = async () => {
    if (!profile || busy) return
    if (!isAccountDeletionConfirmed(confirmation, profile.username)) {
      await Taro.showToast({ title: t('privacy.usernameMismatch'), icon: 'none' })
      return
    }
    const confirmed = await Taro.showModal({
      content: t('privacy.deleteConfirm'),
      confirmColor: '#b42318',
    })
    if (!confirmed.confirm) return
    setBusy(true)
    setError(false)
    try {
      await deleteAccount()
      clearMiniAuthSession()
      clearConversation()
      await Taro.showToast({ title: t('privacy.deleted'), icon: 'success' })
      await Taro.switchTab({ url: '/pages/profile/index' })
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <PageShell title={t('privacy.title')} description={t('privacy.description')}>
      {error ? <Text className='mobile-error'>{t('privacy.deleteFailed')}</Text> : null}
      <View className='mobile-card mobile-stack'>
        <Text className='mobile-card__title'>{t('privacy.documents')}</Text>
        <Button className='mobile-button mobile-button--secondary' onClick={() => Taro.navigateTo({ url: '/pages/legal/index?kind=agreement' })}>
          {t('privacy.userAgreement')}
        </Button>
        <Button className='mobile-button mobile-button--secondary' onClick={() => Taro.navigateTo({ url: '/pages/legal/index?kind=privacy' })}>
          {t('privacy.policy')}
        </Button>
      </View>

      <View className='mobile-card'>
        <Text className='mobile-card__title'>{t('privacy.collection')}</Text>
        <Text className='mobile-card__meta'>{t('privacy.collectionDetail')}</Text>
      </View>

      <View className='mobile-card mobile-stack settings-danger'>
        <Text className='mobile-card__title'>{t('privacy.deleteTitle')}</Text>
        <Text className='mobile-error'>{t('privacy.deleteWarning')}</Text>
        {profile ? (
          <Input
            className='mobile-input'
            value={confirmation}
            placeholder={t('privacy.confirmPlaceholder', { username: profile.username })}
            onInput={(event) => setConfirmation(event.detail.value)}
          />
        ) : null}
        <Button
          className='mobile-button mobile-button--danger'
          disabled={busy || !profile || !isAccountDeletionConfirmed(confirmation, profile.username)}
          onClick={() => void removeAccount()}
        >
          {t('privacy.deleteTitle')}
        </Button>
      </View>
    </PageShell>
  )
}
