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
import { MessageCircle } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { SecurityConfirmationDialog } from '@/features/auth/secure-verification'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { SecurityVerificationForm } from './security-verification-form'
import { WeChatAvatar } from './wechat-avatar'

interface WeChatBinding {
  app_id: string
  openid?: string
  unionid?: string
  nickname: string
  has_avatar: boolean
  bound_at: string
  updated_at?: string
  last_login_at?: string
}

export function WeChatBindingCard(props: {
  userId?: number
  showDetails?: boolean
  onChanged?: () => void
}) {
  const { t } = useTranslation()
  const base = props.userId
    ? `/api/user/${props.userId}/wechat-miniapp`
    : '/api/user/self/wechat-miniapp'
  const query = useQuery({
    queryKey: ['wechat-miniapp-binding', props.userId ?? 'self'],
    queryFn: async () =>
      (await api.get<{ data: WeChatBinding[] }>(base)).data.data,
  })
  const [open, setOpen] = useState(false)
  const [unlinking, setUnlinking] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const unbind = async (proof: string) => {
    setBusy(true)
    setError('')
    try {
      await api.post(
        `${base}/unbind`,
        { app_id: unlinking },
        { headers: { 'X-Security-Proof': proof } }
      )
      setUnlinking('')
      if (!props.userId) {
        useAuthStore.getState().auth.reset()
        window.location.assign('/sign-in')
        return
      }
      await query.refetch()
      props.onChanged?.()
    } catch (error) {
      const code = (error as { response?: { data?: { code?: string } } })
        .response?.data?.code
      setError(
        code === 'MINI_AUTH_LAST_CREDENTIAL'
          ? t('Add another sign-in method before disconnecting WeChat.')
          : t('Could not disconnect WeChat. Verify your account and try again.')
      )
    } finally {
      setBusy(false)
    }
  }
  let summary = t('Not bound')
  if (query.data?.length) {
    summary = query.data
      .map((binding) => binding.nickname || t('Bound'))
      .join(', ')
  }
  if (query.isError) summary = t('Failed to load WeChat binding.')
  if (query.isPending) summary = t('Loading...')
  return (
    <>
      <section
        className='flex flex-col gap-3 rounded-lg border p-2.5 sm:p-3'
        aria-label={t('WeChat mini program binding')}
      >
        <div className='flex w-full items-center justify-between gap-3'>
          <div className='flex min-w-0 items-center gap-2.5 sm:gap-3'>
            <div className='bg-muted shrink-0 rounded-md p-1.5 sm:p-2'>
              <MessageCircle className='h-4 w-4' />
            </div>
            <div className='min-w-0'>
              <div className='flex items-center gap-1.5'>
                <p className='text-sm font-medium'>
                  {t('WeChat mini program binding')}
                </p>
                {query.data?.length ? (
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
          {query.isError ? (
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='h-7 shrink-0 px-2.5 text-xs'
              onClick={() => query.refetch()}
            >
              {t('Retry')}
            </Button>
          ) : (
            <Button
              type='button'
              variant='outline'
              size='sm'
              className='h-7 shrink-0 px-2.5 text-xs'
              disabled={query.isPending}
              aria-label={t('Manage WeChat binding')}
              onClick={() => {
                setOpen(true)
                setError('')
              }}
            >
              {query.data?.length ? t('Manage') : t('Bind')}
            </Button>
          )}
        </div>
        {props.showDetails && props.userId
          ? query.data?.map((binding) => (
              <div
                key={binding.app_id}
                className='space-y-2 border-t pt-3 text-xs'
              >
                <div className='flex items-center gap-3'>
                  <WeChatAvatar binding={binding} base={base} />
                  <p className='min-w-0 font-medium break-words'>
                    {binding.nickname || t('WeChat profile not provided')}
                  </p>
                </div>
                <p className='break-all'>AppID: {binding.app_id}</p>
                {binding.openid ? (
                  <p className='break-all'>OpenID: {binding.openid}</p>
                ) : null}
                {binding.unionid ? (
                  <p className='break-all'>UnionID: {binding.unionid}</p>
                ) : null}
                <p>
                  {t('Bound at')}: {new Date(binding.bound_at).toLocaleString()}
                </p>
                {binding.last_login_at ? (
                  <p>
                    {t('Last login')}:{' '}
                    {new Date(binding.last_login_at).toLocaleString()}
                  </p>
                ) : null}
              </div>
            ))
          : null}
      </section>
      <Dialog
        open={open && !unlinking}
        onOpenChange={(value) => {
          if (!busy) {
            setOpen(value)
            setUnlinking('')
            setError('')
          }
        }}
        title={t('WeChat mini program binding')}
        description={t(
          'Confirm the account link in the mini program using your verified phone number.'
        )}
        contentClassName='sm:max-w-md'
        showCloseButton={!busy}
      >
        <div className='space-y-4'>
          {query.data?.length === 0 ? (
            <p className='text-muted-foreground text-sm'>
              {t(
                'Open the mini program and choose WeChat phone sign-in to link your account.'
              )}
            </p>
          ) : null}
          {query.data?.map((binding) => (
            <div key={binding.app_id} className='space-y-2 text-sm'>
              <div className='flex items-center gap-3'>
                <WeChatAvatar binding={binding} base={base} />
                <span>
                  {binding.nickname || t('WeChat profile not provided')}
                </span>
              </div>
              {props.userId ? <p>AppID: {binding.app_id}</p> : null}
              {binding.openid ? (
                <p className='break-all'>OpenID: {binding.openid}</p>
              ) : null}
              {binding.unionid ? (
                <p className='break-all'>UnionID: {binding.unionid}</p>
              ) : null}
              <p>
                {t('Bound at')}: {new Date(binding.bound_at).toLocaleString()}
              </p>
              {binding.last_login_at ? (
                <p>
                  {t('Last login')}:{' '}
                  {new Date(binding.last_login_at).toLocaleString()}
                </p>
              ) : null}
              <Button
                type='button'
                variant='outline'
                disabled={busy}
                onClick={() => {
                  setUnlinking(binding.app_id)
                  setError('')
                }}
              >
                {t('Disconnect WeChat')}
              </Button>
            </div>
          ))}
          {error ? (
            <p role='alert' className='text-destructive text-sm'>
              {error}
            </p>
          ) : null}
        </div>
      </Dialog>
      <SecurityConfirmationDialog
        open={Boolean(unlinking)}
        onOpenChange={(value) => {
          if (!busy && !value) {
            setUnlinking('')
            setError('')
          }
        }}
        title={t('Disconnect WeChat')}
        description={t(
          'Disconnecting WeChat signs this account out on all devices.'
        )}
        showCloseButton={!busy}
      >
        <SecurityVerificationForm
          scope='wechat.manage'
          allowSMS={!props.userId}
          onVerified={(proof) => {
            void unbind(proof)
          }}
        />
        {error ? (
          <p role='alert' className='text-destructive text-sm'>
            {error}
          </p>
        ) : null}
        <Button
          type='button'
          variant='ghost'
          disabled={busy}
          onClick={() => {
            setUnlinking('')
            setError('')
          }}
        >
          {t('Cancel')}
        </Button>
      </SecurityConfirmationDialog>
    </>
  )
}
