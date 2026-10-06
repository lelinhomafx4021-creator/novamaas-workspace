/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Button, Image, Input, Text, View } from '@tarojs/components'
import Taro, { useDidShow } from '@tarojs/taro'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiRequestError } from '@/api/request'
import {
  getWeChatAvatar,
  getWeChatBindings,
  sendWeChatSecuritySMS,
  unlinkWeChat,
  updateWeChatProfile,
  verifyWeChatSecurity,
  type WeChatBinding,
} from '@/api/wechat'
import { clearMiniAuthSession } from '@/auth/session'
import { clearConversation } from '@/playground/storage'

import './wechat-binding.scss'

export function WeChatBindingEntry(props: {
  binding: WeChatBinding
  editable?: boolean
  onReload: () => void
  onUnlinked?: () => void
}) {
  const { t } = useTranslation()
  const [avatar, setAvatar] = useState('')
  const [selectedAvatar, setSelectedAvatar] = useState('')
  const [nickname, setNickname] = useState(props.binding.nickname)
  const nicknameValue = useRef(props.binding.nickname)
  const [editing, setEditing] = useState(false)
  const [nicknameFocused, setNicknameFocused] = useState(false)
  const [unlinking, setUnlinking] = useState(false)
  const [password, setPassword] = useState('')
  const [twoFactor, setTwoFactor] = useState('')
  const [challenge, setChallenge] = useState('')
  const [code, setCode] = useState('')
  const [seconds, setSeconds] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    let cancelled = false
    if (props.binding.has_avatar) {
      void getWeChatAvatar(props.binding.app_id)
        .then((url) => {
          if (!cancelled) setAvatar(url)
        })
        .catch(() => {
          if (!cancelled) setAvatar('')
        })
    } else setAvatar('')
    return () => {
      cancelled = true
    }
  }, [props.binding.app_id, props.binding.has_avatar, props.binding.updated_at])
  useEffect(() => {
    setNickname(props.binding.nickname)
    nicknameValue.current = props.binding.nickname
  }, [props.binding.nickname])
  useEffect(() => {
    if (seconds <= 0) return
    const timer = setTimeout(() => setSeconds((value) => value - 1), 1000)
    return () => clearTimeout(timer)
  }, [seconds])
  const save = async () => {
    setBusy(true)
    setError('')
    try {
      await updateWeChatProfile(
        props.binding.app_id,
        nicknameValue.current.trim(),
        selectedAvatar || undefined
      )
      setSelectedAvatar('')
      setEditing(false)
      props.onReload()
    } catch {
      setError('wechat.profileError')
    } finally {
      setBusy(false)
    }
  }
  const send = async () => {
    setBusy(true)
    setError('')
    setCode('')
    try {
      const data = await sendWeChatSecuritySMS()
      setChallenge(data.challenge_token)
      setSeconds(60)
    } catch {
      setError('phone.unavailable')
    } finally {
      setBusy(false)
    }
  }
  const unlink = async (method: 'password' | 'sms') => {
    setBusy(true)
    setError('')
    try {
      const proof = await verifyWeChatSecurity(
        method === 'password'
          ? { password, two_factor_code: twoFactor }
          : { challenge_token: challenge, code }
      )
      await unlinkWeChat(props.binding.app_id, proof)
      clearMiniAuthSession()
      clearConversation()
      props.onUnlinked?.()
      await Taro.showToast({ title: t('wechat.unlinked'), icon: 'none' })
    } catch (error) {
      setError(
        error instanceof ApiRequestError &&
          error.code === 'MINI_AUTH_LAST_CREDENTIAL'
          ? 'wechat.lastCredential'
          : 'wechat.unlinkError'
      )
    } finally {
      setBusy(false)
    }
  }
  return (
    <View className='wechat-binding-entry'>
      <View className='wechat-binding-entry__summary'>
        {avatar || selectedAvatar ? (
          <Image
            className='wechat-binding-entry__avatar'
            src={selectedAvatar || avatar}
            mode='aspectFill'
          />
        ) : (
          <View className='wechat-binding-entry__avatar wechat-binding-entry__fallback'>
            <Text>WX</Text>
          </View>
        )}
        <View className='wechat-binding-entry__identity'>
          <Text className='wechat-binding-entry__name'>
            {props.binding.nickname || t('wechat.noProfile')}
          </Text>
          <Text className='wechat-binding-entry__status'>
            {t('wechat.bound')}
          </Text>
          <Text className='wechat-binding-entry__time'>
            {t('wechat.boundAt', {
              time: new Date(props.binding.bound_at).toLocaleString(),
            })}
          </Text>
        </View>
      </View>
      {props.editable ? (
        <>
          {!unlinking && !editing ? (
            <View className='wechat-binding-entry__actions'>
              <Button
                className='wechat-binding-entry__action'
                disabled={busy}
                onClick={() => setEditing(true)}
              >
                {t('wechat.editProfile')}
              </Button>
              <Button
                className='profile-link'
                disabled={busy}
                onClick={() => setUnlinking(true)}
              >
                {t('wechat.unlink')}
              </Button>
            </View>
          ) : null}
          {editing ? (
            <View className='profile-form wechat-binding-entry__editor'>
              <Text className='profile-card__hint'>
                {t('wechat.profileHint')}
              </Text>
              <Button
                className='wechat-binding-entry__action'
                openType='chooseAvatar'
                disabled={busy}
                onChooseAvatar={(event) =>
                  setSelectedAvatar(event.detail.avatarUrl)
                }
              >
                {avatar || selectedAvatar
                  ? t('wechat.changeAvatar')
                  : t('wechat.chooseAvatar')}
              </Button>
              <Button
                className='wechat-binding-entry__action'
                disabled={busy}
                onClick={() => setNicknameFocused(true)}
              >
                {t('wechat.useWeChatNickname')}
              </Button>
              <Text className='profile-card__hint'>
                {t('wechat.nicknameAuthorizationHint')}
              </Text>
              <Input
                className='profile-input'
                type='nickname'
                focus={nicknameFocused}
                value={nickname}
                maxlength={64}
                disabled={busy}
                placeholder={t('wechat.nickname')}
                ariaLabel={t('wechat.nickname')}
                onInput={(event) => {
                  nicknameValue.current = event.detail.value
                  setNickname(event.detail.value)
                }}
                onFocus={() => setNicknameFocused(true)}
                onBlur={(event) => {
                  nicknameValue.current = event.detail.value
                  setNickname(event.detail.value)
                  setNicknameFocused(false)
                }}
                onConfirm={(event) => {
                  nicknameValue.current = event.detail.value
                  setNickname(event.detail.value)
                  setNicknameFocused(false)
                }}
              />
              <View className='wechat-binding-entry__actions'>
                <Button
                  className='wechat-binding-entry__action'
                  disabled={
                    busy ||
                    (nickname === props.binding.nickname && !selectedAvatar)
                  }
                  onClick={save}
                >
                  {t('wechat.save')}
                </Button>
                <Button
                  className='profile-link'
                  disabled={busy}
                  onClick={() => {
                    setEditing(false)
                    setSelectedAvatar('')
                    setNickname(props.binding.nickname)
                    nicknameValue.current = props.binding.nickname
                    setNicknameFocused(false)
                    setError('')
                  }}
                >
                  {t('wechat.cancelEdit')}
                </Button>
              </View>
            </View>
          ) : null}
          {unlinking ? (
            <>
              <Text className='profile-card__hint'>
                {t('wechat.unlinkHint')}
              </Text>
              <Input
                className='profile-input'
                password
                value={password}
                disabled={busy}
                placeholder={t('auth.password')}
                onInput={(event) => setPassword(event.detail.value)}
              />
              <Input
                className='profile-input'
                value={twoFactor}
                disabled={busy}
                placeholder={t('auth.twoFactorCode')}
                onInput={(event) => setTwoFactor(event.detail.value)}
              />
              <Button
                className='profile-button'
                disabled={busy || !password}
                onClick={() => unlink('password')}
              >
                {t('wechat.verifyUnlink')}
              </Button>
              <Button
                className='profile-button profile-button--secondary'
                disabled={busy || seconds > 0}
                onClick={send}
              >
                {seconds
                  ? t('phone.resend', { seconds })
                  : t('wechat.verifyPhone')}
              </Button>
              <Input
                className='profile-input'
                type='number'
                maxlength={6}
                disabled={busy || !challenge}
                value={code}
                placeholder={t('phone.code')}
                onInput={(event) => setCode(event.detail.value)}
              />
              <Button
                className='profile-button'
                disabled={busy || !challenge || !/^\d{6}$/.test(code)}
                onClick={() => unlink('sms')}
              >
                {t('wechat.verifyUnlink')}
              </Button>
              <Button
                className='profile-link'
                disabled={busy}
                onClick={() => {
                  setUnlinking(false)
                  setPassword('')
                  setTwoFactor('')
                  setCode('')
                  setChallenge('')
                  setError('')
                }}
              >
                {t('auth.accountLoginBack')}
              </Button>
            </>
          ) : null}
        </>
      ) : null}
      {error ? <Text className='profile-card__error'>{t(error)}</Text> : null}
    </View>
  )
}

