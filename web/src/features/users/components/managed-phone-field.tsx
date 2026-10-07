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
import { useEffect, useRef, useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { SecurityConfirmationDialog } from '@/features/auth/secure-verification'
import {
  getPhoneStatus,
  phoneErrorMessage,
  sendUserMutationSMS,
} from '@/features/phone/api'
import { PhoneNumberInput } from '@/features/phone/components/phone-number-input'
import { SecurityVerificationForm } from '@/features/phone/components/security-verification-form'
import {
  canonicalMobilePhone,
  nationalPhoneNumber,
} from '@/features/phone/phone-number'
import { useSmsCountdown } from '@/features/phone/use-sms-countdown'

import {
  EMPTY_PHONE_VERIFICATION,
  type ManagedPhoneVerification,
  type UserFormValues,
} from '../lib/user-form'

export function ManagedPhoneField(props: {
  userId?: number
  disabled: boolean
  verification: ManagedPhoneVerification
  onVerificationChange: (value: ManagedPhoneVerification) => void
}) {
  const { t } = useTranslation()
  const form = useFormContext<UserFormValues>()
  const phone = form.watch('phone') || ''
  const username = form.watch('username') || ''
  const status = useQuery({
    queryKey: ['phone-binding', props.userId ?? 'self'],
    queryFn: () => getPhoneStatus(props.userId),
  })
  const [checkingSecurity, setCheckingSecurity] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const countdown = useSmsCountdown()
  const draftRequest = useRef<AbortController | null>(null)
  useEffect(() => {
    const pending = new AbortController()
    draftRequest.current = pending
    return () => pending.abort()
  }, [phone, username, props.userId])
  const bound =
    !!props.userId &&
    status.data?.verified?.phone === canonicalMobilePhone(phone)
  const canSend =
    /^1[3-9]\d{9}$/.test(nationalPhoneNumber(phone)) &&
    !bound &&
    !!username.trim() &&
    !!status.data?.sms_enabled
  const send = async (proof: string) => {
    const currentDraft = draftRequest.current
    const destination = canonicalMobilePhone(phone)
    const draftUsername = username.trim()
    setCheckingSecurity(false)
    setSending(true)
    setError('')
    props.onVerificationChange(EMPTY_PHONE_VERIFICATION)
    try {
      const challenge = await sendUserMutationSMS(
        { phone: destination, user_id: props.userId, username: draftUsername },
        proof
      )
      countdown.start(challenge.retry_after)
      if (currentDraft && !currentDraft.signal.aborted) {
        props.onVerificationChange({
          challenge_token: challenge.challenge_token,
          code: '',
          security_proof: proof,
          phone: destination,
          username: draftUsername,
        })
      }
    } catch (error) {
      if (currentDraft && !currentDraft.signal.aborted) {
        setError(phoneErrorMessage(error, t))
      }
    } finally {
      setSending(false)
    }
  }
  let bindingLabel = t('Not bound')
  if (phone) bindingLabel = t('Phone verification pending')
  if (bound) bindingLabel = t('Bound')
  return (
    <>
      <FormField
        control={form.control}
        name='phone'
        render={({ field }) => (
          <FormItem>
            <div className='flex items-center justify-between gap-2'>
              <FormLabel>{t('Phone Number')}</FormLabel>
              <StatusBadge
                label={bindingLabel}
                variant={bound ? 'success' : 'neutral'}
                copyable={false}
              />
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <div className='min-w-0 flex-1'>
                <FormControl>
                  <PhoneNumberInput
                    {...field}
                    maxLength={32}
                    placeholder={t('Enter phone number')}
                    disabled={props.disabled || sending}
                    onChange={(event) => {
                      field.onChange(nationalPhoneNumber(event.target.value))
                      form.clearErrors('phone')
                      props.onVerificationChange(EMPTY_PHONE_VERIFICATION)
                      setError('')
                    }}
                  />
                </FormControl>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={
                  props.disabled || sending || !canSend || countdown.seconds > 0
                }
                onClick={() => {
                  setCheckingSecurity(true)
                  setError('')
                }}
              >
                {countdown.seconds > 0
                  ? t('Resend in {{seconds}}s', { seconds: countdown.seconds })
                  : t('Send SMS code')}
              </Button>
            </div>
            <FormDescription>
              {t(
                'Changed phone numbers must be verified when saving. Unchanged bindings do not require another code.'
              )}
            </FormDescription>
            {!status.isPending &&
            !status.isError &&
            !status.data?.sms_enabled ? (
              <p className='text-muted-foreground text-xs'>
                {t('SMS binding is not configured yet.')}
              </p>
            ) : null}
            {status.isPending ? (
              <p className='text-muted-foreground text-xs'>{t('Loading...')}</p>
            ) : null}
            {status.isError ? (
              <div role='alert'>
                <p>{t('Failed to load phone binding.')}</p>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => status.refetch()}
                >
                  {t('Retry')}
                </Button>
              </div>
            ) : null}
            {props.verification.challenge_token ? (
              <Label className='grid gap-2'>
                {t('SMS verification code')}
                <Input
                  value={props.verification.code}
                  inputMode='numeric'
                  autoComplete='one-time-code'
                  maxLength={6}
                  disabled={props.disabled || sending}
                  onChange={(event) => {
                    form.clearErrors('phone')
                    props.onVerificationChange({
                      ...props.verification,
                      code: event.target.value,
                    })
                  }}
                />
              </Label>
            ) : null}
            {error ? (
              <p role='alert' className='text-destructive text-sm'>
                {error}
              </p>
            ) : null}
            <FormMessage />
          </FormItem>
        )}
      />
      <SecurityConfirmationDialog
        open={checkingSecurity}
        onOpenChange={setCheckingSecurity}
        title={t('Verify')}
        description={t('Verify your current account before changing bindings.')}
        contentClassName='sm:max-w-md'
      >
        <SecurityVerificationForm
          scope='phone.manage'
          allowSMS={false}
          onVerified={(proof) => {
            void send(proof)
          }}
        />
      </SecurityConfirmationDialog>
    </>
  )
}
