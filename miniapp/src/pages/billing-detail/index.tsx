import { useEffect, useState } from 'react'

import { Button, Text, View } from '@tarojs/components'
import Taro, { useRouter } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import {
  getBillingArtifactDownloadOptions,
  getBillingStatement,
  type BillingStatementDetail,
} from '@/api/usage'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import { formatNumber, formatTime } from '@/utils/format'

export default function BillingDetailPage() {
  const { t } = useTranslation()
  const router = useRouter()
  const statementId = decodeURIComponent(router.params.id ?? '')
  const [detail, setDetail] = useState<BillingStatementDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  usePageTitle('usage.statementDetail')

  useEffect(() => {
    if (!statementId) {
      setError(true)
      setLoading(false)
      return
    }
    getBillingStatement(statementId)
      .then(setDetail)
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }, [statementId])

  const download = async (kind: string, ordinal: number) => {
    try {
      const options = await getBillingArtifactDownloadOptions(statementId, kind, ordinal)
      const result = await Taro.downloadFile(options)
      await Taro.openDocument({ filePath: result.tempFilePath, showMenu: true })
    } catch {
      setError(true)
    }
  }

  return (
    <PageShell title={detail?.statement.month || t('usage.statementDetail')} description={t('usage.statementDescription')}>
      {loading ? <Text className='mobile-empty'>{t('common.loading')}</Text> : null}
      {error ? <Text className='mobile-error'>{t('usage.error')}</Text> : null}
      {detail ? (
        <>
          <View className='mobile-card'>
            <View className='mobile-row'><Text className='mobile-card__title'>{detail.statement.month}</Text><Text className='mobile-chip'>{detail.statement.status}</Text></View>
            <Text className='mobile-card__meta'>{formatTime(detail.statement.start_at)} — {formatTime(detail.statement.end_at)}</Text>
            <Text className='mobile-card__meta'>{t('usage.dueAt')}: {formatTime(detail.statement.due_at)}</Text>
            {detail.source_warning ? <Text className='mobile-error'>{t('usage.sourceWarning')}</Text> : null}
          </View>
          <Text className='mobile-section-title'>{t('usage.events')}</Text>
          {detail.events.length === 0 ? <Text className='mobile-empty'>{t('usage.noEvents')}</Text> : null}
          {detail.events.map((event, index) => (
            <View className='mobile-card' key={event.id ?? index}>
              <Text className='mobile-card__title'>{event.action || t('usage.event')}</Text>
              <Text className='mobile-card__meta'>{event.note || ''}</Text>
              {event.actor_username ? <Text className='mobile-card__meta'>{event.actor_username}</Text> : null}
              <Text className='mobile-card__meta'>{formatTime(event.created_at)}</Text>
            </View>
          ))}
          <Text className='mobile-section-title'>{t('usage.artifacts')}</Text>
          {detail.artifacts.length === 0 ? <Text className='mobile-empty'>{t('usage.noArtifacts')}</Text> : null}
          {detail.artifacts.map((artifact) => (
            <View className='mobile-card' key={artifact.id}>
              <Text className='mobile-card__title'>{artifact.kind} #{artifact.ordinal + 1}</Text>
              <Text className='mobile-card__meta'>
                {formatNumber(artifact.size)} {t('usage.bytes')} · {formatNumber(artifact.rows)} {t('usage.rows')}
              </Text>
              <Button className='mobile-button mobile-button--secondary' onClick={() => download(artifact.kind, artifact.ordinal)}>{t('usage.openArtifact')}</Button>
            </View>
          ))}
        </>
      ) : null}
    </PageShell>
  )
}
