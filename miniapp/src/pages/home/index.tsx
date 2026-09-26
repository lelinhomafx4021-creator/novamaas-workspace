import { useCallback, useEffect, useRef, useState } from 'react'

import { Button, Text, View } from '@tarojs/components'
import Taro, { useDidShow, usePullDownRefresh } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import {
  getAccountSummary,
  getPlatformNotice,
  getSubscriptionSummary,
  getUsageStats,
  type AccountSummary,
  type SubscriptionSummary,
  type UsageStats,
} from '@/api/dashboard'
import { getConfiguredApiBaseUrl } from '@/api/request'
import { getPlatformStatus, type PlatformStatus } from '@/api/status'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'
import { formatNumber, formatTime, getTodayRange } from '@/utils/format'

import './index.scss'

type Loadable<T> =
  | { phase: 'idle' | 'loading' }
  | { phase: 'error' }
  | { phase: 'ready'; data: T }

type ConnectionState =
  | { phase: 'loading' | 'not-configured' | 'error' }
  | { phase: 'ready'; data: PlatformStatus }

export default function HomePage() {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const [signedIn, setSignedIn] = useState(() => !!getMiniAuthSession())
  const [connection, setConnection] = useState<ConnectionState>({ phase: 'loading' })
  const [account, setAccount] = useState<Loadable<AccountSummary>>({ phase: 'idle' })
  const [subscription, setSubscription] = useState<Loadable<SubscriptionSummary>>({ phase: 'idle' })
  const [usage, setUsage] = useState<Loadable<UsageStats>>({ phase: 'idle' })
  const [notice, setNotice] = useState<Loadable<string>>({ phase: 'loading' })
  const [reloadCount, setReloadCount] = useState(0)
  const shownOnce = useRef(false)

  usePageTitle('nav.home')

  const load = useCallback(async () => {
    let apiBaseUrl = ''
    try {
      apiBaseUrl = getConfiguredApiBaseUrl()
    } catch {
      setConnection({ phase: 'error' })
      return
    }
    if (!apiBaseUrl) {
      setConnection({ phase: 'not-configured' })
      return
    }

    setConnection({ phase: 'loading' })
    setNotice({ phase: 'loading' })
    if (signedIn) {
      setAccount({ phase: 'loading' })
      setSubscription({ phase: 'loading' })
      setUsage({ phase: 'loading' })
    }

    const today = getTodayRange()
    const requests = [
      getPlatformStatus(),
      getPlatformNotice(),
      ...(signedIn
        ? [
            getAccountSummary(),
            getSubscriptionSummary(),
            getUsageStats({ startTimestamp: today.startTimestamp, endTimestamp: today.endTimestamp }),
          ]
        : []),
    ]
    const results = await Promise.allSettled(requests)
    const statusResult = results[0]
    setConnection(
      statusResult.status === 'fulfilled'
        ? { phase: 'ready', data: statusResult.value as PlatformStatus }
        : { phase: 'error' }
    )
    const noticeResult = results[1]
    setNotice(
      noticeResult.status === 'fulfilled'
        ? { phase: 'ready', data: noticeResult.value as string }
        : { phase: 'error' }
    )
    if (signedIn) {
      const accountResult = results[2]
      const subscriptionResult = results[3]
      const usageResult = results[4]
      setAccount(
        accountResult.status === 'fulfilled'
          ? { phase: 'ready', data: accountResult.value as AccountSummary }
          : { phase: 'error' }
      )
      setSubscription(
        subscriptionResult.status === 'fulfilled'
          ? { phase: 'ready', data: subscriptionResult.value as SubscriptionSummary }
          : { phase: 'error' }
      )
      setUsage(
        usageResult.status === 'fulfilled'
          ? { phase: 'ready', data: usageResult.value as UsageStats }
          : { phase: 'error' }
      )
    }
  }, [signedIn])

  useEffect(() => {
    void load()
  }, [load, reloadCount])

  useDidShow(() => {
    setSignedIn(!!getMiniAuthSession())
    if (shownOnce.current) setReloadCount((count) => count + 1)
    shownOnce.current = true
  })

  usePullDownRefresh(() => {
    void load().finally(() => Taro.stopPullDownRefresh())
  })

  let statusClassName = 'home-status__signal home-status__signal--pending'
  let statusText = t('home.statusLoading')
  if (connection.phase === 'not-configured') {
    statusClassName = 'home-status__signal home-status__signal--warning'
    statusText = t('home.statusNotConfigured')
  } else if (connection.phase === 'error') {
    statusClassName = 'home-status__signal home-status__signal--error'
    statusText = t('home.statusUnavailable')
  } else if (connection.phase === 'ready') {
    statusClassName = connection.data.setup
      ? 'home-status__signal home-status__signal--ready'
      : 'home-status__signal home-status__signal--warning'
    statusText = connection.data.setup
      ? t('home.statusConnected', {
          systemName: connection.data.system_name || t('home.defaultSystemName'),
        })
      : t('home.statusSetupRequired')
  }

  const moduleError = <Text className='mobile-error'>{t('home.moduleError')}</Text>
  const activeSubscriptions = subscription.phase === 'ready'
    ? subscription.data.subscriptions.map((item) => item.subscription)
    : []
  const hasUnlimitedSubscription = activeSubscriptions.some((item) => item.amount_total <= 0)
  const subscriptionRemaining = activeSubscriptions.reduce(
    (total, item) => total + Math.max(0, item.amount_total - item.amount_used),
    0
  )
  const subscriptionExpiries = activeSubscriptions
    .map((item) => item.end_time)
    .filter((time) => time > 0)
  const nextSubscriptionExpiry = subscriptionExpiries.length
    ? Math.min(...subscriptionExpiries)
    : 0

  return (
    <PageShell eyebrow={t('home.eyebrow')} title={t('home.title')} description={t('home.description')}>
      <View className='home-status'>
        <View className='home-status__heading'>
          <Text className='home-status__title'>{t('home.statusTitle')}</Text>
          <View className={statusClassName} />
        </View>
        <Text className='home-status__message'>{statusText}</Text>
        {connection.phase === 'error' ? (
          <Button className='home-status__button' onClick={() => setReloadCount((count) => count + 1)}>
            {t('common.retry')}
          </Button>
        ) : null}
      </View>

      {!signedIn ? (
        <View className='mobile-card'>
          <Text className='mobile-card__title'>{t('home.signedOut')}</Text>
          <Text className='mobile-card__meta'>{t('home.loginHint')}</Text>
        </View>
      ) : (
        <>
          <Text className='mobile-section-title'>{t('home.accountSummary')}</Text>
          {account.phase === 'idle' || account.phase === 'loading' ? (
            <Text className='mobile-muted'>{t('common.loading')}</Text>
          ) : null}
          {account.phase === 'error' ? moduleError : null}
          {account.phase === 'ready' ? (
            <>
              <View className='home-account-heading'>
                <Text className='mobile-card__title'>{account.data.display_name || account.data.username}</Text>
                <Text className='mobile-card__meta'>{t('home.accountGroup', { group: account.data.group })}</Text>
              </View>
              <View className='home-metrics'>
                <View className='mobile-card home-metric'>
                  <Text className='home-metric__value'>{formatQuota(account.data.quota)}</Text>
                  <Text className='mobile-card__meta'>{t('home.quota')}</Text>
                </View>
                <View className='mobile-card home-metric'>
                  <Text className='home-metric__value'>{usage.phase === 'ready' ? formatQuota(usage.data.quota) : '—'}</Text>
                  <Text className='mobile-card__meta'>{t('home.todayUsage')}</Text>
                </View>
                <View className='mobile-card home-metric'>
                  <Text className='home-metric__value'>{usage.phase === 'ready' ? formatNumber(usage.data.requests) : '—'}</Text>
                  <Text className='mobile-card__meta'>{t('home.todayRequests')}</Text>
                </View>
                <View className='mobile-card home-metric'>
                  <Text className='home-metric__value'>{subscription.phase === 'ready' ? formatNumber(subscription.data.subscriptions.length) : '—'}</Text>
                  <Text className='mobile-card__meta'>{t('home.activePlans')}</Text>
                </View>
              </View>
            </>
          ) : null}

          {subscription.phase === 'idle' || subscription.phase === 'loading' ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
          {subscription.phase === 'error' ? moduleError : null}
          {subscription.phase === 'ready' ? (
            <View className='mobile-card home-subscription'>
              <View className='mobile-row'>
                <Text className='mobile-card__title'>{t('home.subscription')}</Text>
                <Text className='mobile-chip'>{activeSubscriptions.length ? t('home.activeSubscriptions', { count: activeSubscriptions.length }) : t('home.noSubscription')}</Text>
              </View>
              {activeSubscriptions.length ? (
                <>
                  <Text className='home-subscription__value'>{hasUnlimitedSubscription ? t('common.unlimited') : formatQuota(subscriptionRemaining)}</Text>
                  <Text className='mobile-card__meta'>{t('home.subscriptionRemaining')}</Text>
                  {nextSubscriptionExpiry ? <Text className='mobile-card__meta'>{t('home.nextExpiry', { time: formatTime(nextSubscriptionExpiry) })}</Text> : null}
                </>
              ) : null}
              <Button className='mobile-button mobile-button--secondary' onClick={() => Taro.navigateTo({ url: '/pages/wallet/index' })}>{t('home.openWallet')}</Button>
            </View>
          ) : null}
          {usage.phase === 'error' ? moduleError : null}
        </>
      )}

      {notice.phase === 'ready' && notice.data ? (
        <View className='mobile-card'>
          <Text className='mobile-card__title'>{t('home.notice')}</Text>
          <Text className='mobile-card__meta'>{notice.data}</Text>
        </View>
      ) : null}
    </PageShell>
  )
}
