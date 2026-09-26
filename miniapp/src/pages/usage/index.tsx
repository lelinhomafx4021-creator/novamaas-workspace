import { useCallback, useEffect, useRef, useState } from 'react'

import { Button, Image, Input, Picker, Text, View } from '@tarojs/components'
import Taro, { useDidHide, useDidShow } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getAccountSummary, getUsageStats, type UsageStats } from '@/api/dashboard'
import {
  getBillingStatements,
  getDrawingTasks,
  getTaskResultDownloadOptions,
  getTasks,
  getUsageLogs,
  type BillingStatement,
  type DrawingTask,
  type MediaFilter,
  type PlatformTask,
  type UsageFilter,
  type UsageLog,
} from '@/api/usage'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'
import {
  getCacheTokens,
  getDrawingDuration,
  getDrawingMedia,
  getEffectiveGroupRatio,
  getTaskDuration,
  getTaskMedia,
  getTaskModel,
  getTaskStatusKey,
  getUsageLogTypeKey,
  isTerminalTaskStatus,
  parseUsageLogOther,
  showsUsageCost,
  type MediaAsset,
} from '@/usage/presentation'
import { formatDuration, formatNumber, formatTime, getRecentRange, getTodayRange } from '@/utils/format'

import './index.scss'

type Tab = 'logs' | 'tasks' | 'statements'
type Range = 'today' | '7d' | '30d'
type Scope = 'personal' | 'platform'
type MediaKind = 'tasks' | 'drawings'

const logTypeValues = [0, 2, 5, 6, 7, 1, 3, 4]

function rangeFilter(range: Range) {
  if (range === 'today') return getTodayRange()
  return getRecentRange(range === '7d' ? 7 : 30)
}

function DetailRow(props: { label: string; value: string | number | undefined }) {
  if (props.value === undefined || props.value === '') return null
  return (
    <View className='usage-detail-row'>
      <Text className='usage-detail-row__label'>{props.label}</Text>
      <Text className='usage-detail-row__value' userSelect>{props.value}</Text>
    </View>
  )
}

function CompactMetric(props: { label: string; value: string | number | undefined }) {
  if (props.value === undefined || props.value === '') return null
  return (
    <View className='usage-compact-metric'>
      <Text className='usage-compact-metric__label'>{props.label}</Text>
      <Text className='usage-compact-metric__value'>{props.value}</Text>
    </View>
  )
}

