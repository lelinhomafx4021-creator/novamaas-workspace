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
import { UserRound } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { GroupBadge } from '@/components/group-badge'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getPhoneStatus } from '@/features/phone/api'
import { WeChatAvatar } from '@/features/phone/components/wechat-avatar'
import { formatMobilePhone } from '@/features/phone/phone-number'
import { USER_ROLES } from '@/features/users/constants'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

interface WeChatAccount {
  app_id: string
  nickname: string
  has_avatar: boolean
  updated_at?: string
}

export function AccountSummaryPanel() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const phone = useQuery({
    queryKey: ['phone-status', user?.id],
    queryFn: () => getPhoneStatus(),
    enabled: Boolean(user),
    retry: false,
  })
  const wechat = useQuery({
    queryKey: ['wechat-bindings', user?.id],
    queryFn: async () => {
      const response = await api.get<{ data: WeChatAccount[] }>(
        '/api/user/self/wechat-miniapp'
      )
      return response.data.data
    },
    enabled: Boolean(user),
    retry: false,
  })
  const number = phone.data?.verified?.phone || phone.data?.phone
  const role = USER_ROLES[user?.role as keyof typeof USER_ROLES]
  let phoneValue: ReactNode = number
    ? formatMobilePhone(number)
    : t('Not bound')
  if (phone.isLoading) phoneValue = <Skeleton className='h-5 w-28' />
  else if (phone.isError) phoneValue = t('Unable to load account information')

  let wechatValue: ReactNode = user?.wechat_id
    ? t('WeChat account linked')
    : t('Not bound')
  if (wechat.isLoading) wechatValue = <Skeleton className='h-5 w-28' />
  else if (wechat.isError) {
    wechatValue = t('Unable to load account information')
  } else if (wechat.data?.length) {
    wechatValue = wechat.data.map((binding) => (
      <div key={binding.app_id} className='flex min-w-0 items-center gap-3'>
        <WeChatAvatar binding={binding} base='/api/user/self/wechat-miniapp' />
        <span className='min-w-0 font-medium break-words'>
          {binding.nickname || t('WeChat account linked')}
        </span>
      </div>
    ))
  }
  return (
    <section className='bg-card h-full min-w-0 overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <IconBadge tone='neutral' size='sm'>
          <UserRound />
        </IconBadge>
        <h3 className='text-sm font-semibold'>{t('Account information')}</h3>
      </div>
      <div className='p-4 sm:p-5'>
        <div className='mb-4 flex items-start justify-between gap-3 border-b pb-4'>
          <div className='min-w-0'>
            <p className='text-muted-foreground text-xs'>
              {t('Platform account')}
            </p>
            <p className='mt-1 text-lg font-semibold break-words'>
              {user?.display_name || user?.username || '—'}
            </p>
            {user?.display_name ? (
              <p className='text-muted-foreground text-sm break-all'>
                @{user.username}
              </p>
            ) : null}
          </div>
          {user?.group ? (
            <div className='shrink-0 space-y-1 text-right'>
              <p className='text-muted-foreground text-xs'>{t('Group')}</p>
              <GroupBadge group={user.group} />
            </div>
          ) : null}
        </div>
        <dl className='grid grid-cols-2 gap-x-4 gap-y-4 text-sm'>
          <div>
            <dt className='text-muted-foreground text-xs'>{t('User ID')}</dt>
            <dd className='mt-1 font-medium'>{user?.id ?? '—'}</dd>
          </div>
          <div>
            <dt className='text-muted-foreground text-xs'>{t('Role')}</dt>
            <dd className='mt-1 font-medium'>
              {role ? t(role.labelKey) : '—'}
            </dd>
          </div>
          <div className='col-span-2'>
            <dt className='text-muted-foreground text-xs'>
              {t('Phone number')}
            </dt>
            <dd className='mt-1 font-medium tabular-nums'>{phoneValue}</dd>
          </div>
          {user?.email ? (
            <div className='col-span-2'>
              <dt className='text-muted-foreground text-xs'>{t('Email')}</dt>
              <dd className='mt-1 break-all'>{user.email}</dd>
            </div>
          ) : null}
          <div className='col-span-2 border-t pt-4'>
            <dt className='text-muted-foreground mb-2 text-xs'>
              {t('WeChat account')}
            </dt>
            <dd className='space-y-3'>{wechatValue}</dd>
          </div>
        </dl>
      </div>
    </section>
  )
}
