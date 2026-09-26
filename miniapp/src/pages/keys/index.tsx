import { useCallback, useEffect, useState } from 'react'

import { Button, Input, Picker, Text, View } from '@tarojs/components'
import Taro, { useDidHide } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getUserGroups } from '@/api/models'
import {
  createApiToken,
  deleteApiToken,
  getApiTokens,
  revealApiToken,
  setApiTokenEnabled,
  verifyApiTokenAccess,
  type ApiToken,
} from '@/api/tokens'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'
import { formatTime } from '@/utils/format'

import './index.scss'

const tokenStatusKeys: Record<number, string> = {
  1: 'keys.enabled',
  2: 'keys.disabled',
  3: 'keys.expired',
  4: 'keys.exhausted',
}

export default function KeysPage() {
  const { t } = useTranslation()
  const { formatQuota } = useQuotaDisplay()
  const [tokens, setTokens] = useState<ApiToken[]>([])
  const [name, setName] = useState('')
  const [groups, setGroups] = useState<string[]>([])
  const [group, setGroup] = useState('')
  const [selected, setSelected] = useState<ApiToken | null>(null)
  const [password, setPassword] = useState('')
  const [twoFA, setTwoFA] = useState('')
  const [fullKey, setFullKey] = useState('')
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState<number | null>(null)
  const [error, setError] = useState(false)
  usePageTitle('keys.title')

  const clearSecret = useCallback(() => {
    setSelected(null)
    setPassword('')
    setTwoFA('')
    setFullKey('')
  }, [])
  useDidHide(clearSecret)

  const load = useCallback(() => {
    setLoading(true)
    setError(false)
    Promise.all([getApiTokens(), getUserGroups()])
      .then(([page, userGroups]) => {
        const groupNames = Object.keys(userGroups).filter((name) => name !== 'auto')
        setTokens(page.items)
        setGroups(groupNames)
        setGroup((current) => (groupNames.includes(current) ? current : groupNames[0] || ''))
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    load()
    return clearSecret
  }, [clearSecret, load])

  const create = async () => {
    if (!name.trim()) return
    setError(false)
    setBusyId(0)
    try {
      await createApiToken(name.trim(), group)
      setName('')
      load()
    } catch {
      setError(true)
    } finally {
      setBusyId(null)
    }
  }

  const toggle = async (token: ApiToken) => {
    setBusyId(token.id)
    setError(false)
    try {
      await setApiTokenEnabled(token, token.status !== 1)
      setTokens((items) =>
        items.map((item) =>
          item.id === token.id ? { ...item, status: token.status === 1 ? 2 : 1 } : item
        )
      )
    } catch {
      setError(true)
    } finally {
      setBusyId(null)
    }
  }

  const remove = async (token: ApiToken) => {
    const confirmation = await Taro.showModal({
      title: t('keys.delete'),
      content: t('keys.deleteConfirm'),
    })
    if (!confirmation.confirm) return
    setBusyId(token.id)
    try {
      await deleteApiToken(token.id)
      setTokens((items) => items.filter((item) => item.id !== token.id))
      if (selected?.id === token.id) clearSecret()
    } catch {
      setError(true)
    } finally {
      setBusyId(null)
    }
  }

  const reveal = async () => {
    if (!selected || !password) return
    setBusyId(selected.id)
    setError(false)
    try {
      const proof = await verifyApiTokenAccess(password, twoFA)
      const secret = await revealApiToken(selected.id, proof.proof_token)
      setFullKey(secret.key)
      setPassword('')
      setTwoFA('')
    } catch {
      setError(true)
      setFullKey('')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <PageShell title={t('keys.title')} description={t('keys.description')}>
      <View className='mobile-card mobile-stack'>
        <Text className='mobile-card__title'>{t('keys.create')}</Text>
        <Input className='mobile-input' value={name} maxlength={50} placeholder={t('keys.name')} onInput={(event) => setName(event.detail.value)} />
        <Picker mode='selector' range={groups} value={Math.max(0, groups.indexOf(group))} onChange={(event) => setGroup(groups[Number(event.detail.value)] ?? '')}>
          <View className='mobile-picker'>{t('models.group')}: {group || '—'}</View>
        </Picker>
        <Button className='mobile-button' disabled={!name.trim() || !group || busyId !== null} onClick={create}>{t('common.create')}</Button>
      </View>

      {error ? <Text className='mobile-error'>{t('keys.error')}</Text> : null}
      {loading ? <Text className='mobile-empty'>{t('common.loading')}</Text> : null}
      {!loading && tokens.length === 0 ? <Text className='mobile-empty'>{t('keys.empty')}</Text> : null}
      <View className='mobile-stack'>
        {tokens.map((token) => (
          <View className='mobile-card' key={token.id}>
            <View className='mobile-row'>
              <Text className='mobile-card__title'>{token.name}</Text>
              <Text className='mobile-chip'>{t(tokenStatusKeys[token.status] ?? 'common.unknown')}</Text>
            </View>
            <Text className='mobile-card__meta'>{token.key}</Text>
            <Text className='mobile-card__meta'>{t('keys.created')}: {formatTime(token.created_time)}</Text>
            <Text className='mobile-card__meta'>
              {token.unlimited_quota
                ? t('keys.unlimited')
                : `${t('keys.remaining')}: ${formatQuota(token.remain_quota)}`}
            </Text>
            <View className='mobile-actions'>
              <Button className='mobile-button mobile-button--small mobile-button--secondary' onClick={() => { clearSecret(); setSelected(token) }}>{t('keys.reveal')}</Button>
              <Button className='mobile-button mobile-button--small mobile-button--secondary' disabled={busyId === token.id} onClick={() => toggle(token)}>
                {token.status === 1 ? t('common.disable') : t('common.enable')}
              </Button>
              <Button className='mobile-button mobile-button--small mobile-button--danger' disabled={busyId === token.id} onClick={() => remove(token)}>{t('common.delete')}</Button>
            </View>
          </View>
        ))}
      </View>

      {selected ? (
        <View className='mobile-card mobile-stack'>
          <Text className='mobile-card__title'>{t('keys.revealTitle', { name: selected.name })}</Text>
          <Text className='mobile-card__meta'>{t('keys.secretWarning')}</Text>
          {!fullKey ? (
            <>
              <Input className='mobile-input' password value={password} placeholder={t('keys.password')} onInput={(event) => setPassword(event.detail.value)} />
              <Input className='mobile-input' value={twoFA} placeholder={t('keys.twoFA')} onInput={(event) => setTwoFA(event.detail.value)} />
              <Button className='mobile-button' disabled={!password || busyId !== null} onClick={reveal}>{t('keys.verify')}</Button>
            </>
          ) : (
            <>
              <Text className='keys-secret'>{fullKey}</Text>
              <Button className='mobile-button' onClick={() => Taro.setClipboardData({ data: fullKey })}>{t('common.copy')}</Button>
            </>
          )}
          <Button className='mobile-button mobile-button--secondary' onClick={clearSecret}>{t('common.cancel')}</Button>
        </View>
      ) : null}
    </PageShell>
  )
}
