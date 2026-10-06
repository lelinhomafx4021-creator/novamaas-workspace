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

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { verify as verifySecurityMethod } from '@/features/auth/secure-verification/api'

import {
  phoneErrorMessage,
  sendSecuritySMS,
  verifyAccountSecurity,
} from '../api'
import { useSmsCountdown } from '../use-sms-countdown'

export function SecurityVerificationForm(props: {
  scope: 'phone.manage' | 'wechat.manage'
  allowSMS?: boolean
  onVerified: (proof: string) => void
}) {
  const { t } = useTranslation()
  const [method, setMethod] = useState<'password' | 'sms' | '2fa'>('password')
  const [password, setPassword] = useState('')
  const [twoFactor, setTwoFactor] = useState('')
  const [code, setCode] = useState('')
  const [challenge, setChallenge] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const countdown = useSmsCountdown()

  const sendCode = async () => {
    setBusy(true)
    setError('')
    try {
      const data = await sendSecuritySMS()
      setChallenge(data.challenge_token)
      setCode('')
      countdown.start(data.retry_after)
    } catch (error) {
      setError(phoneErrorMessage(error, t))
    } finally {
      setBusy(false)
    }
  }

  const verify = async () => {
    setBusy(true)
    setError('')
    try {
      const proof =
        method === '2fa'
          ? (await verifySecurityMethod('2fa', props.scope, twoFactor))
              .proof_token
          : await verifyAccountSecurity({
              scope: props.scope,
              password: method === 'password' ? password : undefined,
              two_factor_code: method === 'password' ? twoFactor : undefined,
              challenge_token: method === 'sms' ? challenge : undefined,
              code: method === 'sms' ? code : undefined,
            })
      setPassword('')
      setTwoFactor('')
      setCode('')
      props.onVerified(proof)
    } catch (error) {
      setError(phoneErrorMessage(error, t))
    } finally {
      setBusy(false)
    }
  }

  let missingInput = !challenge || !/^\d{6}$/.test(code)
  if (method === 'password') missingInput = !password
  if (method === '2fa') missingInput = !twoFactor

  return (
    <div className='space-y-3'>
      <p className='text-muted-foreground text-sm'>
        {t('Verify your current account before changing bindings.')}
      </p>
      <div className='flex flex-wrap gap-2'>
        <Button
          type='button'
          size='sm'
          variant={method === 'password' ? 'default' : 'outline'}
          aria-pressed={method === 'password'}
          disabled={busy}
          onClick={() => {
            setMethod('password')
            setError('')
          }}
        >
          {t('Password')}
        </Button>
        {props.allowSMS !== false ? (
          <Button
            type='button'
            size='sm'
            variant={method === 'sms' ? 'default' : 'outline'}
            aria-pressed={method === 'sms'}
            disabled={busy}
            onClick={() => {
              setMethod('sms')
              setError('')
            }}
          >
            {t('Verify current phone')}
          </Button>
        ) : null}
        <Button
          type='button'
          size='sm'
          variant={method === '2fa' ? 'default' : 'outline'}
          disabled={busy}
          onClick={() => {
            setMethod('2fa')
            setError('')
          }}
        >
          {t('Authenticator')}
        </Button>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={busy}
          onClick={async () => {
            setBusy(true)
            setError('')
            try {
              props.onVerified(
                (await verifySecurityMethod('passkey', props.scope)).proof_token
              )
            } catch (error) {
              setError(phoneErrorMessage(error, t))
            } finally {
              setBusy(false)
            }
          }}
        >
          {t('Passkey')}
        </Button>
      </div>
      {method === 'password' ? (
        <>
          <Label className='grid gap-2'>
            {t('Current account password')}
            <Input
              type='password'
              autoComplete='current-password'
              value={password}
              disabled={busy}
              onChange={(event) => setPassword(event.target.value)}
            />
          </Label>
          <Label className='grid gap-2'>
            {t('Authenticator or backup code, if enabled')}
            <Input
              autoComplete='one-time-code'
              value={twoFactor}
              disabled={busy}
              onChange={(event) => setTwoFactor(event.target.value)}
            />
          </Label>
        </>
      ) : null}
      {method === '2fa' ? (
        <Label className='grid gap-2'>
          {t('Authenticator or backup code, if enabled')}
          <Input
            autoComplete='one-time-code'
            value={twoFactor}
            disabled={busy}
            onChange={(event) => setTwoFactor(event.target.value)}
          />
        </Label>
      ) : null}
      {method === 'sms' ? (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('A code will be sent to your currently verified phone number.')}
          </p>
          <Button
            type='button'
            variant='outline'
            disabled={busy || countdown.seconds > 0}
            onClick={sendCode}
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
              disabled={busy}
              onChange={(event) => setCode(event.target.value)}
            />
          </Label>
        </>
      ) : null}
      {error ? (
        <p role='alert' className='text-destructive text-sm'>
          {error}
        </p>
      ) : null}
      <Button type='button' disabled={busy || missingInput} onClick={verify}>
        {busy ? t('Verifying...') : t('Verify')}
      </Button>
    </div>
  )
}
