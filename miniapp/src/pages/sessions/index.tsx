import { useCallback, useEffect, useState } from 'react'

import { Button, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import {
  getLoginSessions,
  revokeLoginSession,
  revokeOtherLoginSessions,
  type LoginSession,
} from '@/api/account'
import { clearMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import { clearConversation } from '@/playground/storage'
import { formatTime } from '@/utils/format'

import './index.scss'

export default function SessionsPage() {
  const { t } = useTranslation()
  const [sessions, setSessions] = useState<LoginSession[]>([])
  const [loading, setLoading] = useState(true)
  const [busySid, setBusySid] = useState('')
  const [error, setError] = useState(false)

  usePageTitle('sessions.title')

  const load = useCallback(async () => {
    setLoading(true)
    setError(false)
    try {
      setSessions(await getLoginSessions())
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const revoke = async (session: LoginSession) => {
    const confirmed = await Taro.showModal({ content: t('sessions.revokeConfirm') })
    if (!confirmed.confirm) return
    setBusySid(session.sid)
    try {
      const result = await revokeLoginSession(session.sid)
      await Taro.showToast({ title: t('sessions.revoked'), icon: 'success' })
      if (result.current) {
        clearMiniAuthSession()
        clearConversation()
        await Taro.switchTab({ url: '/pages/profile/index' })
        return
      }
      setSessions((items) => items.filter((item) => item.sid !== session.sid))
    } catch {
      setError(true)
    } finally {
      setBusySid('')
    }
  }

  const revokeOthers = async () => {
    const confirmed = await Taro.showModal({ content: t('sessions.revokeOthersConfirm') })
    if (!confirmed.confirm) return
    setBusySid('others')
    try {
      const result = await revokeOtherLoginSessions()
      setSessions((items) => items.filter((item) => item.current))
      await Taro.showToast({
        title: t('sessions.othersRevoked', { count: result.revoked_count }),
        icon: 'success',
      })
    } catch {
      setError(true)
    } finally {
      setBusySid('')
    }
  }

  return (
    <PageShell title={t('sessions.title')} description={t('sessions.description')}>
      {error ? <Text className='mobile-error'>{t('sessions.loadError')}</Text> : null}
      {loading ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {sessions.length > 1 ? (
        <Button
          className='mobile-button mobile-button--danger'
          disabled={!!busySid}
          onClick={() => void revokeOthers()}
        >
          {t('sessions.revokeOthers')}
        </Button>
      ) : null}
      {!sessions.length && !loading ? <Text className='mobile-empty'>{t('sessions.empty')}</Text> : null}
      <View className='mobile-stack'>
        {sessions.map((session) => (
          <View className='mobile-card settings-list-item' key={session.sid}>
            <View className='mobile-row'>
              <Text className='mobile-card__title'>{session.current ? t('sessions.current') : session.ip || '—'}</Text>
              {session.current ? <Text className='mobile-chip'>{t('sessions.current')}</Text> : null}
            </View>
            <Text className='mobile-card__meta'>{t('sessions.method')}: {session.login_method}</Text>
            <Text className='mobile-card__meta'>{t('sessions.lastActive')}: {formatTime(session.last_active_at)}</Text>
            <Text className='mobile-card__meta'>{t('sessions.expires')}: {formatTime(session.expires_at)}</Text>
            <Text className='mobile-card__meta settings-user-agent'>{session.user_agent || '—'}</Text>
            <Button
              className='mobile-button mobile-button--danger'
              disabled={!!busySid}
              onClick={() => void revoke(session)}
            >
              {t('sessions.revoke')}
            </Button>
          </View>
        ))}
      </View>
    </PageShell>
  )
}
