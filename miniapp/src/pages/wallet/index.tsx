import { useCallback, useEffect, useMemo, useState } from 'react'

import { Button, Input, Picker, Text, View } from '@tarojs/components'
import Taro, { useDidShow, usePullDownRefresh } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getAccountProfile, type AccountProfile } from '@/api/account'
import {
  buySubscriptionWithBalance,
  getSelfSubscriptions,
  getSubscriptionPlans,
  getTopUpInfo,
  getTopUps,
  redeemCode,
  updateBillingPreference,
  type SelfSubscriptionData,
  type SubscriptionPlan,
  type TopUpInfo,
  type TopUpRecord,
} from '@/api/wallet'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'
import { formatTime } from '@/utils/format'

import './index.scss'

const billingPreferences = [
  'subscription_first',
  'wallet_first',
  'subscription_only',
  'wallet_only',
] as const

export default function WalletPage() {
  const { t } = useTranslation()
  const {
    formatBillingCurrencyFromUSD,
    formatCurrencyFromUSD,
    formatLocalCurrencyAmount,
    formatQuota,
  } = useQuotaDisplay()
  const [signedIn, setSignedIn] = useState(() => !!getMiniAuthSession())
  const [account, setAccount] = useState<AccountProfile | null>(null)
  const [info, setInfo] = useState<TopUpInfo | null>(null)
  const [topups, setTopups] = useState<TopUpRecord[]>([])
  const [topupTotal, setTopupTotal] = useState(0)
  const [topupPage, setTopupPage] = useState(1)
  const [plans, setPlans] = useState<SubscriptionPlan[]>([])
  const [subscriptions, setSubscriptions] = useState<SelfSubscriptionData | null>(null)
  const [redemptionCode, setRedemptionCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)

  usePageTitle('wallet.title')

  const load = useCallback(async () => {
    if (!getMiniAuthSession()) return
    setLoading(true)
    const results = await Promise.allSettled([
      getAccountProfile(),
      getTopUpInfo(),
      getTopUps(),
      getSubscriptionPlans(),
      getSelfSubscriptions(),
    ])
    const [accountResult, infoResult, topupsResult, plansResult, subscriptionsResult] = results
    if (accountResult.status === 'fulfilled') setAccount(accountResult.value)
    if (infoResult.status === 'fulfilled') setInfo(infoResult.value)
    if (topupsResult.status === 'fulfilled') {
      setTopups(topupsResult.value.items)
      setTopupTotal(topupsResult.value.total)
      setTopupPage(1)
    }
    if (plansResult.status === 'fulfilled') setPlans(plansResult.value)
    if (subscriptionsResult.status === 'fulfilled') setSubscriptions(subscriptionsResult.value)
    setError(results.some((result) => result.status === 'rejected'))
    setLoading(false)
  }, [])

  useEffect(() => {
    if (signedIn) void load()
  }, [load, signedIn])

  useDidShow(() => setSignedIn(!!getMiniAuthSession()))
  usePullDownRefresh(() => void load().finally(() => Taro.stopPullDownRefresh()))

  const planTitles = useMemo(
    () => new Map(plans.map((plan) => [plan.id, plan.title])),
    [plans]
  )
  const preference = subscriptions?.billing_preference || 'subscription_first'
  const preferenceIndex = Math.max(
    0,
    billingPreferences.indexOf(preference as (typeof billingPreferences)[number])
  )
  const preferenceLabels = billingPreferences.map((item) =>
    t(`wallet.preference.${item}`)
  )

  const redeem = async () => {
    const key = redemptionCode.trim()
    if (!key || busy) return
    setBusy(true)
    try {
      await redeemCode(key)
      setRedemptionCode('')
      await Taro.showToast({ title: t('wallet.redeemed'), icon: 'success' })
      await load()
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const buy = async (plan: SubscriptionPlan) => {
    if (busy || !plan.allow_balance_pay) return
    const confirmed = await Taro.showModal({
      content: t('wallet.buyConfirm', {
        plan: plan.title,
        price: formatBillingCurrencyFromUSD(plan.price_amount, {
          digitsLarge: 2,
          digitsSmall: 2,
        }),
      }),
    })
    if (!confirmed.confirm) return
    setBusy(true)
    try {
      await buySubscriptionWithBalance(plan.id)
      await Taro.showToast({ title: t('wallet.bought'), icon: 'success' })
      await load()
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const savePreference = async (index: number) => {
    const nextPreference = billingPreferences[index]
    if (!nextPreference || busy) return
    setBusy(true)
    try {
      const result = await updateBillingPreference(nextPreference)
      setSubscriptions((current) =>
        current
          ? { ...current, billing_preference: result.billing_preference }
          : current
      )
      await Taro.showToast({ title: t('wallet.preferenceSaved'), icon: 'success' })
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const loadMore = async () => {
    if (loading || topups.length >= topupTotal) return
    setLoading(true)
    try {
      const nextPage = topupPage + 1
      const result = await getTopUps(nextPage)
      setTopups((items) => [...items, ...result.items])
      setTopupPage(nextPage)
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }

  if (!signedIn) {
    return (
      <PageShell title={t('wallet.title')} description={t('wallet.description')}>
        <Text className='mobile-empty'>{t('wallet.loginRequired')}</Text>
      </PageShell>
    )
  }

  return (
    <PageShell title={t('wallet.title')} description={t('wallet.description')}>
      {error ? <Text className='mobile-error'>{t('wallet.loadError')}</Text> : null}
      {loading && !account ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}

      <View className='settings-summary'>
        <View className='mobile-card'>
          <Text className='settings-summary__value'>{formatQuota(account?.quota)}</Text>
          <Text className='mobile-card__meta'>{t('wallet.balance')}</Text>
        </View>
        <View className='mobile-card'>
          <Text className='settings-summary__value'>{subscriptions?.subscriptions.length ?? 0}</Text>
          <Text className='mobile-card__meta'>{t('wallet.subscriptions')}</Text>
        </View>
      </View>

      <Text className='mobile-error'>{t('wallet.nativePaymentBlocked')}</Text>
      {info && !info.payment_compliance_confirmed ? (
        <Text className='mobile-error'>{t('wallet.complianceLocked')}</Text>
      ) : null}

      {info?.enable_redemption ? (
        <View className='mobile-card mobile-stack'>
          <Text className='mobile-card__title'>{t('wallet.redemption')}</Text>
          <Input
            className='mobile-input'
            maxlength={128}
            value={redemptionCode}
            placeholder={t('wallet.redemptionCode')}
            onInput={(event) => setRedemptionCode(event.detail.value)}
          />
          <Button className='mobile-button' disabled={busy || !redemptionCode.trim()} onClick={redeem}>
            {t('wallet.redeem')}
          </Button>
        </View>
      ) : null}

      <Text className='mobile-section-title'>{t('wallet.subscriptions')}</Text>
      <View className='mobile-card mobile-stack'>
        <Text className='settings-label'>{t('wallet.preference')}</Text>
        <Picker
          mode='selector'
          range={preferenceLabels}
          value={preferenceIndex}
          onChange={(event) => void savePreference(Number(event.detail.value))}
        >
          <View className='mobile-picker'>{preferenceLabels[preferenceIndex]}</View>
        </Picker>
      </View>

      {subscriptions?.all_subscriptions.length ? (
        <View className='mobile-stack'>
          {subscriptions.all_subscriptions.map(({ subscription }) => {
            const unlimited = subscription.amount_total <= 0
            const remaining = Math.max(0, subscription.amount_total - subscription.amount_used)
            const usedPercent = unlimited
              ? 0
              : Math.min(100, Math.round((subscription.amount_used / subscription.amount_total) * 100))
            return (
              <View className='mobile-card settings-list-item' key={subscription.id}>
                <View className='mobile-row'>
                  <Text className='mobile-card__title'>
                    {planTitles.get(subscription.plan_id) || `#${subscription.plan_id}`}
                  </Text>
                  <Text className='mobile-chip'>{subscription.status}</Text>
                </View>
                <Text className='mobile-card__meta'>
                  {t('wallet.activeUntil', { time: formatTime(subscription.end_time) })}
                </Text>
                <Text className='mobile-card__meta'>
                  {unlimited ? t('wallet.unlimited') : t('wallet.remaining', { amount: formatQuota(remaining) })}
                </Text>
                {!unlimited ? (
                  <View className='settings-progress'>
                    <View className='settings-progress__bar' style={{ width: `${usedPercent}%` }} />
                  </View>
                ) : null}
              </View>
            )
          })}
        </View>
      ) : (
        <Text className='mobile-empty'>{t('wallet.noSubscriptions')}</Text>
      )}

      <View className='mobile-stack'>
        {plans.map((plan) => (
          <View className='mobile-card settings-list-item' key={plan.id}>
            <View className='mobile-row'>
              <View>
                <Text className='mobile-card__title'>{plan.title}</Text>
                <Text className='mobile-card__meta'>{plan.subtitle}</Text>
              </View>
              <Text className='wallet-plan__price'>
                {formatBillingCurrencyFromUSD(plan.price_amount, { digitsLarge: 2, digitsSmall: 2 })}
              </Text>
            </View>
            <Text className='mobile-card__meta'>
              {plan.duration_value} {t(`wallet.duration.${plan.duration_unit}`, { defaultValue: plan.duration_unit })}
            </Text>
            <Text className='mobile-card__meta'>
              {plan.total_amount > 0 ? t('wallet.amount') + ': ' + formatQuota(plan.total_amount) : t('wallet.unlimited')}
            </Text>
            <Button
              className='mobile-button mobile-button--secondary'
              disabled={busy || !plan.allow_balance_pay}
              onClick={() => void buy(plan)}
            >
              {plan.allow_balance_pay ? t('wallet.buy') : t('wallet.buyUnavailable')}
            </Button>
          </View>
        ))}
      </View>
      {!plans.length && !loading ? <Text className='mobile-empty'>{t('wallet.noPlans')}</Text> : null}

      <Text className='mobile-section-title'>{t('wallet.history')}</Text>
      {topups.length ? (
        <View className='mobile-stack'>
          {topups.map((record) => (
            <View className='mobile-card settings-list-item' key={record.id}>
              <View className='mobile-row'>
                <Text className='mobile-card__title'>{record.payment_method || record.payment_provider}</Text>
                <Text className='mobile-chip'>
                  {t(`wallet.status.${record.status}`, { defaultValue: record.status })}
                </Text>
              </View>
              <Text className='mobile-card__meta'>{formatTime(record.complete_time || record.create_time)}</Text>
              <Text className='mobile-card__meta'>
                {t('wallet.amount')}: {formatCurrencyFromUSD(record.amount, { digitsLarge: 2, digitsSmall: 2 })}
              </Text>
              <Text className='mobile-card__meta'>
                {t('wallet.paid')}: {formatLocalCurrencyAmount(record.money, { digitsLarge: 2, digitsSmall: 2 })}
              </Text>
            </View>
          ))}
        </View>
      ) : !loading ? (
        <Text className='mobile-empty'>{t('wallet.noHistory')}</Text>
      ) : null}
      {topups.length < topupTotal ? (
        <Button className='mobile-button mobile-button--secondary' onClick={() => void loadMore()}>
          {t('common.loadMore')}
        </Button>
      ) : null}
    </PageShell>
  )
}