export function WeChatBindingPanel(props: {
  editable?: boolean
  onUnlinked?: () => void
}) {
  const { t } = useTranslation()
  const [bindings, setBindings] = useState<WeChatBinding[]>([])
  const [phase, setPhase] = useState<'loading' | 'ready' | 'error'>('loading')
  const [revision, setRevision] = useState(0)
  useDidShow(() => setRevision((value) => value + 1))
  useEffect(() => {
    let cancelled = false
    setPhase('loading')
    void getWeChatBindings()
      .then((data) => {
        if (!cancelled) {
          setBindings(data)
          setPhase('ready')
        }
      })
      .catch(() => {
        if (!cancelled) setPhase('error')
      })
    return () => {
      cancelled = true
    }
  }, [revision])
  return (
    <View className='profile-card wechat-binding-panel'>
      <Text className='profile-card__label'>{t('wechat.title')}</Text>
      {phase === 'loading' ? <Text>{t('auth.busy')}</Text> : null}
      {phase === 'error' ? (
        <Button
          className='profile-link'
          onClick={() => setRevision((value) => value + 1)}
        >
          {t('common.retry')}
        </Button>
      ) : null}
      {phase === 'ready' && bindings.length === 0 ? (
        <Text className='profile-card__hint'>{t('wechat.notBound')}</Text>
      ) : null}
      {bindings.map((binding) => (
        <WeChatBindingEntry
          key={binding.app_id}
          binding={binding}
          editable={props.editable}
          onReload={() => setRevision((value) => value + 1)}
          onUnlinked={props.onUnlinked}
        />
      ))}
    </View>
  )
}