function UsageLogCard(props: {
  expanded: boolean
  isAdmin: boolean
  log: UsageLog
  onToggle: () => void
}) {
  const { t } = useTranslation()
  const { formatBillingCurrencyFromUSD, formatQuota } = useQuotaDisplay()
  const { log } = props
  const other = parseUsageLogOther(log.other)
  const cache = getCacheTokens(other)
  const modelName = log.model_name || other.upstream_model_name
  const typeLabel = t(getUsageLogTypeKey(log.type))
  const groupRatio = getEffectiveGroupRatio(other)
  const tokensPerSecond = log.use_time > 0 && log.completion_tokens > 0
    ? log.completion_tokens / log.use_time
    : 0
  const hasCompactMetrics = log.prompt_tokens > 0 || log.completion_tokens > 0 ||
    log.use_time > 0 || cache.read > 0

  return (
    <View className={`mobile-card usage-log-card usage-log-card--type-${log.type}`}>
      <View className='mobile-row'>
        <View className='usage-log-card__heading'>
          <Text className='mobile-card__title'>{modelName || typeLabel}</Text>
          <Text className='mobile-chip'>{typeLabel}</Text>
        </View>
        {showsUsageCost(log) ? (
          <Text className='usage-log-card__cost'>{formatQuota(log.quota)}</Text>
        ) : null}
      </View>
      <View className='usage-inline-meta'>
        <Text>{formatTime(log.created_at)}</Text>
        {log.group ? <Text>{log.group}</Text> : null}
        {log.token_name ? <Text>{log.token_name}</Text> : null}
      </View>
      {log.content ? <Text className='usage-log-card__preview'>{log.content}</Text> : null}

      {hasCompactMetrics ? (
        <View className='usage-compact-metrics'>
          {log.prompt_tokens > 0 ? <CompactMetric label={t('usage.inputTokens')} value={formatNumber(log.prompt_tokens)} /> : null}
          {log.completion_tokens > 0 ? <CompactMetric label={t('usage.outputTokens')} value={formatNumber(log.completion_tokens)} /> : null}
          {log.use_time > 0 ? <CompactMetric label={t('usage.duration')} value={formatDuration(log.use_time)} /> : null}
          {cache.read > 0 ? <CompactMetric label={t('usage.cacheRead')} value={formatNumber(cache.read)} /> : null}
        </View>
      ) : null}
      {props.isAdmin && (log.username || log.channel) ? (
        <View className='usage-admin-meta'>
          {log.username ? <Text>{t('usage.user')}: {log.username} (#{log.user_id})</Text> : null}
          {log.channel ? <Text>{t('usage.channel')}: {log.channel_name ? `${log.channel_name} #${log.channel}` : `#${log.channel}`}</Text> : null}
        </View>
      ) : null}

      <Button className='usage-details-toggle' onClick={props.onToggle}>
        {t(props.expanded ? 'common.hideDetails' : 'common.showDetails')} {props.expanded ? '⌃' : '⌄'}
      </Button>
      {props.expanded ? (
        <View className='usage-details'>
          <DetailRow label={t('usage.requestId')} value={log.request_id || '—'} />
          <DetailRow label={t('usage.upstreamRequestId')} value={log.upstream_request_id} />
          {other.upstream_model_name && other.upstream_model_name !== log.model_name ? <DetailRow label={t('usage.upstreamModel')} value={other.upstream_model_name} /> : null}
          {showsUsageCost(log) ? <DetailRow label={t('usage.cost')} value={formatQuota(log.quota)} /> : null}
          <DetailRow label={t('usage.billingSource')} value={other.billing_source ? t(`usage.billingSource.${other.billing_source}`, { defaultValue: other.billing_source }) : undefined} />
          <DetailRow label={t('usage.subscriptionPlan')} value={other.subscription_plan_title} />
          {other.subscription_remain !== undefined ? <DetailRow label={t('usage.subscriptionBalance')} value={`${formatQuota(other.subscription_remain)} / ${formatQuota(other.subscription_total)}`} /> : null}
          <DetailRow label={t('usage.pricingMode')} value={other.matched_tier || other.billing_mode} />
          {other.model_price !== undefined ? <DetailRow label={t('models.modelPrice')} value={formatBillingCurrencyFromUSD(other.model_price)} /> : null}
          {other.model_ratio !== undefined ? <DetailRow label={t('usage.modelRatio')} value={`${other.model_ratio}x`} /> : null}
          {other.completion_ratio !== undefined ? <DetailRow label={t('usage.completionRatio')} value={`${other.completion_ratio}x`} /> : null}
          {groupRatio !== undefined ? <DetailRow label={t('usage.groupRatio')} value={`${groupRatio}x`} /> : null}
          {cache.read > 0 ? <DetailRow label={t('usage.cacheRead')} value={formatNumber(cache.read)} /> : null}
          {cache.write > 0 ? <DetailRow label={t('usage.cacheWrite')} value={formatNumber(cache.write)} /> : null}
          {log.is_stream ? <DetailRow label={t('usage.stream')} value={t('usage.streamed')} /> : null}
          {log.use_time > 0 ? <DetailRow label={t('usage.duration')} value={formatDuration(log.use_time)} /> : null}
          {other.frt ? <DetailRow label={t('usage.firstResponse')} value={formatDuration(other.frt / 1000)} /> : null}
          {tokensPerSecond > 0 ? <DetailRow label={t('usage.throughput')} value={`${tokensPerSecond.toFixed(1)} tok/s`} /> : null}
          <DetailRow label={t('usage.loginMethod')} value={other.login_method} />
          <DetailRow label={t('usage.requestPath')} value={other.request_path} />
          <DetailRow label={t('usage.ip')} value={log.ip} />
          <DetailRow label={t('usage.userAgent')} value={other.user_agent} />
          <DetailRow label={t('usage.content')} value={log.content} />
        </View>
      ) : null}
    </View>
  )
}

function MediaActions(props: {
  assets: MediaAsset[]
  onOpen: (asset: MediaAsset) => void
  onSave: (asset: MediaAsset) => void
  playingUrl: string
}) {
  const { t } = useTranslation()
  if (props.assets.length === 0) return null
  return (
    <View className='usage-media-section'>
      <Text className='mobile-section-title'>{t('usage.mediaResults')}</Text>
      <View className='usage-media-grid'>
        {props.assets.map((asset, index) => (
          <View className='usage-media-item' key={asset.url}>
            {asset.kind === 'image' ? <Image className='usage-media-thumb' mode='aspectFill' src={asset.url} /> : null}
            <Text className='usage-media-item__label'>{t(`usage.${asset.kind}`)} {index + 1}</Text>
            <View className='mobile-actions'>
              <Button className='mobile-button mobile-button--small' onClick={() => props.onOpen(asset)}>
                {asset.kind === 'audio' && props.playingUrl === asset.url ? t('usage.stopAudio') : asset.kind === 'audio' ? t('usage.playAudio') : t('usage.preview')}
              </Button>
              <Button className='mobile-button mobile-button--small mobile-button--secondary' onClick={() => props.onSave(asset)}>{t('usage.download')}</Button>
            </View>
          </View>
        ))}
      </View>
    </View>
  )
}

function TaskCard(props: {
  expanded: boolean
  isAdmin: boolean
  onOpenMedia: (asset: MediaAsset) => void
  onSaveMedia: (asset: MediaAsset) => void
  onToggle: () => void
  playingUrl: string
  task: PlatformTask
}) {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const model = getTaskModel(props.task)
  const assets = getTaskMedia(props.task)
  const failed = ['FAILURE', 'FAILED'].includes(props.task.status.toUpperCase())
  return (
    <View className='mobile-card usage-task-card'>
      <View className='mobile-row'>
        <View className='usage-log-card__heading'>
          <Text className='mobile-card__title'>{model || props.task.action || props.task.platform}</Text>
          <Text className='mobile-card__meta'>{props.task.platform} · {props.task.action || '—'}</Text>
        </View>
        <Text className='mobile-chip'>{t(getTaskStatusKey(props.task.status))}</Text>
      </View>
      <View className='usage-inline-meta'>
        <Text>{formatTime(props.task.submit_time)}</Text>
        {props.task.group ? <Text>{props.task.group}</Text> : null}
      </View>
      <View className='usage-compact-metrics'>
        <CompactMetric label={t('usage.progress')} value={props.task.progress || '—'} />
        {props.task.quota > 0 ? <CompactMetric label={t('usage.cost')} value={formatQuota(props.task.quota)} /> : null}
        <CompactMetric label={t('usage.duration')} value={formatDuration(getTaskDuration(props.task))} />
      </View>
      {props.isAdmin ? (
        <View className='usage-admin-meta'>
          {props.task.username || props.task.user_id ? <Text>{t('usage.user')}: {props.task.username ? `${props.task.username} (#${props.task.user_id})` : `#${props.task.user_id}`}</Text> : null}
          {props.task.channel_id ? <Text>{t('usage.channel')}: #{props.task.channel_id}</Text> : null}
        </View>
      ) : null}
      {failed && props.task.fail_reason ? <Text className='mobile-error'>{props.task.fail_reason}</Text> : null}
      <Button className='usage-details-toggle' onClick={props.onToggle}>
        {t(props.expanded ? 'common.hideDetails' : 'common.showDetails')} {props.expanded ? '⌃' : '⌄'}
      </Button>
      {props.expanded ? (
        <View className='usage-details'>
          <DetailRow label={t('usage.taskId')} value={props.task.task_id} />
          <DetailRow label={t('usage.platform')} value={props.task.platform} />
          <DetailRow label={t('usage.action')} value={props.task.action} />
          <DetailRow label={t('usage.model')} value={model} />
          <DetailRow label={t('usage.taskGroup')} value={props.task.group} />
          <DetailRow label={t('usage.startedAt')} value={formatTime(props.task.start_time)} />
          <DetailRow label={t('usage.finishedAt')} value={formatTime(props.task.finish_time)} />
          <DetailRow label={t('usage.prompt')} value={props.task.properties?.input} />
          <MediaActions assets={assets} onOpen={props.onOpenMedia} onSave={props.onSaveMedia} playingUrl={props.playingUrl} />
        </View>
      ) : null}
    </View>
  )
}

function DrawingCard(props: {
  expanded: boolean
  isAdmin: boolean
  onOpenMedia: (asset: MediaAsset) => void
  onSaveMedia: (asset: MediaAsset) => void
  onToggle: () => void
  playingUrl: string
  task: DrawingTask
}) {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const assets = getDrawingMedia(props.task)
  return (
    <View className='mobile-card usage-task-card'>
      <View className='mobile-row'>
        <View className='usage-log-card__heading'>
          <Text className='mobile-card__title'>{props.task.action || t('usage.drawings')}</Text>
          <Text className='mobile-card__meta'>{props.task.mj_id}</Text>
        </View>
        <Text className='mobile-chip'>{t(getTaskStatusKey(props.task.status))}</Text>
      </View>
      <View className='usage-inline-meta'>
        <Text>{formatTime(Math.floor(props.task.submit_time / 1000))}</Text>
      </View>
      <View className='usage-compact-metrics'>
        <CompactMetric label={t('usage.progress')} value={props.task.progress || '—'} />
        {props.task.quota > 0 ? <CompactMetric label={t('usage.cost')} value={formatQuota(props.task.quota)} /> : null}
        <CompactMetric label={t('usage.duration')} value={formatDuration(getDrawingDuration(props.task))} />
      </View>
      {props.isAdmin ? (
        <View className='usage-admin-meta'>
          {props.task.user_id ? <Text>{t('usage.user')}: #{props.task.user_id}</Text> : null}
          {props.task.channel_id ? <Text>{t('usage.channel')}: #{props.task.channel_id}</Text> : null}
        </View>
      ) : null}
      {props.task.fail_reason ? <Text className='mobile-error'>{props.task.fail_reason}</Text> : null}
      <Button className='usage-details-toggle' onClick={props.onToggle}>
        {t(props.expanded ? 'common.hideDetails' : 'common.showDetails')} {props.expanded ? '⌃' : '⌄'}
      </Button>
      {props.expanded ? (
        <View className='usage-details'>
          <DetailRow label={t('usage.taskId')} value={props.task.mj_id} />
          <DetailRow label={t('usage.action')} value={props.task.action} />
          {props.isAdmin ? <DetailRow label={t('usage.submitCode')} value={props.task.code} /> : null}
          <DetailRow label={t('usage.startedAt')} value={formatTime(Math.floor(props.task.start_time / 1000))} />
          <DetailRow label={t('usage.finishedAt')} value={formatTime(Math.floor(props.task.finish_time / 1000))} />
          <DetailRow label={t('usage.prompt')} value={props.task.prompt} />
          {props.task.prompt_en && props.task.prompt_en !== props.task.prompt ? <DetailRow label={t('usage.translatedPrompt')} value={props.task.prompt_en} /> : null}
          <DetailRow label={t('usage.content')} value={props.task.description} />
          <MediaActions assets={assets} onOpen={props.onOpenMedia} onSave={props.onSaveMedia} playingUrl={props.playingUrl} />
        </View>
      ) : null}
    </View>
  )
}

export default function UsagePage() {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const [signedIn, setSignedIn] = useState(() => !!getMiniAuthSession())
  const [isAdmin, setIsAdmin] = useState(false)
  const [scope, setScope] = useState<Scope>('personal')
  const [tab, setTab] = useState<Tab>('logs')
  const [range, setRange] = useState<Range>('7d')
  const [modelName, setModelName] = useState('')
  const [requestId, setRequestId] = useState('')
  const [username, setUsername] = useState('')
  const [channel, setChannel] = useState('')
  const [logType, setLogType] = useState(0)
  const [logFiltersExpanded, setLogFiltersExpanded] = useState(false)
  const [appliedFilter, setAppliedFilter] = useState({ channel: '', logType: 0, modelName: '', range: '7d' as Range, requestId: '', username: '' })
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [logPage, setLogPage] = useState(1)
  const [logTotal, setLogTotal] = useState(0)
  const [stats, setStats] = useState<UsageStats | null>(null)
  const [mediaKind, setMediaKind] = useState<MediaKind>('tasks')
  const [mediaId, setMediaId] = useState('')
  const [mediaPlatform, setMediaPlatform] = useState('')
  const [mediaStatus, setMediaStatus] = useState('')
  const [mediaChannel, setMediaChannel] = useState('')
  const [mediaFiltersExpanded, setMediaFiltersExpanded] = useState(false)
  const [appliedMediaFilter, setAppliedMediaFilter] = useState({ channel: '', id: '', platform: '', range: '30d' as Range, status: '' })
  const [tasks, setTasks] = useState<PlatformTask[]>([])
  const [drawings, setDrawings] = useState<DrawingTask[]>([])
  const [mediaPage, setMediaPage] = useState(1)
  const [mediaTotal, setMediaTotal] = useState(0)
  const [statements, setStatements] = useState<BillingStatement[]>([])
  const [expandedId, setExpandedId] = useState('')
  const [playingUrl, setPlayingUrl] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(false)
  const visible = useRef(true)
  const pollingTimer = useRef<ReturnType<typeof setTimeout>>()
  const audio = useRef<ReturnType<typeof Taro.createInnerAudioContext> | null>(null)
  usePageTitle('nav.usage')

  const loadRole = useCallback(async () => {
    try {
      const account = await getAccountSummary()
      const allowed = account.role >= 10
      setIsAdmin(allowed)
      if (!allowed) setScope('personal')
    } catch {
      setIsAdmin(false)
      setScope('personal')
    }
  }, [])

  const loadLogs = useCallback(async (page = 1, append = false) => {
    setLoading(true)
    setError(false)
    const timeRange = rangeFilter(appliedFilter.range)
    const isAdminScope = isAdmin && scope === 'platform'
    const filter: UsageFilter = {
      ...timeRange,
      channel: isAdminScope && appliedFilter.channel ? Number(appliedFilter.channel) : undefined,
      isAdmin: isAdminScope,
      logType: appliedFilter.logType || undefined,
      modelName: appliedFilter.modelName,
      page,
      pageSize: 20,
      requestId: appliedFilter.requestId,
      username: isAdminScope ? appliedFilter.username : undefined,
    }
    try {
      const [nextLogs, nextStats] = await Promise.all([
        getUsageLogs(filter),
        getUsageStats(filter),
      ])
      setLogs((items) => (append ? [...items, ...nextLogs.items] : nextLogs.items))
      setLogPage(page)
      setLogTotal(nextLogs.total)
      setStats(nextStats)
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [appliedFilter, isAdmin, scope])

  const loadMedia = useCallback(async (page = 1, append = false, background = false) => {
    if (pollingTimer.current) clearTimeout(pollingTimer.current)
    if (!background) setLoading(true)
    const isAdminScope = isAdmin && scope === 'platform'
    const filter: MediaFilter = {
      ...rangeFilter(appliedMediaFilter.range),
      channelId: isAdminScope && appliedMediaFilter.channel ? Number(appliedMediaFilter.channel) : undefined,
      id: appliedMediaFilter.id,
      isAdmin: isAdminScope,
      page,
      pageSize: 20,
      platform: appliedMediaFilter.platform,
      status: appliedMediaFilter.status,
    }
    try {
      if (mediaKind === 'tasks') {
        const next = await getTasks(filter)
        setTasks((items) => (append ? [...items, ...next.items] : next.items))
        setMediaTotal(next.total)
        const pending = next.items.some((task) => !isTerminalTaskStatus(task.status))
        if (pending && visible.current) pollingTimer.current = setTimeout(() => void loadMedia(1, false, true), 10_000)
      } else {
        const next = await getDrawingTasks(filter)
        setDrawings((items) => (append ? [...items, ...next.items] : next.items))
        setMediaTotal(next.total)
        const pending = next.items.some((task) => !isTerminalTaskStatus(task.status))
        if (pending && visible.current) pollingTimer.current = setTimeout(() => void loadMedia(1, false, true), 10_000)
      }
      setMediaPage(page)
      setError(false)
    } catch {
      if (!background) setError(true)
    } finally {
      if (!background) setLoading(false)
    }
  }, [appliedMediaFilter, isAdmin, mediaKind, scope])

  const loadStatements = useCallback(async () => {
    setLoading(true)
    setError(false)
    try {
      setStatements(await getBillingStatements())
    } catch {
      setError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!signedIn) return
    void loadRole()
  }, [loadRole, signedIn])

  useEffect(() => {
    if (!signedIn) return
    setExpandedId('')
    if (tab === 'logs') void loadLogs()
    else if (tab === 'tasks') void loadMedia()
    else void loadStatements()
    return () => {
      if (pollingTimer.current) clearTimeout(pollingTimer.current)
    }
  }, [loadLogs, loadMedia, loadStatements, signedIn, tab])

  useEffect(() => () => audio.current?.destroy(), [])

  useDidShow(() => {
    visible.current = true
    setSignedIn(!!getMiniAuthSession())
    if (tab === 'tasks') void loadMedia(1, false, true)
  })
  useDidHide(() => {
    visible.current = false
    if (pollingTimer.current) clearTimeout(pollingTimer.current)
    audio.current?.stop()
    setPlayingUrl('')
  })

  const openMedia = async (asset: MediaAsset) => {
    if (asset.kind === 'audio') {
      if (playingUrl === asset.url) {
        audio.current?.stop()
        setPlayingUrl('')
        return
      }
      audio.current?.destroy()
      const player = Taro.createInnerAudioContext()
      player.src = asset.url
      player.onEnded(() => setPlayingUrl(''))
      player.onError(() => {
        setPlayingUrl('')
        setError(true)
      })
      audio.current = player
      setPlayingUrl(asset.url)
      player.play()
      return
    }
    try {
      const result = await Taro.downloadFile(await getTaskResultDownloadOptions(asset.url))
      await Taro.previewMedia({ sources: [{ url: result.tempFilePath, type: asset.kind }] })
    } catch {
      setError(true)
    }
  }

  const saveMedia = async (asset: MediaAsset) => {
    try {
      const result = await Taro.downloadFile(await getTaskResultDownloadOptions(asset.url))
      if (asset.kind === 'video') await Taro.saveVideoToPhotosAlbum({ filePath: result.tempFilePath })
      else if (asset.kind === 'image') await Taro.saveImageToPhotosAlbum({ filePath: result.tempFilePath })
      else await Taro.saveFile({ tempFilePath: result.tempFilePath })
      await Taro.showToast({ title: t('usage.saved'), icon: 'success' })
    } catch {
      setError(true)
    }
  }

  const tabs: Array<{ key: Tab; label: string }> = [
    { key: 'logs', label: t('usage.logs') },
    { key: 'tasks', label: t('usage.tasks') },
    { key: 'statements', label: t('usage.statements') },
  ]
  const logTypeLabels = logTypeValues.map((value) => value === 0 ? t('common.all') : t(getUsageLogTypeKey(value)))
  const selectedLogTypeIndex = Math.max(0, logTypeValues.indexOf(logType))
  const appliedLogFilterLabels = [
    appliedFilter.modelName ? `${t('usage.model')}: ${appliedFilter.modelName}` : '',
    appliedFilter.requestId ? `${t('usage.requestId')}: ${appliedFilter.requestId}` : '',
    isAdmin && scope === 'platform' && appliedFilter.username ? `${t('usage.user')}: ${appliedFilter.username}` : '',
    isAdmin && scope === 'platform' && appliedFilter.channel ? `${t('usage.channel')}: #${appliedFilter.channel}` : '',
  ].filter(Boolean)
  const appliedMediaFilterLabels = [
    appliedMediaFilter.id ? `${t('usage.taskId')}: ${appliedMediaFilter.id}` : '',
    appliedMediaFilter.platform ? `${t('usage.platform')}: ${appliedMediaFilter.platform}` : '',
    appliedMediaFilter.status ? `${t('usage.progress')}: ${appliedMediaFilter.status}` : '',
    isAdmin && scope === 'platform' && appliedMediaFilter.channel ? `${t('usage.channel')}: #${appliedMediaFilter.channel}` : '',
  ].filter(Boolean)

  if (!signedIn) {
    return (
      <PageShell title={t('usage.title')} description={t('usage.description')}>
        <Text className='mobile-empty'>{t('usage.loginRequired')}</Text>
      </PageShell>
    )
  }

  return (
    <PageShell title={t('usage.title')} description={t('usage.description')}>
      {isAdmin ? (
        <View className='usage-scope'>
          <Button className={scope === 'personal' ? 'usage-scope__button usage-scope__button--active' : 'usage-scope__button'} onClick={() => setScope('personal')}>{t('usage.scopePersonal')}</Button>
          <Button className={scope === 'platform' ? 'usage-scope__button usage-scope__button--active' : 'usage-scope__button'} onClick={() => setScope('platform')}>{t('usage.scopePlatform')}</Button>
        </View>
      ) : null}
      {isAdmin && scope === 'platform' ? <Text className='mobile-muted'>{t('usage.adminScopeHint')}</Text> : null}

      <View className='mobile-tabs'>
        {tabs.map((item) => (
          <Button key={item.key} className={`mobile-tabs__item ${tab === item.key ? 'mobile-tabs__item--active' : ''}`} onClick={() => setTab(item.key)}>{item.label}</Button>
        ))}
      </View>
      {error ? <Text className='mobile-error'>{t('usage.error')}</Text> : null}
      {loading ? <Text className='mobile-muted'>{t('common.loading')}</Text> : null}

      {tab === 'logs' ? (
        <>
          <View className='mobile-card usage-filter-card'>
            <View className='usage-ranges'>
              {(['today', '7d', '30d'] as Range[]).map((item) => (
                <Button key={item} className={`mobile-button mobile-button--small ${range === item ? '' : 'mobile-button--secondary'}`} onClick={() => setRange(item)}>{t(`common.${item}`)}</Button>
              ))}
            </View>
            <View className='usage-filter-row'>
              <View className='usage-filter-picker'>
                <Picker mode='selector' range={logTypeLabels} value={selectedLogTypeIndex} onChange={(event) => setLogType(logTypeValues[Number(event.detail.value)] ?? 0)}>
                  <View className='mobile-picker'>{logTypeLabels[selectedLogTypeIndex]}</View>
                </Picker>
              </View>
              <Button className='usage-filter-toggle' onClick={() => setLogFiltersExpanded((value) => !value)}>
                {t(logFiltersExpanded ? 'usage.lessFilters' : 'usage.moreFilters')}
              </Button>
            </View>
            {logFiltersExpanded ? (
              <>
                <Input className='mobile-input' value={modelName} placeholder={t('usage.modelFilter')} onInput={(event) => setModelName(event.detail.value)} />
                <Input className='mobile-input' value={requestId} placeholder={t('usage.requestFilter')} onInput={(event) => setRequestId(event.detail.value)} />
                {isAdmin && scope === 'platform' ? (
                  <>
                    <Input className='mobile-input' value={username} placeholder={t('usage.user')} onInput={(event) => setUsername(event.detail.value)} />
                    <Input className='mobile-input' type='number' value={channel} placeholder={t('usage.channel')} onInput={(event) => setChannel(event.detail.value)} />
                  </>
                ) : null}
              </>
            ) : null}
            {!logFiltersExpanded && appliedLogFilterLabels.length > 0 ? (
              <Text className='usage-active-filters'>{appliedLogFilterLabels.join(' · ')}</Text>
            ) : null}
            <Button className='mobile-button mobile-button--small usage-apply-filter' onClick={() => {
              setAppliedFilter({ channel: channel.trim(), logType, modelName: modelName.trim(), range, requestId: requestId.trim(), username: username.trim() })
              setLogFiltersExpanded(false)
            }}>{t('usage.apply')}</Button>
          </View>
          {stats ? (
            <>
              <View className='mobile-card usage-stats'>
                <View className='usage-stat'><Text className='usage-stat__value'>{formatQuota(stats.quota)}</Text><Text className='usage-stat__label'>{t('usage.quota')}</Text></View>
                <View className='usage-stat'><Text className='usage-stat__value'>{formatNumber(stats.requests)}</Text><Text className='usage-stat__label'>{t('usage.requests')}</Text></View>
                <View className='usage-stat'><Text className='usage-stat__value'>{formatNumber(stats.rpm)} / {formatNumber(stats.tpm)}</Text><Text className='usage-stat__label'>{t('usage.liveRate')} · RPM / TPM</Text></View>
              </View>
              {stats.revenue_quota !== undefined ? (
                <View className='mobile-card'>
                  <Text className='mobile-card__title'>{t('usage.financialSummary')}</Text>
                  <View className='usage-summary-grid'>
                    <DetailRow label={t('usage.turnover')} value={formatQuota(stats.revenue_quota)} />
                    <DetailRow label={t('usage.costAmount')} value={formatQuota(stats.cost_quota)} />
                    <DetailRow label={t('usage.profitAmount')} value={formatQuota(stats.profit_quota)} />
                    <DetailRow label={t('usage.records')} value={formatNumber(stats.records)} />
                  </View>
                </View>
              ) : null}
            </>
          ) : null}
          {logs.length === 0 && !loading ? <Text className='mobile-empty'>{t('usage.noLogs')}</Text> : null}
          <View className='mobile-stack usage-list'>
            {logs.map((log) => (
              <UsageLogCard key={`${log.id}-${log.request_id}`} log={log} isAdmin={isAdmin && scope === 'platform'} expanded={expandedId === `log-${log.id}-${log.request_id}`} onToggle={() => setExpandedId((value) => value === `log-${log.id}-${log.request_id}` ? '' : `log-${log.id}-${log.request_id}`)} />
            ))}
          </View>
          {logs.length < logTotal ? <Button className='mobile-button mobile-button--secondary' onClick={() => loadLogs(logPage + 1, true)}>{t('common.loadMore')}</Button> : null}
        </>
      ) : null}

      {tab === 'tasks' ? (
        <>
          <View className='usage-scope'>
            <Button className={mediaKind === 'tasks' ? 'usage-scope__button usage-scope__button--active' : 'usage-scope__button'} onClick={() => setMediaKind('tasks')}>{t('usage.standardTasks')}</Button>
            <Button className={mediaKind === 'drawings' ? 'usage-scope__button usage-scope__button--active' : 'usage-scope__button'} onClick={() => setMediaKind('drawings')}>{t('usage.drawings')}</Button>
          </View>
          <View className='mobile-card usage-filter-card'>
            <View className='usage-ranges'>
              {(['today', '7d', '30d'] as Range[]).map((item) => (
                <Button key={item} className={`mobile-button mobile-button--small ${appliedMediaFilter.range === item ? '' : 'mobile-button--secondary'}`} onClick={() => setAppliedMediaFilter((filter) => ({ ...filter, range: item }))}>{t(`common.${item}`)}</Button>
              ))}
            </View>
            <View className='usage-filter-row'>
              <Text className='usage-filter-summary'>{mediaKind === 'tasks' ? t('usage.standardTasks') : t('usage.drawings')}</Text>
              <Button className='usage-filter-toggle' onClick={() => setMediaFiltersExpanded((value) => !value)}>
                {t(mediaFiltersExpanded ? 'usage.lessFilters' : 'usage.moreFilters')}
              </Button>
            </View>
            {mediaFiltersExpanded ? (
              <>
                <Input className='mobile-input' value={mediaId} placeholder={t('usage.taskIdFilter')} onInput={(event) => setMediaId(event.detail.value)} />
                {mediaKind === 'tasks' ? (
                  <>
                    <Input className='mobile-input' value={mediaPlatform} placeholder={t('usage.platformFilter')} onInput={(event) => setMediaPlatform(event.detail.value)} />
                    <Input className='mobile-input' value={mediaStatus} placeholder={t('usage.statusFilter')} onInput={(event) => setMediaStatus(event.detail.value)} />
                  </>
                ) : null}
                {isAdmin && scope === 'platform' ? <Input className='mobile-input' type='number' value={mediaChannel} placeholder={t('usage.channel')} onInput={(event) => setMediaChannel(event.detail.value)} /> : null}
              </>
            ) : null}
            {!mediaFiltersExpanded && appliedMediaFilterLabels.length > 0 ? (
              <Text className='usage-active-filters'>{appliedMediaFilterLabels.join(' · ')}</Text>
            ) : null}
            <Button className='mobile-button mobile-button--small usage-apply-filter' onClick={() => {
              setAppliedMediaFilter((filter) => ({ ...filter, channel: mediaChannel.trim(), id: mediaId.trim(), platform: mediaPlatform.trim(), status: mediaStatus.trim() }))
              setMediaFiltersExpanded(false)
            }}>{t('usage.apply')}</Button>
          </View>
          {mediaKind === 'tasks' && tasks.length === 0 && !loading ? <Text className='mobile-empty'>{t('usage.noTasks')}</Text> : null}
          {mediaKind === 'drawings' && drawings.length === 0 && !loading ? <Text className='mobile-empty'>{t('usage.noTasks')}</Text> : null}
          <View className='mobile-stack usage-list'>
            {mediaKind === 'tasks' ? tasks.map((task) => (
              <TaskCard key={task.id} task={task} isAdmin={isAdmin && scope === 'platform'} expanded={expandedId === `task-${task.id}`} onToggle={() => setExpandedId((value) => value === `task-${task.id}` ? '' : `task-${task.id}`)} onOpenMedia={openMedia} onSaveMedia={saveMedia} playingUrl={playingUrl} />
            )) : drawings.map((task) => (
              <DrawingCard key={task.id} task={task} isAdmin={isAdmin && scope === 'platform'} expanded={expandedId === `drawing-${task.id}`} onToggle={() => setExpandedId((value) => value === `drawing-${task.id}` ? '' : `drawing-${task.id}`)} onOpenMedia={openMedia} onSaveMedia={saveMedia} playingUrl={playingUrl} />
            ))}
          </View>
          {(mediaKind === 'tasks' ? tasks.length : drawings.length) < mediaTotal ? <Button className='mobile-button mobile-button--secondary' onClick={() => loadMedia(mediaPage + 1, true)}>{t('common.loadMore')}</Button> : null}
        </>
      ) : null}

      {tab === 'statements' ? (
        <>
          {statements.length === 0 && !loading ? <Text className='mobile-empty'>{t('usage.noStatements')}</Text> : null}
          <View className='mobile-stack'>
            {statements.map((statement) => (
              <View className='mobile-card' key={statement.id} onClick={() => Taro.navigateTo({ url: `/pages/billing-detail/index?id=${encodeURIComponent(statement.id)}` })}>
                <View className='mobile-row'><Text className='mobile-card__title'>{statement.month}</Text><Text className='mobile-chip'>{statement.status}</Text></View>
                <Text className='mobile-card__meta'>{t('usage.revision', { revision: statement.revision })}</Text>
                <Text className='mobile-card__meta'>{formatTime(statement.start_at)} — {formatTime(statement.end_at)}</Text>
              </View>
            ))}
          </View>
        </>
      ) : null}
    </PageShell>
  )
}
