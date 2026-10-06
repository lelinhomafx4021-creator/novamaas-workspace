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
import { Button, Input, Text, View } from '@tarojs/components'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ApiRequestError } from '@/api/request'
import { loginWithMiniSMS, sendMiniSMS } from '@/auth/client'
import type { MiniAuthSession } from '@/auth/session'

export function smsErrorKey(error: unknown) {
  const code = error instanceof ApiRequestError ? error.code : ''
  if (code === 'PHONE_RATE_LIMITED') return 'phone.rateLimited'
  if (code === 'PHONE_SMS_UNAVAILABLE') return 'phone.unavailable'
  if (code === 'PHONE_NUMBER_INVALID') return 'phone.invalidNumber'
  return 'phone.invalidCode'
}

export function SmsLogin(props: {
  flowToken?: string
  onLogin: (session: MiniAuthSession) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [seconds, setSeconds] = useState(0)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => {
    if (seconds <= 0) return
    const timer = setTimeout(() => setSeconds((value) => value - 1), 1000)
    return () => clearTimeout(timer)
  }, [seconds])
  const send = async () => {
    setBusy(true)
    setError('')
    setToken('')
    setCode('')
    try {
      const result = await sendMiniSMS(phone, props.flowToken)
      setToken(result.challenge_token)
      setSeconds(result.retry_after)
    } catch (error) {
      setError(smsErrorKey(error))
    } finally {
      setBusy(false)
    }
  }
  const login = async () => {
    setBusy(true)
    setError('')
    try {
      props.onLogin(await loginWithMiniSMS(token, code, props.flowToken))
    } catch (error) {
      setError(smsErrorKey(error))
    } finally {
      setBusy(false)
    }
  }
  return (
    <View className='profile-card profile-form'>
      <Text className='profile-card__label'>{t('phone.login')}</Text>
      {!props.flowToken ? (
        <Input
          className='profile-input'
          type='text'
          maxlength={16}
          value={phone}
          placeholder={t('phone.number')}
          disabled={busy}
          onInput={(event) => {
            setPhone(event.detail.value)
            setToken('')
            setCode('')
          }}
        />
      ) : null}
      <Button
        className='profile-button profile-button--secondary'
        disabled={busy || seconds > 0 || (!props.flowToken && !phone.trim())}
        onClick={send}
      >
        {seconds ? t('phone.resend', { seconds }) : t('phone.send')}
      </Button>
      <Input
        className='profile-input'
        type='number'
        maxlength={6}
        value={code}
        placeholder={t('phone.code')}
        disabled={busy || !token}
        onInput={(event) => setCode(event.detail.value)}
      />
      {error ? <Text className='profile-card__error'>{t(error)}</Text> : null}
      <Button
        className='profile-button'
        disabled={busy || !token || !/^\d{6}$/.test(code)}
        onClick={login}
      >
        {busy ? t('auth.busy') : t('auth.accountLoginSubmit')}
      </Button>
      <Button className='profile-link' disabled={busy} onClick={props.onCancel}>
        {t('auth.accountLoginBack')}
      </Button>
    </View>
  )
}
