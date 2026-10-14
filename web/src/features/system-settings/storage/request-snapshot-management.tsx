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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'

import {
  getRequestSnapshotStoragePolicy,
  updateRequestSnapshotStoragePolicy,
} from './api'
import { RequestSnapshotMaintenance } from './request-snapshot-maintenance'
import { RequestSnapshotPolicyForm } from './request-snapshot-policy-form'
import {
  requestSnapshotPolicyToInput,
  type RequestSnapshotPolicyFormValues,
} from './request-snapshot-schemas'
import { assertStorageSuccess, storageErrorMessage } from './storage-utils'
import type { StorageProfile } from './types'

const POLICY_QUERY_KEY = ['storage', 'request-snapshots', 'policy'] as const

export function RequestSnapshotManagement(props: {
  profiles: StorageProfile[]
  profilesReady: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const policyQuery = useQuery({
    queryKey: POLICY_QUERY_KEY,
    queryFn: async () =>
      assertStorageSuccess(await getRequestSnapshotStoragePolicy()),
  })
  const saveMutation = useMutation({
    mutationFn: async (values: RequestSnapshotPolicyFormValues) =>
      assertStorageSuccess(
        await updateRequestSnapshotStoragePolicy(
          requestSnapshotPolicyToInput(
            values,
            policyQuery.data?.allowed_mime_types || 'application/json'
          )
        )
      ),
    onSuccess: (policy) => {
      queryClient.setQueryData(POLICY_QUERY_KEY, policy)
      toast.success(t('Storage policy saved'))
    },
    onError: (error) => toast.error(storageErrorMessage(error)),
  })

  return (
    <>
      {(policyQuery.isPending || !props.profilesReady) && (
        <Skeleton className='h-80 w-full' />
      )}
      {policyQuery.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Snapshot policy unavailable')}</AlertTitle>
          <AlertDescription>
            {storageErrorMessage(policyQuery.error)}
          </AlertDescription>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void policyQuery.refetch()}
          >
            {t('Retry')}
          </Button>
        </Alert>
      )}
      {policyQuery.data && props.profilesReady && (
        <RequestSnapshotPolicyForm
          key={JSON.stringify(policyQuery.data)}
          policy={policyQuery.data}
          profiles={props.profiles}
          isSaving={saveMutation.isPending}
          onSave={(values) => saveMutation.mutate(values)}
        />
      )}
      <RequestSnapshotMaintenance
        migrationEnabled={
          policyQuery.data?.enabled === true && !policyQuery.isError
        }
      />
    </>
  )
}
