/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Turnstile } from '@/components/turnstile'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import { useTurnstile } from '@/features/auth/hooks/use-turnstile'

import { loginWithSMS, phoneErrorMessage, sendLoginSMS } from '../api'
import { useSmsCountdown } from '../use-sms-countdown'

export function SmsLoginForm(props: {
  flowToken?: string
  redirectTo?: string
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const { handleLoginSuccess } = useAuthRedirect()
  const turnstile = useTurnstile()
  const countdown = useSmsCountdown()
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [widgetKey, setWidgetKey] = useState(0)

  const send = async () => {
    if (!props.flowToken && !turnstile.validateTurnstile()) return
    setBusy(true)
    setError('')
    setToken('')
    try {
      const challenge = await sendLoginSMS({
        phone: props.flowToken ? undefined : phone,
        flow_token: props.flowToken,
        turnstile: turnstile.turnstileToken,
      })
      setToken(challenge.challenge_token)
      setCode('')
      countdown.start(challenge.retry_after)
    } catch (error) {
      setError(phoneErrorMessage(error, t))
    } finally {
      setBusy(false)
      turnstile.setTurnstileToken('')
      setWidgetKey((value) => value + 1)
    }
  }

  const submit = async () => {
    setBusy(true)
    setError('')
    try {
      await handleLoginSuccess(
        await loginWithSMS(token, code, props.flowToken),
        props.redirectTo
      )
    } catch (error) {
      setError(phoneErrorMessage(error, t))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className='space-y-4' aria-label={t('SMS sign-in')}>
      {props.flowToken ? (
        <p className='text-muted-foreground text-sm'>
          {t('A code will be sent to your currently verified phone number.')}
        </p>
      ) : (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('Sign in with a phone number already verified on your account.')}
          </p>
          <Label className='grid gap-2'>
            {t('Phone Number')}
            <Input
              type='tel'
              autoComplete='tel'
              value={phone}
              disabled={busy || props.disabled}
              onChange={(event) => {
                setPhone(event.target.value)
                setToken('')
                setCode('')
              }}
            />
          </Label>
        </>
      )}
      {!props.flowToken && turnstile.isTurnstileEnabled ? (
        <Turnstile
          key={widgetKey}
          siteKey={turnstile.turnstileSiteKey}
          onVerify={turnstile.setTurnstileToken}
          onExpire={() => turnstile.setTurnstileToken('')}
        />
      ) : null}
      <Button
        type='button'
        variant='outline'
        disabled={
          busy ||
          props.disabled ||
          countdown.seconds > 0 ||
          (!props.flowToken && !phone.trim())
        }
        onClick={send}
      >
        {countdown.seconds > 0
          ? t('Resend in {{seconds}}s', { seconds: countdown.seconds })
          : t('Send SMS code')}
      </Button>
      <Label className='grid gap-2'>
        {t('SMS verification code')}
        <Input
          inputMode='numeric'
          autoComplete='one-time-code'
          maxLength={6}
          value={code}
          disabled={busy || props.disabled || !token}
          onChange={(event) => setCode(event.target.value)}
        />
      </Label>
      {error ? (
        <p role='alert' className='text-destructive text-sm'>
          {error}
        </p>
      ) : null}
      <Button
        type='button'
        className='w-full'
        disabled={busy || props.disabled || !token || !/^\d{6}$/.test(code)}
        onClick={submit}
      >
        {busy ? t('Verifying...') : t('Verify and Sign In')}
      </Button>
    </section>
  )
}
