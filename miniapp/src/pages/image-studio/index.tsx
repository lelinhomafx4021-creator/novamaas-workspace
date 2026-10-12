import { Button, Picker, ScrollView, Text, Textarea, View } from '@tarojs/components'
import Taro, { useDidHide, useDidShow } from '@tarojs/taro'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getAccountSummary } from '@/api/dashboard'
import {
  assistStudioPrompt,
  createStudioRun,
  getStudioKeys,
  getStudioModels,
  getStudioRun,
  getStudioSettings,
  listStudioRuns,
  type StudioRun,
  type StudioSettings,
} from '@/api/image-studio'
import { ApiRequestError } from '@/api/request'
import type { ApiToken } from '@/api/tokens'
import { getMiniAuthSession } from '@/auth/session'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'
import { StudioImagePreview, StudioThumbnail } from '@/image-studio/image'
import { saveStudioImage } from '@/image-studio/images'

import './index.scss'

export default function ImageStudioPage() {
  const { t } = useTranslation()
  const [owner, setOwner] = useState(() => getMiniAuthSession()?.user.id)
  const [visible, setVisible] = useState(true)
  const [refresh, setRefresh] = useState(0)
  const shown = useRef(false)
  usePageTitle('studio.title')
  useDidShow(() => {
    setOwner(getMiniAuthSession()?.user.id)
    setVisible(true)
    if (shown.current) setRefresh(value => value + 1)
    shown.current = true
  })
  useDidHide(() => setVisible(false))

  return owner ? (
    <StudioWorkspace key={owner} owner={owner} visible={visible} refresh={refresh} />
  ) : (
    <PageShell compact title={t('studio.title')} description={t('studio.description')}>
      <View className='mobile-card mobile-stack'>
        <Text>{t('studio.login')}</Text>
        <Button className='mobile-button' onClick={() => Taro.switchTab({ url: '/pages/profile/index' })}>
          {t('studio.signIn')}
        </Button>
      </View>
    </PageShell>
  )
}

