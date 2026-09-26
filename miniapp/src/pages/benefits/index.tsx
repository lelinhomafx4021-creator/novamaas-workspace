import { useCallback, useEffect, useState } from 'react'

import { Button, Input, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import {
  checkin,
  getAccountProfile,
  getAffiliateCode,
  getCheckinStatus,
  transferAffiliateQuota,
  type AccountProfile,
  type CheckinStatus,
} from '@/api/account'
import { getTopUpInfo } from '@/api/wallet'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'

import './index.scss'

function currentMonth() {
  const now = new Date()
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
}

export default function BenefitsPage() {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const [profile, setProfile] = useState<AccountProfile | null>(null)
  const [affiliateCode, setAffiliateCode] = useState('')
  const [checkinStatus, setCheckinStatus] = useState<CheckinStatus | null>(null)
  const [complianceConfirmed, setComplianceConfirmed] = useState(false)
  const [transferAmount, setTransferAmount] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState(false)

  usePageTitle('benefits.title')

  const load = useCallback(async () => {
    setLoading(true)
    setError(false)
    const results = await Promise.allSettled([
      getAccountProfile(),
      getAffiliateCode(),
      getCheckinStatus(currentMonth()),
      getTopUpInfo(),
    ])
    const [profileResult, codeResult, checkinResult, infoResult] = results
    if (profileResult.status === 'fulfilled') setProfile(profileResult.value)
    if (codeResult.status === 'fulfilled') setAffiliateCode(codeResult.value)
    if (checkinResult.status === 'fulfilled') setCheckinStatus(checkinResult.value)
    else setCheckinStatus(null)
    if (infoResult.status === 'fulfilled') {
      setComplianceConfirmed(infoResult.value.payment_compliance_confirmed)
    }
    setError(
      profileResult.status === 'rejected' ||
        codeResult.status === 'rejected' ||
        infoResult.status === 'rejected'
    )
    setLoading(false)
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const doCheckin = async () => {
    if (busy) return
    setBusy(true)
    setError(false)
    try {
      const result = await checkin()
      await Taro.showToast({
        title: t('benefits.checkinSuccess', { amount: formatQuota(result.quota_awarded) }),
        icon: 'success',
      })
      await load()
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  const transfer = async () => {
    const amount = Number(transferAmount)
    if (busy || !Number.isSafeInteger(amount) || amount <= 0) {
      setError(true)
      return
    }
    setBusy(true)
    setError(false)
    try {
      await transferAffiliateQuota(amount)
      setTransferAmount('')
      await Taro.showToast({ title: t('benefits.transferSuccess'), icon: 'success' })
      await load()
    } catch {
      setError(true)
    } finally {
      setBusy(false)
    }
  }

  return (
    <PageShell title={t('benefits.title')} description={t('benefits.description')}>
      {loading ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {error ? <Text className='mobile-error'>{t('benefits.error')}</Text> : null}

      <View className='mobile-card mobile-stack'>
        <Text className='mobile-card__title'>{t('benefits.checkin')}</Text>
        {checkinStatus ? (
          <>
            <View className='settings-summary'>
              <View>
                <Text className='settings-summary__value'>{checkinStatus.stats.checkin_count}</Text>
                <Text className='mobile-card__meta'>{t('benefits.monthCount', { count: checkinStatus.stats.checkin_count })}</Text>
              </View>
              <View>
                <Text className='settings-summary__value'>{formatQuota(checkinStatus.stats.total_quota)}</Text>
                <Text className='mobile-card__meta'>{t('benefits.totalReward', { amount: formatQuota(checkinStatus.stats.total_quota) })}</Text>
              </View>
            </View>
            <Text className='mobile-card__meta'>{t('benefits.totalCount', { count: checkinStatus.stats.total_checkins })}</Text>
            <Button className='mobile-button' disabled={busy || checkinStatus.stats.checked_in_today} onClick={() => void doCheckin()}>
              {checkinStatus.stats.checked_in_today ? t('benefits.checkedToday') : t('benefits.checkinAction')}
            </Button>
          </>
        ) : (
          <Text className='mobile-card__meta'>{t('benefits.checkinUnavailable')}</Text>
        )}
      </View>

      <View className='mobile-card mobile-stack'>
        <Text className='mobile-card__title'>{t('benefits.referral')}</Text>
        <Text className='settings-label'>{t('benefits.referralCode')}</Text>
        <Text className='settings-code'>{affiliateCode || '—'}</Text>
        {affiliateCode ? (
          <Button
            className='mobile-button mobile-button--secondary'
            onClick={() => void Taro.setClipboardData({ data: affiliateCode })}
          >
            {t('common.copy')}
          </Button>
        ) : null}
        <Text className='mobile-card__meta'>{t('benefits.invites', { count: profile?.aff_count ?? 0 })}</Text>
        <Text className='mobile-card__meta'>{t('benefits.pendingReward', { amount: formatQuota(profile?.aff_quota) })}</Text>
        <Text className='mobile-card__meta'>{t('benefits.earnedReward', { amount: formatQuota(profile?.aff_history_quota) })}</Text>
        {!complianceConfirmed ? <Text className='mobile-error'>{t('benefits.complianceLocked')}</Text> : null}
        <Input
          className='mobile-input'
          disabled={!complianceConfirmed}
          type='number'
          value={transferAmount}
          placeholder={t('benefits.transferAmount')}
          onInput={(event) => setTransferAmount(event.detail.value)}
        />
        <Button className='mobile-button' disabled={busy || !complianceConfirmed || !transferAmount} onClick={() => void transfer()}>
          {t('benefits.transfer')}
        </Button>
      </View>
    </PageShell>
  )
}
