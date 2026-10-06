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
import { useQuery } from '@tanstack/react-query'
import { Smartphone } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import {
  confirmPhoneBinding,
  getPhoneStatus,
  phoneErrorMessage,
  sendBindingSMS,
} from '../api'
import { formatMobilePhone, nationalPhoneNumber } from '../phone-number'
import { useSmsCountdown } from '../use-sms-countdown'
import { PhoneNumberInput } from './phone-number-input'
import { SecurityVerificationForm } from './security-verification-form'

export function PhoneBindingCard(props: {
  userId?: number
  onChanged?: () => void
}) {
  const { t } = useTranslation()
  const phoneInputId = useId()
  const status = useQuery({
    queryKey: ['phone-binding', props.userId ?? 'self'],
    queryFn: () => getPhoneStatus(props.userId),
  })
  const [editing, setEditing] = useState(false)
  const [proof, setProof] = useState('')
  const [phone, setPhone] = useState('')
  const [token, setToken] = useState('')
  const [code, setCode] = useState('')
  const countdown = useSmsCountdown()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const send = async () => {
    setBusy(true)
    setError('')
    setToken('')
    try {
      const challenge = await sendBindingSMS(phone, proof, props.userId)
      setToken(challenge.challenge_token)
      setCode('')
      countdown.start(challenge.retry_after)
    } catch (error) {
      setError(phoneErrorMessage(error, t))
      setProof('')
    } finally {
      setBusy(false)
    }
  }

  const confirm = async () => {
    setBusy(true)
    setError('')
    try {
      await confirmPhoneBinding(token, code, proof, props.userId)
      setEditing(false)
      setProof('')
      setToken('')
      setCode('')
      setPhone('')
      await status.refetch()
      props.onChanged?.()
    } catch (error) {
      setError(phoneErrorMessage(error, t))
    } finally {
      setBusy(false)
    }
  }

  const close = () => {
    setEditing(false)
    setProof('')
    setToken('')
    setCode('')
    setError('')
  }
  const rawPhone = status.data?.verified?.phone || status.data?.phone
  const value = rawPhone ? formatMobilePhone(rawPhone) : t('Not bound')
  let summary = value
  if (status.data?.phone && !status.data.verified) {
    summary = `${value} · ${t('Phone verification pending')}`
  }
  if (!status.data?.sms_enabled) {
    summary = `${value} · ${t('SMS binding is not configured yet.')}`
  }
  if (status.isError) summary = t('Failed to load phone binding.')
  if (status.isPending) summary = t('Loading...')

  return (
    <>
      <section
        className='flex items-center justify-between gap-2.5 rounded-lg border p-2.5 sm:gap-3 sm:p-3'
        aria-label={t('Phone binding')}
      >
        <div className='flex min-w-0 items-center gap-2.5 sm:gap-3'>
          <div className='bg-muted shrink-0 rounded-md p-1.5 sm:p-2'>
            <Smartphone className='h-4 w-4' />
          </div>
          <div className='min-w-0'>
            <div className='flex items-center gap-1.5'>
              <p className='text-sm font-medium'>{t('Phone Number')}</p>
              {status.data?.verified ? (
                <StatusBadge
                  label={t('Bound')}
                  variant='success'
                  copyable={false}
                />
              ) : null}
            </div>
            <p
              className='text-muted-foreground truncate text-xs'
              title={summary}
            >
              {summary}
            </p>
          </div>
        </div>
        {status.isError ? (
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-7 shrink-0 px-2.5 text-xs'
            onClick={() => status.refetch()}
          >
            {t('Retry')}
          </Button>
        ) : (
          <Button
            type='button'
            variant='outline'
            size='sm'
            className='h-7 shrink-0 px-2.5 text-xs'
            aria-label={
              status.data?.verified
                ? t('Change phone number')
                : t('Verify phone number')
            }
            disabled={!status.data?.sms_enabled}
            onClick={() => {
              setEditing(true)
              setPhone(
                nationalPhoneNumber(
                  status.data?.verified?.phone || status.data?.phone || ''
                )
              )
              setError('')
            }}
          >
            {status.data?.verified ? t('Change') : t('Bind')}
          </Button>
        )}
      </section>
      <Dialog
        open={editing}
        onOpenChange={(open) => {
          if (!busy && !open) close()
        }}
        title={
          status.data?.verified
            ? t('Change phone number')
            : t('Verify phone number')
        }
        description={t('Verify your account before binding a phone number.')}
        contentClassName='sm:max-w-md'
        showCloseButton={!busy}
      >
        <div className='space-y-3'>
          {status.data?.verified ? (
            <p className='text-muted-foreground text-xs'>
              {t('Verified at')}:{' '}
              {new Date(status.data.verified.verified_at).toLocaleString()}
            </p>
          ) : null}
          {!proof ? (
            <SecurityVerificationForm
              scope='phone.manage'
              allowSMS={!props.userId}
              onVerified={setProof}
            />
          ) : (
            <div className='space-y-3'>
              <div className='grid gap-2'>
                <Label htmlFor={phoneInputId}>
                  {t('New phone number (+86)')}
                </Label>
                <PhoneNumberInput
                  id={phoneInputId}
                  value={phone}
                  disabled={busy}
                  onChange={(event) => {
                    setPhone(nationalPhoneNumber(event.target.value))
                    setToken('')
                    setCode('')
                  }}
                />
              </div>
              <Button
                type='button'
                variant='outline'
                disabled={busy || !phone.trim() || countdown.seconds > 0}
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
                  disabled={busy || !token}
                  onChange={(event) => setCode(event.target.value)}
                />
              </Label>
              <Button
                type='button'
                disabled={busy || !token || !/^\d{6}$/.test(code)}
                onClick={confirm}
              >
                {t('Confirm phone binding')}
              </Button>
            </div>
          )}
          <Button type='button' variant='ghost' disabled={busy} onClick={close}>
            {t('Cancel')}
          </Button>
          {error ? (
            <p role='alert' className='text-destructive text-sm'>
              {error}
            </p>
          ) : null}
        </div>
      </Dialog>
    </>
  )
}
