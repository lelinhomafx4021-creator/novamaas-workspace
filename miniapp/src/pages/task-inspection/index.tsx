import { useEffect, useState } from 'react'

import { Button, Text, View } from '@tarojs/components'
import { useRouter } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getAccountSummary } from '@/api/dashboard'
import { getTaskInformation, getTaskPollHistory, getTaskRequestSnapshots, type TaskPollHistoryEntry, type TaskRequestSnapshots } from '@/api/usage'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import { formatTime } from '@/utils/format'

import './index.scss'

function formatTaskJson(value: unknown) {
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch {
      return value
    }
  }
  return JSON.stringify(value, null, 2) ?? 'null'
}

export default function TaskInspectionPage() {
  const { t } = useTranslation()
  const router = useRouter()
  const taskId = router.params.task_id ?? ''
  const mode = router.params.mode
  const platform = router.params.platform ?? ''
  const [allowed, setAllowed] = useState<boolean | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [snapshots, setSnapshots] = useState<TaskRequestSnapshots | null>(null)
  const [information, setInformation] = useState<unknown>(null)
  const [requestTab, setRequestTab] = useState<'original' | 'upstream'>('original')
  const [entries, setEntries] = useState<TaskPollHistoryEntry[]>([])
  const [nextBeforeId, setNextBeforeId] = useState(0)
  const [loadingMore, setLoadingMore] = useState(false)
  const [expandedResponseId, setExpandedResponseId] = useState<number | null>(null)
  usePageTitle('usage.inspectionTitle')

  useEffect(() => {
    let active = true
    if (!taskId || (mode !== 'request' && mode !== 'response' && mode !== 'information')) {
      setError(true)
      setLoading(false)
      return
    }
    setLoading(true)
    setError(false)
    void (async () => {
      try {
        if (mode === 'information') {
          const nextInformation = await getTaskInformation(taskId, platform)
          if (active) {
            setAllowed(true)
            setInformation(nextInformation)
          }
          return
        }
        const account = await getAccountSummary()
        if (!active) return
        if (account.role < 10) {
          setAllowed(false)
          return
        }
        setAllowed(true)
        if (mode === 'request') {
          const nextSnapshots = await getTaskRequestSnapshots(taskId)
          if (active) setSnapshots(nextSnapshots)
        } else {
          const page = await getTaskPollHistory(taskId)
          if (!active) return
          setEntries(page.items)
          setNextBeforeId(page.next_before_id)
        }
      } catch {
        if (active) setError(true)
      } finally {
        if (active) setLoading(false)
      }
    })()
    return () => { active = false }
  }, [mode, platform, taskId])

  const loadMore = async () => {
    if (!nextBeforeId || loadingMore) return
    setLoadingMore(true)
    setError(false)
    try {
      const page = await getTaskPollHistory(taskId, nextBeforeId)
      setEntries((current) => [...current, ...page.items])
      setNextBeforeId(page.next_before_id)
    } catch {
      setError(true)
    } finally {
      setLoadingMore(false)
    }
  }

  const activeRequestTab = snapshots?.original === undefined ? 'upstream' : requestTab
  const selectedSnapshot = activeRequestTab === 'original'
    ? (snapshots?.original ?? snapshots?.upstream)
    : (snapshots?.upstream ?? snapshots?.original)

  return (
    <PageShell title={t('usage.inspectionTitle')} description={t('usage.inspectionDescription')}>
      <Text className='mobile-muted'>{t('usage.taskId')}: {taskId}</Text>
      {loading ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}
      {allowed === false ? <Text className='mobile-error'>{t('usage.inspectionForbidden')}</Text> : null}
      {error ? <Text className='mobile-error'>{t('usage.inspectionUnavailable')}</Text> : null}
      {!loading && allowed && mode === 'request' && snapshots ? (
        <View className='mobile-card task-inspection'>
          <View className='task-inspection__tabs'>
            {snapshots.original !== undefined ? <Button className={`mobile-button mobile-button--small ${activeRequestTab === 'original' ? '' : 'mobile-button--secondary'}`} onClick={() => setRequestTab('original')}>{t('usage.originalRequest')}</Button> : null}
            {snapshots.upstream !== undefined ? <Button className={`mobile-button mobile-button--small ${activeRequestTab === 'upstream' ? '' : 'mobile-button--secondary'}`} onClick={() => setRequestTab('upstream')}>{t('usage.upstreamRequest')}</Button> : null}
          </View>
          {selectedSnapshot === undefined ? <Text className='mobile-empty'>{t('usage.noRequestSnapshot')}</Text> : <Text className='task-inspection__json' userSelect>{formatTaskJson(selectedSnapshot)}</Text>}
        </View>
      ) : null}
      {!loading && allowed && mode === 'information' ? <View className='mobile-card task-inspection'><Text className='task-inspection__json' userSelect>{formatTaskJson(information)}</Text></View> : null}
      {!loading && allowed && mode === 'response' ? (
        <View className='task-inspection__list'>
          {entries.length === 0 && !error ? <Text className='mobile-empty'>{t('usage.noResponses')}</Text> : null}
          {entries.map((entry) => (
            <View className='mobile-card task-inspection' key={entry.id}>
              <Text className='mobile-card__title'>{entry.status || t('usage.pollingError')}</Text>
              <Text className='mobile-card__meta'>{formatTime(entry.first_seen_at)}{entry.repeat_count > 1 ? ` · ${t('usage.repeatedCount', { count: entry.repeat_count })}` : ''}</Text>
              {entry.repeat_count > 1 ? <Text className='mobile-card__meta'>{t('usage.lastSeen')}: {formatTime(entry.last_seen_at)}</Text> : null}
              {entry.http_status ? <Text className='mobile-card__meta'>HTTP {entry.http_status}</Text> : null}
              {entry.error ? <Text className='mobile-error' userSelect>{entry.error}</Text> : null}
              {entry.response_omitted ? <Text className='mobile-muted'>{t('usage.responseOmitted', { size: entry.response_size ?? 0 })}</Text> : null}
              {entry.response_truncated ? <Text className='mobile-muted'>{t('usage.responseTruncated')}</Text> : null}
              {entry.response !== undefined ? (
                <>
                  <Button className='mobile-button mobile-button--small mobile-button--secondary' onClick={() => setExpandedResponseId((current) => current === entry.id ? null : entry.id)}>{t('usage.viewResponseJson')}</Button>
                  {expandedResponseId === entry.id ? <Text className='task-inspection__json' userSelect>{formatTaskJson(entry.response)}</Text> : null}
                </>
              ) : null}
            </View>
          ))}
          {nextBeforeId > 0 ? <Button className='mobile-button mobile-button--secondary' disabled={loadingMore} onClick={loadMore}>{loadingMore ? t('common.loading') : t('common.loadMore')}</Button> : null}
        </View>
      ) : null}
    </PageShell>
  )
}
