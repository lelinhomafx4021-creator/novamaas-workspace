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
import {
  Delete02Icon,
  PencilEdit02Icon,
  SentIcon,
  WebhookIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { formatTimestampToDate } from '@/lib/format'

import {
  saveWebhookEndpoint,
  deleteWebhookEndpoint,
  listWebhookEndpoints,
  testWebhookEndpoint,
} from '../api'
import { useWebhookCapabilities } from '../hooks/use-webhook-capabilities'
import { assertWebhookSuccess, webhookErrorMessage } from '../lib/response'
import type {
  WebhookEndpoint,
  WebhookEndpointInput,
  WebhookEventType,
  WebhookProbeResult,
} from '../types'
import { WebhookEndpointForm } from './webhook-endpoint-form'
import { WebhookManualButton } from './webhook-manual-button'
import { WebhookTestResult } from './webhook-test-result'

const WEBHOOKS_QUERY_KEY = ['webhook-endpoints'] as const
const MAX_WEBHOOK_ENDPOINTS = 5

function eventLabelKey(eventType: WebhookEventType) {
  if (eventType === 'asset.active') return 'Asset becomes active'
  if (eventType === 'asset.failed') return 'Asset fails review'
  return 'Media task status changes'
}

export function WebhookDialog(props: {
  scope?: 'assets' | 'tasks'
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const capabilities = useWebhookCapabilities(props.open)
  const scope = props.scope ?? 'assets'
  const enabledEvents: WebhookEventType[] = []
  if (!capabilities.error && capabilities.data?.asset_library_enabled) {
    enabledEvents.push('asset.active', 'asset.failed')
  }
  if (!capabilities.error && capabilities.data?.media_tasks_enabled) {
    enabledEvents.push('task.status_changed')
  }
  const scopeEvents: WebhookEventType[] =
    scope === 'tasks'
      ? ['task.status_changed']
      : ['asset.active', 'asset.failed']
  const scopeEnabled =
    scope === 'tasks'
      ? enabledEvents.includes('task.status_changed')
      : enabledEvents.includes('asset.active')
  const [deleteTarget, setDeleteTarget] = useState<WebhookEndpoint | null>(null)
  const [testResult, setTestResult] = useState<{
    endpointId: string
    result: WebhookProbeResult
  } | null>(null)
  const [editTarget, setEditTarget] = useState<WebhookEndpoint | null>(null)
  const endpointsQuery = useQuery({
    queryKey: WEBHOOKS_QUERY_KEY,
    queryFn: async () => assertWebhookSuccess(await listWebhookEndpoints()),
    enabled: props.open,
  })
  const allEndpoints = endpointsQuery.data ?? []
  const endpoints = allEndpoints.filter((endpoint) =>
    endpoint.event_types.some((event) => scopeEvents.includes(event))
  )
  const saveMutation = useMutation({
    mutationFn: async (input: { values: WebhookEndpointInput; id?: string }) =>
      assertWebhookSuccess(await saveWebhookEndpoint(input.values, input.id)),
    onSuccess: async (_, input) => {
      setEditTarget(null)
      setTestResult(null)
      await queryClient.invalidateQueries({ queryKey: WEBHOOKS_QUERY_KEY })
      toast.success(
        input.id ? t('Webhook endpoint updated') : t('Webhook endpoint created')
      )
    },
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })
  const deleteMutation = useMutation({
    mutationFn: async (id: string) =>
      assertWebhookSuccess(await deleteWebhookEndpoint(id)),
    onSuccess: async () => {
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: WEBHOOKS_QUERY_KEY })
      toast.success(t('Webhook endpoint deleted'))
    },
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })
  const testMutation = useMutation({
    mutationFn: async (id: string) =>
      assertWebhookSuccess(await testWebhookEndpoint(id)),
    onMutate: () => setTestResult(null),
    onSuccess: (result, endpointId) => {
      setTestResult({ endpointId, result })
    },
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setDeleteTarget(null)
      setEditTarget(null)
      setTestResult(null)
    }
    props.onOpenChange(open)
  }

  return (
    <>
      <Dialog open={props.open} onOpenChange={handleOpenChange}>
        <DialogContent className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl'>
          <DialogHeader>
            <DialogTitle>
              {scope === 'tasks'
                ? t('Media task webhook')
                : t('Asset library webhook')}
            </DialogTitle>
            <DialogDescription>
              {scope === 'tasks'
                ? t('Receive media task status events via HTTPS POST.')
                : t('Receive asset review events via HTTPS POST.')}
            </DialogDescription>
          </DialogHeader>

          <WebhookManualButton
            active={props.open}
            scope={scope === 'tasks' ? 'media_tasks' : 'asset_library'}
          />
          {capabilities.isLoading && (
            <Spinner aria-label={t('Loading webhooks')} />
          )}
          {capabilities.error && (
            <p role='alert' className='text-destructive'>
              {webhookErrorMessage(capabilities.error)}
            </p>
          )}
          {!capabilities.isLoading && !capabilities.error && !scopeEnabled && (
            <p role='status' className='text-muted-foreground text-sm'>
              {t('Webhooks are disabled by the administrator.')}
            </p>
          )}
          {scopeEnabled && (
            <WebhookEndpointForm
              key={`${editTarget?.id ?? 'new'}:${enabledEvents.join(',')}`}
              enabledEvents={enabledEvents}
              scope={scope}
              endpoint={editTarget}
              atLimit={allEndpoints.length >= MAX_WEBHOOK_ENDPOINTS}
              saving={saveMutation.isPending}
              onSave={(values) =>
                saveMutation
                  .mutateAsync({ values, id: editTarget?.id })
                  .then(() => undefined)
              }
              onCancel={() => setEditTarget(null)}
            />
          )}
          {scopeEnabled &&
            allEndpoints.length >= MAX_WEBHOOK_ENDPOINTS &&
            endpoints.length === 0 && (
              <p role='status' className='text-muted-foreground text-sm'>
                {t(
                  'The account limit is full. Delete an endpoint in the other category to add one here.'
                )}
              </p>
            )}

          {endpointsQuery.isLoading && (
            <div className='grid gap-2' aria-label={t('Loading webhooks')}>
              <Skeleton className='h-24' />
              <Skeleton className='h-24' />
            </div>
          )}
          {endpointsQuery.error && (
            <p className='text-destructive py-4 text-center text-sm'>
              {webhookErrorMessage(endpointsQuery.error)}
            </p>
          )}
          {!endpointsQuery.isLoading &&
            !endpointsQuery.error &&
            endpoints.length === 0 && (
              <Empty className='min-h-32 border'>
                <EmptyHeader>
                  <EmptyMedia variant='icon'>
                    <HugeiconsIcon icon={WebhookIcon} />
                  </EmptyMedia>
                  <EmptyTitle>{t('No webhook endpoints yet')}</EmptyTitle>
                  {scopeEnabled &&
                    allEndpoints.length < MAX_WEBHOOK_ENDPOINTS && (
                      <EmptyDescription>
                        {t(
                          'Add your callback address above to receive events.'
                        )}
                      </EmptyDescription>
                    )}
                </EmptyHeader>
              </Empty>
            )}
          {endpoints.length > 0 && (
            <div className='grid gap-2'>
              {endpoints.map((endpoint) => (
                <div
                  key={endpoint.id}
                  className='grid gap-3 rounded-xl border p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'
                >
                  <div className='min-w-0 space-y-1'>
                    <p className='truncate text-sm font-medium'>
                      {endpoint.name}
                    </p>
                    <p className='text-muted-foreground truncate text-xs'>
                      {endpoint.url}
                    </p>
                    <div className='flex flex-wrap gap-1'>
                      {endpoint.event_types
                        .filter((eventType) => scopeEvents.includes(eventType))
                        .map((eventType) => (
                          <Badge key={eventType} variant='outline'>
                            {t(eventLabelKey(eventType))}
                          </Badge>
                        ))}
                    </div>
                    <p className='text-muted-foreground text-xs'>
                      {t('Created')}{' '}
                      {formatTimestampToDate(endpoint.created_at)}
                    </p>
                  </div>
                  <div className='flex justify-end gap-1'>
                    <Button
                      size='icon-sm'
                      variant='ghost'
                      aria-label={t('Edit webhook {{name}}', {
                        name: endpoint.name,
                      })}
                      disabled={
                        saveMutation.isPending ||
                        !scopeEnabled ||
                        !endpoint.event_types.some((event) =>
                          scopeEvents.includes(event)
                        )
                      }
                      onClick={() => setEditTarget(endpoint)}
                    >
                      <HugeiconsIcon icon={PencilEdit02Icon} />
                    </Button>
                    <Button
                      size='icon-sm'
                      variant='ghost'
                      aria-label={t('Send test event to {{name}}', {
                        name: endpoint.name,
                      })}
                      disabled={
                        testMutation.isPending ||
                        !scopeEnabled ||
                        !endpoint.event_types.every((event) =>
                          enabledEvents.includes(event)
                        )
                      }
                      onClick={() => testMutation.mutate(endpoint.id)}
                    >
                      {testMutation.isPending &&
                      testMutation.variables === endpoint.id ? (
                        <Spinner />
                      ) : (
                        <HugeiconsIcon icon={SentIcon} />
                      )}
                    </Button>
                    <Button
                      size='icon-sm'
                      variant='ghost'
                      className='text-destructive'
                      aria-label={t('Delete webhook {{name}}', {
                        name: endpoint.name,
                      })}
                      onClick={() => setDeleteTarget(endpoint)}
                    >
                      <HugeiconsIcon icon={Delete02Icon} />
                    </Button>
                  </div>
                  {testResult?.endpointId === endpoint.id && (
                    <div className='sm:col-span-2'>
                      <WebhookTestResult result={testResult.result} />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete webhook endpoint?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Future events will no longer be sent to this callback address.'
              )}
              {deleteTarget?.event_types.some((event) =>
                event.startsWith('asset.')
              ) &&
                deleteTarget.event_types.includes('task.status_changed') && (
                  <span className='mt-2 block'>
                    {t(
                      'This endpoint also subscribes to the other category. Deleting it stops both.'
                    )}
                  </span>
                )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={deleteMutation.isPending}
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {deleteMutation.isPending && <Spinner data-icon='inline-start' />}
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
