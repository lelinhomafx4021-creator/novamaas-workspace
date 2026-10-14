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
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'

import {
  getRequestSnapshotStatus,
  startRequestSnapshotCleanup,
  startRequestSnapshotMigration,
} from './api'
import { RequestSnapshotNodeStatus } from './request-snapshot-status'
import { assertStorageSuccess, storageErrorMessage } from './storage-utils'
import type { RequestSnapshotStatus } from './types'

const SNAPSHOT_STATUS_QUERY_KEY = [
  'storage',
  'request-snapshots',
  'status',
] as const

export function RequestSnapshotMaintenance(props: {
  migrationEnabled: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const titleId = useId()
  const [action, setAction] = useState<'migrate' | 'cleanup' | null>(null)
  const statusQuery = useQuery({
    queryKey: SNAPSHOT_STATUS_QUERY_KEY,
    queryFn: async () => assertStorageSuccess(await getRequestSnapshotStatus()),
    retry: false,
    refetchInterval: (query) =>
      query.state.data?.migration.status === 'running' ||
      (query.state.data?.pending_files ?? 0) > 0
        ? 2000
        : 10000,
  })
  const jobMutation = useMutation({
    mutationFn: async (mode: 'migrate' | 'cleanup') => {
      const response =
        mode === 'migrate'
          ? await startRequestSnapshotMigration()
          : await startRequestSnapshotCleanup()
      return assertStorageSuccess(response)
    },
    onSuccess: (job) => {
      queryClient.setQueryData<RequestSnapshotStatus>(
        SNAPSHOT_STATUS_QUERY_KEY,
        (previous) => (previous ? { ...previous, migration: job } : previous)
      )
      toast.success(t('Snapshot maintenance job started'))
    },
    onError: (error) => toast.error(storageErrorMessage(error)),
  })
  const busy =
    jobMutation.isPending || statusQuery.data?.migration.status === 'running'
  const controlsDisabled = busy || statusQuery.isPending || statusQuery.isError

  return (
    <>
      <Card role='region' aria-labelledby={titleId}>
        <CardHeader>
          <CardTitle id={titleId}>
            {t('Snapshot queue and maintenance')}
          </CardTitle>
          <CardDescription>
            {t(
              'Migrate historical snapshots to object storage before removing verified database copies.'
            )}
          </CardDescription>
          <CardAction>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={statusQuery.isFetching}
              onClick={() => void statusQuery.refetch()}
            >
              {t('Refresh')}
            </Button>
          </CardAction>
        </CardHeader>
        <CardContent>
          {statusQuery.isPending && <Skeleton className='h-48 w-full' />}
          {statusQuery.isError && (
            <Alert variant='destructive'>
              <AlertTitle>{t('Snapshot status unavailable')}</AlertTitle>
              <AlertDescription>
                {storageErrorMessage(statusQuery.error)}
                {statusQuery.data && (
                  <p>
                    {t(
                      'Status refresh failed. Showing the last received status.'
                    )}
                  </p>
                )}
              </AlertDescription>
            </Alert>
          )}
          {statusQuery.data && (
            <RequestSnapshotNodeStatus status={statusQuery.data} />
          )}
        </CardContent>
        <CardFooter className='mt-4 flex-wrap gap-2'>
          <Button
            type='button'
            variant='outline'
            disabled={controlsDisabled || !props.migrationEnabled}
            onClick={() => setAction('migrate')}
          >
            {t('Migrate historical snapshots')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={controlsDisabled}
            onClick={() => setAction('cleanup')}
          >
            {t('Remove verified database bodies')}
          </Button>
          {busy && <Spinner aria-label={t('Snapshot maintenance running')} />}
        </CardFooter>
      </Card>
      <AlertDialog
        open={action !== null}
        onOpenChange={(open) => !open && setAction(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {action === 'cleanup'
                ? t('Remove verified database bodies?')
                : t('Migrate historical snapshots?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {action === 'cleanup'
                ? t(
                    'Only database bodies with verified object storage copies are removed. Snapshots that could not be migrated remain in the database.'
                  )
                : t(
                    'Historical snapshots are uploaded and verified in the background. Database bodies remain until you run verified cleanup.'
                  )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={jobMutation.isPending}
              onClick={() => {
                if (action === null) return
                jobMutation.mutate(action)
                setAction(null)
              }}
            >
              {action === 'cleanup'
                ? t('Start verified cleanup')
                : t('Start migration')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