function StudioWorkspace(props: { owner: number; visible: boolean; refresh: number }) {
  const { t } = useTranslation()
  const [settings, setSettings] = useState<StudioSettings>()
  const [keys, setKeys] = useState<ApiToken[]>([])
  const [group, setGroup] = useState('')
  const [keyId, setKeyId] = useState(0)
  const [textKeyId, setTextKeyId] = useState(0)
  const [models, setModels] = useState<string[]>([])
  const [textModels, setTextModels] = useState<string[]>([])
  const [model, setModel] = useState('')
  const [textModel, setTextModel] = useState('')
  const [showConfig, setShowConfig] = useState(false)
  const [prompt, setPrompt] = useState('')
  const [originalPrompt, setOriginalPrompt] = useState<string>()
  const [count, setCount] = useState(1)
  const [runs, setRuns] = useState<StudioRun[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [current, setCurrent] = useState<StudioRun>()
  const [resultId, setResultId] = useState('')
  const [loading, setLoading] = useState(true)
  const [reload, setReload] = useState(0)
  const [historyRefresh, setHistoryRefresh] = useState(0)
  const [operation, setOperation] = useState('')
  const [error, setError] = useState('')
  const [albumDenied, setAlbumDenied] = useState(false)
  const operationLock = useRef(false)
  const mounted = useRef(true)
  const submission = useRef({ input: '', key: '' })
  const selected = current?.results.find(result => result.id === resultId) || current?.results[0]
  const imageUrl = selected?.preview_url || ''
  const pending = current?.status === 'running'
  const busy = Boolean(operation) || pending
  const activeOwner = () => mounted.current && getMiniAuthSession()?.user.id === props.owner
  const showError = (value: unknown) => setError(value instanceof ApiRequestError ? value.message : t('studio.error'))

  useEffect(() => {
    if (imageUrl) void Taro.pageScrollTo({ selector: '#studio-result', duration: 200 }).catch(() => {})
  }, [imageUrl])
  useEffect(() => {
    if (error) void Taro.pageScrollTo({ selector: '#studio-error', duration: 200 }).catch(() => {})
  }, [error])

  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  useEffect(() => {
    let active = true
    setLoading(true)
    setError('')
    void getStudioSettings()
      .then(async config => {
        if (!active || !activeOwner()) return
        setSettings(config)
        if (!config.enabled) {
          setCurrent(undefined)
          setRuns([])
          setKeys([])
          void Taro.switchTab({ url: '/pages/home/index' })
          return
        }
        const [tokens, account] = await Promise.all([getStudioKeys(), getAccountSummary()])
        if (!active || !activeOwner()) return
        setCount(value => Math.min(value, config.max_images))
        setKeys(tokens)
        setGroup(account.group)
        setKeyId(value => (tokens.some(key => key.id === value) ? value : tokens[0]?.id || 0))
        setTextKeyId(value => (tokens.some(key => key.id === value) ? value : tokens[0]?.id || 0))
      })
      .catch(value => {
        if (active && activeOwner()) showError(value)
      })
      .finally(() => {
        if (active) setLoading(false)
      })
    return () => {
      active = false
    }
  }, [reload, props.refresh])

  useEffect(() => {
    let active = true
    setModels([])
    if (keyId && settings?.enabled)
      void getStudioModels(keyId, 'image')
        .then(values => {
          if (active && activeOwner()) {
            setModels(values)
            setModel(value => (values.includes(value) ? value : values[0] || ''))
          }
        })
        .catch(value => {
          if (active && activeOwner()) showError(value)
        })
    return () => {
      active = false
    }
  }, [keyId, settings?.enabled, reload, props.refresh])
  useEffect(() => {
    let active = true
    setTextModels([])
    if (textKeyId && settings?.enabled)
      void getStudioModels(textKeyId, 'text')
        .then(values => {
          if (active && activeOwner()) {
            setTextModels(values)
            setTextModel(value => (values.includes(value) ? value : values[0] || ''))
          }
        })
        .catch(value => {
          if (active && activeOwner()) showError(value)
        })
    return () => {
      active = false
    }
  }, [textKeyId, settings?.enabled, reload, props.refresh])
  useEffect(() => {
    if (!settings?.enabled || !props.visible) return
    let active = true
    void listStudioRuns(page)
      .then(data => {
        if (active && activeOwner()) {
          setRuns(data.items)
          setTotal(data.total)
        }
      })
      .catch(value => {
        if (active && activeOwner()) showError(value)
      })
    return () => {
      active = false
    }
  }, [page, settings?.enabled, props.visible, reload, historyRefresh, props.refresh])

  useEffect(() => {
    if (!pending || !current || !props.visible || !settings?.enabled || loading) return
    let active = true
    const timer = setTimeout(() => {
      void getStudioRun(current.id)
        .then(run => {
          if (!active || !activeOwner()) return
          if (run.status === 'failed') {
            setError(run.error_message || t('studio.error'))
            setCurrent(undefined)
          } else setCurrent(run)
          if (run.status !== 'running') {
            submission.current = { input: '', key: '' }
            setHistoryRefresh(value => value + 1)
          }
        })
        .catch(value => {
          if (active && activeOwner()) {
            showError(value)
            setCurrent(undefined)
          }
        })
    }, 1500)
    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [current, pending, props.visible, settings?.enabled, loading])

  const maxImages = model === 'dall-e-3' ? 1 : settings?.max_images || 1
  const countOptions = Array.from({ length: maxImages }, (_, index) => String(index + 1))
  useEffect(() => {
    setCount(value => Math.min(value, maxImages))
  }, [maxImages])

  const execute = async (kind: 'generate' | 'optimize') => {
    if (operationLock.current || busy || !settings?.enabled || !prompt.trim()) return
    if (kind === 'generate' && !models.includes(model)) return
    if (kind === 'optimize' && !textModels.includes(textModel)) return
    operationLock.current = true
    setOperation(kind)
    setError('')
    try {
      if (kind === 'optimize') {
        const response = await assistStudioPrompt(textKeyId, textModel, prompt)
        if (activeOwner()) {
          setOriginalPrompt(prompt)
          setPrompt(response.suggested_prompt)
        }
      } else {
        const input = {
          token_id: keyId,
          request: {
            model,
            prompt: prompt.trim(),
            ...(count > 1 ? { n: count } : {}),
          },
        }
        const signature = JSON.stringify(input)
        if (submission.current.input !== signature)
          submission.current = { input: signature, key: `mini-${Date.now()}-${Math.random().toString(36).slice(2)}` }
        const run = await createStudioRun({ ...input, idempotency_key: submission.current.key })
        if (!activeOwner()) return
        setCurrent(run)
        setResultId(run.results[0]?.id || '')
        setAlbumDenied(false)
        if (run.status !== 'running') submission.current = { input: '', key: '' }
        setPage(1)
        setHistoryRefresh(value => value + 1)
      }
    } catch (value) {
      if (activeOwner()) showError(value)
    } finally {
      operationLock.current = false
      if (mounted.current) setOperation('')
    }
  }

  const openRun = async (id: string, selectedId: string) => {
    if (operationLock.current || busy) return
    operationLock.current = true
    setOperation('open')
    setError('')
    try {
      const run = await getStudioRun(id)
      if (activeOwner()) {
        setCurrent(run)
        setResultId(run.results.find(result => result.id === selectedId)?.id || run.results[0]?.id || '')
        setAlbumDenied(false)
      }
    } catch (value) {
      if (activeOwner()) showError(value)
    } finally {
      operationLock.current = false
      if (mounted.current) setOperation('')
    }
  }

  const viewResult = async (kind: 'preview' | 'save') => {
    if (!current || !selected || operationLock.current || busy) return
    operationLock.current = true
    setOperation(kind)
    setError('')
    setAlbumDenied(false)
    try {
      // Refresh the signed URL through the same owner-scoped API before opening or saving it.
      const run = await getStudioRun(current.id)
      if (!activeOwner()) return
      const result = run.results.find(value => value.id === selected.id)
      if (!result?.preview_url) throw new Error('STUDIO_IMAGE_UNAVAILABLE')
      setCurrent(run)
      if (kind === 'preview') {
        await Taro.previewImage({
          current: result.preview_url,
          urls: run.results.flatMap(value => value.preview_url ? [value.preview_url] : []),
        })
      } else {
        await saveStudioImage(result.download_url || result.preview_url)
        if (activeOwner()) await Taro.showToast({ title: t('studio.saved'), icon: 'success' })
      }
    } catch (value) {
      if (activeOwner()) {
        const denied = value instanceof Error && value.message === 'STUDIO_ALBUM_PERMISSION'
        setAlbumDenied(denied)
        let message = kind === 'save' ? 'studio.saveFailed' : 'studio.previewFailed'
        if (denied) message = 'studio.albumPermission'
        setError(t(message))
      }
    } finally {
      operationLock.current = false
      if (mounted.current) setOperation('')
    }
  }
  const keyLabels = keys.map(key => `${key.name} · ${key.group || group}`)

  if (loading || !settings?.enabled) {
    return (
      <PageShell compact title={t('studio.title')} description={t('studio.description')}>
        <View className='mobile-card mobile-stack'>
          <Text className={error ? 'mobile-error' : 'mobile-muted'}>
            {error || t(loading ? 'common.loading' : 'studio.disabled')}
          </Text>
          {error ? (
            <Button className='mobile-button' onClick={() => setReload(value => value + 1)}>
              {t('common.retry')}
            </Button>
          ) : null}
        </View>
      </PageShell>
    )
  }

  return (
    <View className='studio-workspace'>
      <PageShell
        compact
        title={t('studio.title')}
        description={t('studio.description')}
        trailing={
          <Button
            className='mobile-button studio-config-toggle'
            aria-expanded={showConfig}
            disabled={busy}
            onClick={() => setShowConfig(value => !value)}
          >
            <View className='studio-config-toggle__icon' aria-hidden />
            {t('studio.configure')}
          </Button>
        }
      >
        {error ? (
          <View id='studio-error' className='mobile-stack'>
            <Text className='mobile-error'>{error}</Text>
            {albumDenied ? (
              <Button
                className='mobile-button mobile-button--secondary'
                onClick={() => void Taro.openSetting().catch(() => setError(t('studio.albumPermission')))}
              >
                {t('studio.openSettings')}
              </Button>
            ) : (
              <Button
                className='mobile-button mobile-button--small mobile-button--secondary'
                disabled={busy}
                onClick={() => setReload(value => value + 1)}
              >
                {t('common.retry')}
              </Button>
            )}
          </View>
        ) : null}
        {!loading && settings && !keys.length ? (
          <View className='mobile-card mobile-stack'>
            <Text>{t('studio.noKeys')}</Text>
            <Button
              className='mobile-button mobile-button--secondary'
              onClick={() => Taro.navigateTo({ url: '/pages/keys/index' })}
            >
              {t('keys.manage')}
            </Button>
          </View>
        ) : null}
        {showConfig && keys.length ? (
          <View className='mobile-card mobile-stack studio-config'>
            <View className='studio-config__columns'>
              <View className='mobile-stack studio-config__column'>
                <Text className='mobile-card__title'>{t('studio.imageModel')}</Text>
                <Text className='studio-label'>{t('studio.key')}</Text>
                <Picker
                  disabled={busy}
                  range={keyLabels}
                  value={Math.max(
                    0,
                    keys.findIndex(key => key.id === keyId)
                  )}
                  onChange={event => setKeyId(keys[Number(event.detail.value)].id)}
                >
                  <View className='mobile-picker'>{keyLabels[keys.findIndex(key => key.id === keyId)]}</View>
                </Picker>
                <Picker
                  disabled={busy || !models.length}
                  range={models}
                  value={Math.max(0, models.indexOf(model))}
                  onChange={event => setModel(models[Number(event.detail.value)])}
                >
                  <View className='mobile-picker'>{model || t('studio.noModels')}</View>
                </Picker>
              </View>
              <View className='mobile-stack studio-config__column'>
                <Text className='mobile-card__title'>{t('studio.textModel')}</Text>
                <Text className='studio-label'>{t('studio.key')}</Text>
                <Picker
                  disabled={busy}
                  range={keyLabels}
                  value={Math.max(
                    0,
                    keys.findIndex(key => key.id === textKeyId)
                  )}
                  onChange={event => setTextKeyId(keys[Number(event.detail.value)].id)}
                >
                  <View className='mobile-picker'>{keyLabels[keys.findIndex(key => key.id === textKeyId)]}</View>
                </Picker>
                <Picker
                  disabled={busy || !textModels.length}
                  range={textModels}
                  value={Math.max(0, textModels.indexOf(textModel))}
                  onChange={event => setTextModel(textModels[Number(event.detail.value)])}
                >
                  <View className='mobile-picker'>{textModel || t('studio.noModels')}</View>
                </Picker>
              </View>
            </View>
            <Text className='mobile-card__meta'>{t('studio.billing')}</Text>
          </View>
        ) : null}
        <View className='studio-inspiration'>
          <Text className='studio-label'>{t('studio.try')}</Text>
          <View className='studio-examples'>
            {['cat', 'town', 'product'].map(example => (
              <Button
                key={example}
                className={`studio-example studio-example--${example}`}
                disabled={busy || !settings?.enabled}
                onClick={() => {
                  setPrompt(t(`studio.example.${example}`))
                  setOriginalPrompt(undefined)
                }}
              >
                <Text className='studio-example__title'>{t(`studio.style.${example}`)}</Text>
                <Text className='studio-example__prompt'>{t(`studio.example.${example}`)}</Text>
              </Button>
            ))}
          </View>
        </View>
        <View id='studio-composer' className='mobile-card mobile-stack studio-composer'>
          <View className='mobile-row'>
            <Text className='mobile-card__title'>{t('studio.prompt')}</Text>
            <Text className='mobile-chip'>{model || t('studio.noModels')}</Text>
          </View>
          <Textarea
            className='mobile-textarea studio-prompt'
            value={prompt}
            disabled={busy || !settings?.enabled}
            maxlength={settings?.max_prompt_characters || 32000}
            placeholder={t('studio.placeholder')}
            onInput={event => {
              setPrompt(event.detail.value)
              setOriginalPrompt(undefined)
            }}
          />
          <View className='mobile-actions studio-prompt-actions'>
            <Button
              className='mobile-button mobile-button--small mobile-button--secondary'
              disabled={busy || !settings?.enabled || !prompt.trim() || !textModels.includes(textModel)}
              onClick={() => void execute('optimize')}
            >
              {t(operation === 'optimize' ? 'studio.optimizing' : 'studio.optimize')}
            </Button>
            {originalPrompt !== undefined ? (
              <Button
                className='mobile-button mobile-button--small mobile-button--secondary'
                disabled={busy}
                onClick={() => {
                  setPrompt(originalPrompt)
                  setOriginalPrompt(undefined)
                }}
              >
                {t('studio.undo')}
              </Button>
            ) : null}
          </View>
          <Text className='mobile-card__meta'>{t('studio.textModel')}: {textModel || t('studio.noModels')}</Text>
          <View>
            <Text className='studio-label'>{t('studio.proportions')}</Text>
            <View className='mobile-picker'>{t('studio.modelDefault')}</View>
            <Text className='mobile-card__meta'>{t('studio.proportionsHint')}</Text>
          </View>
          <View className='studio-fields'>
            <View>
              <Text className='studio-label'>{t('studio.count')}</Text>
              <Picker
                disabled={busy || maxImages === 1}
                range={countOptions}
                value={count - 1}
                onChange={event => setCount(Number(event.detail.value) + 1)}
              >
                <View className='mobile-picker studio-picker'>
                  <Text>{count}</Text>
                  <Text aria-hidden>⌄</Text>
                </View>
              </Picker>
            </View>
          </View>
          {model === 'dall-e-3' ? <Text className='mobile-card__meta'>{t('studio.singleImage')}</Text> : null}
          <Button
            className='mobile-button studio-generate'
            loading={operation === 'generate' || pending}
            disabled={busy || !settings?.enabled || !models.includes(model) || !prompt.trim()}
            onClick={() => void execute('generate')}
          >
            {t(operation === 'generate' || pending ? 'studio.generating' : 'studio.generate')}
          </Button>
        </View>
        {current ? (
          <View id='studio-result' className='mobile-card mobile-stack studio-result-card'>
            <View className='mobile-row'>
              <Text className='mobile-card__title'>{t('studio.result')}</Text>
              <Text className='mobile-chip'>{current.model}</Text>
            </View>
            {pending ? <Text className='mobile-muted'>{t('studio.generating')}</Text> : null}
            {imageUrl ? (
              <StudioImagePreview
                key={imageUrl}
                path={imageUrl}
                onPreview={() => void viewResult('preview')}
                onError={() => setError(t('studio.previewFailed'))}
              />
            ) : selected ? (
              <Text className='mobile-muted'>{t('studio.previewFailed')}</Text>
            ) : null}
            {current.results.length > 1 ? (
              <ScrollView className='studio-result-strip' scrollX>
                <View className='studio-result-strip__items'>
                  {current.results.map(result => (
                    <View
                      key={result.id}
                      className={`studio-result-option ${selected?.id === result.id ? 'studio-result-option--active' : ''}`}
                      onClick={() => {
                        if (!busy) {
                          setResultId(result.id)
                          setAlbumDenied(false)
                        }
                      }}
                    >
                      <StudioThumbnail source={result.thumbnail_url || result.preview_url} />
                    </View>
                  ))}
                </View>
              </ScrollView>
            ) : null}
            <Text className='studio-result-description'>{selected?.description || current.prompt}</Text>
            <Button
              className='mobile-button mobile-button--small mobile-button--secondary'
              disabled={
                busy || !settings?.enabled || !current.prompt || current.prompt.length > settings.max_prompt_characters
              }
              onClick={() => {
                setPrompt(current.prompt || '')
                setOriginalPrompt(undefined)
                void Taro.pageScrollTo({ selector: '#studio-composer', duration: 200 }).catch(() => {})
              }}
            >
              {t('studio.reusePrompt')}
            </Button>
            <Text className='mobile-card__meta studio-task-id'>{t('studio.task', { id: current.id })}</Text>
            <View className='studio-result-actions'>
              <Button
                className='mobile-button mobile-button--secondary'
                disabled={busy || !imageUrl}
                onClick={() => void viewResult('preview')}
              >
                {t('studio.preview')}
              </Button>
              <Button
                className='mobile-button'
                loading={operation === 'save'}
                disabled={busy || !imageUrl}
                onClick={() => void viewResult('save')}
              >
                {t('studio.save')}
              </Button>
            </View>
            <Text className='mobile-card__meta studio-storage-hint'>{t('studio.storageHint')}</Text>
          </View>
        ) : null}
        <View className='mobile-stack studio-history'>
          <View className='mobile-row'>
            <Text className='mobile-card__title'>{t('studio.history')}</Text>
            <Text className='mobile-chip'>{total}</Text>
          </View>
          <Text className='mobile-card__meta studio-history-owner'>
            {t('studio.owner', { name: getMiniAuthSession()?.user.username, id: props.owner })}
          </Text>
          {!runs.length ? (
            <Text className='mobile-empty'>{t('studio.empty')}</Text>
          ) : (
            <View className='studio-history-grid'>
              {runs.flatMap(run =>
                run.results.map(result => (
                  <Button
                    key={result.id}
                    aria-pressed={current?.id === run.id && selected?.id === result.id}
                    className={`studio-history-item ${current?.id === run.id && selected?.id === result.id ? 'studio-history-item--active' : ''}`}
                    disabled={busy}
                    onClick={() => void openRun(run.id, result.id)}
                  >
                    <StudioThumbnail source={result.thumbnail_url || result.preview_url} />
                    <View className='studio-history-item__text'>
                      <Text className='studio-history-item__title'>{result.description || run.prompt}</Text>
                      <Text className='mobile-card__meta'>
                        {run.model} · {t('studio.task', { id: run.id })}
                      </Text>
                    </View>
                  </Button>
                ))
              )}
            </View>
          )}
          <View className='mobile-row studio-pagination'>
            <Button
              className='mobile-button mobile-button--small mobile-button--secondary'
              disabled={busy || page <= 1}
              onClick={() => setPage(value => value - 1)}
            >
              {t('studio.previous')}
            </Button>
            <Text className='mobile-muted'>{page}</Text>
            <Button
              className='mobile-button mobile-button--small mobile-button--secondary'
              disabled={busy || page * 8 >= total}
              onClick={() => setPage(value => value + 1)}
            >
              {t('common.loadMore')}
            </Button>
          </View>
        </View>
      </PageShell>
    </View>
  )
}
