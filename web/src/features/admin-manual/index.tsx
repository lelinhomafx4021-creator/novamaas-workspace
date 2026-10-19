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
import { useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { DocumentationPage } from '@/features/api-docs/page'
import { NotFoundError } from '@/features/errors/not-found-error'

import { adminManualDocumentOptions } from './api'
import { useAdminManualAccess } from './use-admin-manual-access'

export function AdminManual() {
  const { t } = useTranslation()
  const { article } = useSearch({ from: '/admin-manual/' })
  const access = useAdminManualAccess()
  const documents = useQuery({
    ...adminManualDocumentOptions(access.userID, access.sessionID, access.role),
    enabled: access.available,
  })
  const loading = (
    <div role='status' aria-label={t('Loading')} className='space-y-4'>
      <Skeleton className='h-10 w-2/3' />
      <Skeleton className='h-64 w-full' />
    </div>
  )
  if (access.pending) return loading
  if (!access.available || documents.isError) {
    return <NotFoundError />
  }
  if (
    !documents.isPending &&
    (!documents.data?.length ||
      (article && !documents.data.some((document) => document.id === article)))
  ) {
    return <NotFoundError />
  }

  return (
    <DocumentationPage
      kind='admin'
      documents={documents.data ?? []}
      article={article}
      status={documents.isPending ? loading : undefined}
    />
  )
}
