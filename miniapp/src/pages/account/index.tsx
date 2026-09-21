import { useCallback, useEffect, useState } from 'react'

import { Button, Input, Picker, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import {
  getAccountProfile,
  updateAccountLanguage,
  updateAccountProfile,
  type AccountProfile,
} from '@/api/account'
import { getMiniAuthSession, saveMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import {
  parseAccountLanguage,
  supportedLanguages,
  type SupportedLanguage,
} from '@/account/preferences'
import { changeLanguage } from '@/i18n/config'

import './index.scss'

export default function AccountPage() {
  const { i18n, t } = useTranslation()
  const [profile, setProfile] = useState<AccountProfile | null>(null)
  const [displayName, setDisplayName] = useState('')
  const [language, setLanguage] = useState<SupportedLanguage>(() => {
    const active = i18n.resolvedLanguage || i18n.language
    return supportedLanguages.includes(active as SupportedLanguage)
      ? (active as SupportedLanguage)
      : 'en'
  })
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)

  usePageTitle('account.title')

  const load = useCallback(async () => {
    setLoading(true)
    setError(false)
    try {
      const nextProfile = await getAccountProfile()
      setProfile(nextProfile)
      setDisplayName(nextProfile.display_name)
      const serverLanguage = parseAccountLanguage(nextProfile.setting)
      if (serverLanguage) {
        setLanguage(serverLanguage)
        await changeLanguage(serverLanguage)
      }
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const languageLabels = supportedLanguages.map((code) =>
    t(`account.language.${code}`)
  )
  const languageIndex = Math.max(0, supportedLanguages.indexOf(language))

  const selectLanguage = async (index: number) => {
    const nextLanguage = supportedLanguages[index]
    if (!nextLanguage || nextLanguage === language || busy) return
    setBusy(true)
    setError(false)
    try {
      await updateAccountLanguage(nextLanguage)
      await changeLanguage(nextLanguage)
      setLanguage(nextLanguage)
      await Taro.showToast({ title: t('account.saved'), icon: 'success' })
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const saveProfile = async () => {
    if (!profile || busy) return
    const nextDisplayName = displayName.trim()
    setBusy(true)
    setError(false)
    try {
      await updateAccountProfile(nextDisplayName)
      setProfile({ ...profile, display_name: nextDisplayName })
      const session = getMiniAuthSession()
      if (session) {
        saveMiniAuthSession({
          ...session,
          user: { ...session.user, display_name: nextDisplayName },
        })
      }
      await Taro.showToast({ title: t('account.saved'), icon: 'success' })
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <PageShell title={t('account.title')} description={t('account.description')}>
      {loading ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {error ? <Text className='mobile-error'>{t('account.loadError')}</Text> : null}
      {profile ? (
        <View className='mobile-card mobile-stack'>
          <View>
            <Text className='settings-label'>{t('account.displayName')}</Text>
            <Input
              className='mobile-input'
              maxlength={20}
              value={displayName}
              onInput={(event) => setDisplayName(event.detail.value)}
            />
          </View>
          <View>
            <Text className='settings-label'>{t('account.language')}</Text>
            <Picker
              mode='selector'
              range={languageLabels}
              value={languageIndex}
              onChange={(event) => void selectLanguage(Number(event.detail.value))}
            >
              <View className='mobile-picker'>{languageLabels[languageIndex]}</View>
            </Picker>
          </View>
          <Button className='mobile-button' disabled={busy || !displayName.trim()} onClick={() => void saveProfile()}>
            {t('account.save')}
          </Button>
        </View>
      ) : null}
    </PageShell>
  )
}
