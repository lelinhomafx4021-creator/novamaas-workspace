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
import { Link, useRouteContext, useSearch } from '@tanstack/react-router'
import { isAxiosError } from 'axios'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useAdminManualAccess } from '@/features/admin-manual/use-admin-manual-access'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { DocumentationPage, type APIDocument } from './page'

export function APIDocs() {
  const { t } = useTranslation()
  const { article = 'quickstart' } = useSearch({ from: '/docs/' })
  const { access } = useRouteContext({ from: '/docs/' })
  const manual = useAdminManualAccess()
  const userID = useAuthStore((state) => state.auth.user?.id)
  const sessionID = useAuthStore((state) => state.auth.session?.sid)
  const allowed = access.enabled && (!access.requireAuth || !!userID)
  const docs = useQuery({
    queryKey: ['api-docs', userID ?? null, sessionID ?? null],
    queryFn: async () => {
      const response = await api.get<{ success: boolean; data: APIDocument[] }>(
        '/api/docs',
        { skipErrorHandler: true }
      )
      return response.data.data
    },
    enabled: allowed,
    gcTime: 0,
    retry: false,
  })
  let status: ReactNode
  if (!allowed || docs.isError) {
    status = (
      <div role='alert' className='space-y-4 rounded-xl border p-6'>
        <h1 className='text-xl font-semibold'>
          {t('Documentation unavailable')}
        </h1>
        <p className='text-muted-foreground text-sm'>
          {!allowed
            ? t('The documentation may be disabled or require login.')
            : t('Request failed')}
          {allowed &&
            isAxiosError(docs.error) &&
            docs.error.response &&
            ` (${docs.error.response.status})`}
        </p>
        <Button
          variant='outline'
          onClick={() => void docs.refetch()}
          disabled={!allowed}
        >
          {t('Retry')}
        </Button>
      </div>
    )
  } else if (docs.isPending) {
    status = (
      <div role='status' aria-label={t('Loading')} className='space-y-4'>
        <Skeleton className='h-10 w-2/3' />
        <Skeleton className='h-64 w-full' />
      </div>
    )
  } else if (!docs.data?.some((doc) => doc.id === article)) {
    status = (
      <div role='alert' className='space-y-4'>
        <h1 className='text-xl font-semibold'>{t('Document not found')}</h1>
        <Link to='/docs' search={{ article: 'quickstart' }}>
          {t('Quick start')}
        </Link>
      </div>
    )
  }
  return (
    <DocumentationPage
      kind='api'
      manualAvailable={manual.available}
      documents={allowed && !docs.isError ? (docs.data ?? []) : []}
      article={article}
      status={status}
    />
  )
}
