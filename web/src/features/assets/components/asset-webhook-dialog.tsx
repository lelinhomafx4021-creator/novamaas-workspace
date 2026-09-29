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
import { zodResolver } from '@hookform/resolvers/zod'
import {
  Delete02Icon,
  RefreshIcon,
  SentIcon,
  WebhookIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { CopyButton } from '@/components/copy-button'
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
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { formatTimestampToDate } from '@/lib/format'

import {
  createAssetWebhookEndpoint,
  deleteAssetWebhookEndpoint,
  listAssetWebhookEndpoints,
  rotateAssetWebhookEndpointSecret,
  testAssetWebhookEndpoint,
} from '../api'
import { assertAssetSuccess, assetErrorMessage } from '../asset-utils'
import type {
  AssetWebhookEndpoint,
  AssetWebhookEventType,
  CreatedAssetWebhookEndpoint,
} from '../types'

const WEBHOOKS_QUERY_KEY = ['asset-library', 'webhook-endpoints'] as const
const MAX_WEBHOOK_ENDPOINTS = 5

type WebhookFormValues = {
  name: string
  url: string
  active: boolean
  failed: boolean
}

function eventLabelKey(eventType: AssetWebhookEventType) {
  return eventType === 'asset.active'
    ? 'Asset becomes active'
    : 'Asset fails review'
}

export function AssetWebhookDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [revealedSecret, setRevealedSecret] =
    useState<CreatedAssetWebhookEndpoint | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AssetWebhookEndpoint | null>(
    null
  )
  const [rotateTarget, setRotateTarget] = useState<AssetWebhookEndpoint | null>(
    null
  )
  const schema = z
    .object({
      name: z.string().trim().min(1, t('Endpoint name is required')).max(64),
      url: z
        .string()
        .trim()
        .url(t('Enter a valid HTTPS callback URL'))
        .refine((value) => value.startsWith('https://'), {
          message: t('Enter a valid HTTPS callback URL'),
        }),
      active: z.boolean(),
      failed: z.boolean(),
    })
    .refine((value) => value.active || value.failed, {
      message: t('Select at least one event'),
      path: ['active'],
    })
  const form = useForm<WebhookFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { name: '', url: '', active: true, failed: true },
  })
  const endpointsQuery = useQuery({
    queryKey: WEBHOOKS_QUERY_KEY,
    queryFn: async () => assertAssetSuccess(await listAssetWebhookEndpoints()),
    enabled: props.open,
  })
  const endpoints = endpointsQuery.data ?? []
  const createMutation = useMutation({
    mutationFn: async (values: WebhookFormValues) => {
      const eventTypes: AssetWebhookEventType[] = []
      if (values.active) eventTypes.push('asset.active')
      if (values.failed) eventTypes.push('asset.failed')
      return assertAssetSuccess(
        await createAssetWebhookEndpoint({
          name: values.name.trim(),
          url: values.url.trim(),
          event_types: eventTypes,
        })
      )
    },
    onSuccess: async (endpoint) => {
      setRotateTarget(null)
      setRevealedSecret(endpoint)
      form.reset()
      await queryClient.invalidateQueries({ queryKey: WEBHOOKS_QUERY_KEY })
      toast.success(t('Webhook endpoint created'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const deleteMutation = useMutation({
    mutationFn: async (id: string) =>
      assertAssetSuccess(await deleteAssetWebhookEndpoint(id)),
    onSuccess: async () => {
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: WEBHOOKS_QUERY_KEY })
      toast.success(t('Webhook endpoint deleted'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const rotateMutation = useMutation({
    mutationFn: async (id: string) =>
      assertAssetSuccess(await rotateAssetWebhookEndpointSecret(id)),
    onSuccess: async (endpoint) => {
      setRevealedSecret(endpoint)
      await queryClient.invalidateQueries({ queryKey: WEBHOOKS_QUERY_KEY })
      toast.success(t('Signing secret rotated'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const testMutation = useMutation({
    mutationFn: async (id: string) =>
      assertAssetSuccess(await testAssetWebhookEndpoint(id)),
    onSuccess: () => toast.success(t('Test event queued')),
    onError: (error) => toast.error(assetErrorMessage(error)),
  })

  const handleOpenChange = (open: boolean) => {
    if (!open && revealedSecret) return
    if (!open) {
      setDeleteTarget(null)
      setRotateTarget(null)
      form.reset()
    }
    props.onOpenChange(open)
  }

  return (
    <>
      <Dialog open={props.open} onOpenChange={handleOpenChange}>
        <DialogContent
          className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl'
          showCloseButton={!revealedSecret}
        >
          <DialogHeader>
            <DialogTitle>{t('Asset status webhooks')}</DialogTitle>
            <DialogDescription>
              {t(
                'We will POST signed events when an asset becomes active or later fails review.'
              )}
            </DialogDescription>
          </DialogHeader>

          {revealedSecret && (
            <section
              aria-labelledby='asset-webhook-secret-title'
              className='border-warning/40 bg-warning/5 grid gap-3 rounded-xl border p-3'
            >
              <div className='grid gap-1'>
                <h2 id='asset-webhook-secret-title' className='font-medium'>
                  {t('Save your webhook signing secret now')}
                </h2>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'This secret is displayed only once. Use it to verify the webhook-signature header.'
                  )}
                </p>
              </div>
              <div className='flex min-w-0 items-center gap-1'>
                <code className='bg-background min-w-0 flex-1 truncate rounded-md border px-2 py-1.5 text-xs'>
                  {revealedSecret.signing_secret}
                </code>
                <CopyButton
                  value={revealedSecret.signing_secret}
                  aria-label={t('Copy signing secret')}
                />
              </div>
              <Button
                size='sm'
                className='justify-self-start'
                onClick={() => setRevealedSecret(null)}
              >
                {t('I have saved the signing secret')}
              </Button>
            </section>
          )}

          <form
            className='rounded-xl border p-3'
            onSubmit={form.handleSubmit((values) =>
              createMutation.mutate(values)
            )}
          >
            <FieldGroup className='gap-3'>
              <div className='grid gap-3 sm:grid-cols-2'>
                <Controller
                  control={form.control}
                  name='name'
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor='asset-webhook-name'>
                        {t('Endpoint name')}
                      </FieldLabel>
                      <Input
                        {...field}
                        id='asset-webhook-name'
                        maxLength={64}
                        placeholder={t('Example: production callback')}
                        aria-invalid={fieldState.invalid}
                        disabled={
                          createMutation.isPending ||
                          endpoints.length >= MAX_WEBHOOK_ENDPOINTS
                        }
                      />
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
                <Controller
                  control={form.control}
                  name='url'
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor='asset-webhook-url'>
                        {t('Callback URL')}
                      </FieldLabel>
                      <Input
                        {...field}
                        id='asset-webhook-url'
                        type='url'
                        inputMode='url'
                        placeholder='https://example.com/webhooks/assets'
                        aria-invalid={fieldState.invalid}
                        disabled={
                          createMutation.isPending ||
                          endpoints.length >= MAX_WEBHOOK_ENDPOINTS
                        }
                      />
                      <FieldDescription>
                        {t('Must be a public HTTPS address.')}
                      </FieldDescription>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
              </div>
              <Field>
                <FieldLabel>{t('Events')}</FieldLabel>
                <div className='flex flex-wrap gap-x-5 gap-y-2'>
                  <Controller
                    control={form.control}
                    name='active'
                    render={({ field }) => (
                      <label className='flex items-center gap-2 text-sm'>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                        {t('Asset becomes active')}
                      </label>
                    )}
                  />
                  <Controller
                    control={form.control}
                    name='failed'
                    render={({ field }) => (
                      <label className='flex items-center gap-2 text-sm'>
                        <Checkbox
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                        {t('Asset fails review')}
                      </label>
                    )}
                  />
                </div>
                <FieldError errors={[form.formState.errors.active]} />
              </Field>
              <div className='flex items-center justify-between gap-3'>
                <FieldDescription>
                  {t('Up to 5 active webhook endpoints per account.')}
                </FieldDescription>
                <Button
                  type='submit'
                  disabled={
                    createMutation.isPending ||
                    endpoints.length >= MAX_WEBHOOK_ENDPOINTS
                  }
                >
                  {createMutation.isPending && (
                    <Spinner data-icon='inline-start' />
                  )}
                  {t('Add endpoint')}
                </Button>
              </div>
            </FieldGroup>
          </form>

          {endpointsQuery.isLoading && (
            <div className='grid gap-2' aria-label={t('Loading webhooks')}>
              <Skeleton className='h-24' />
              <Skeleton className='h-24' />
            </div>
          )}
          {endpointsQuery.error && (
            <p className='text-destructive py-4 text-center text-sm'>
              {assetErrorMessage(endpointsQuery.error)}
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
                  <EmptyDescription>
                    {t('Add your callback address above to receive events.')}
                  </EmptyDescription>
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
                      {endpoint.event_types.map((eventType) => (
                        <Badge key={eventType} variant='outline'>
                          {t(eventLabelKey(eventType))}
                        </Badge>
                      ))}
                    </div>
                    <p className='text-muted-foreground text-xs'>
                      {t('Secret ending in {{hint}}', {
                        hint: endpoint.signing_secret_hint,
                      })}{' '}
                      · {t('Created')}{' '}
                      {formatTimestampToDate(endpoint.created_at)}
                    </p>
                  </div>
                  <div className='flex justify-end gap-1'>
                    <Button
                      size='icon-sm'
                      variant='ghost'
                      aria-label={t('Send test event to {{name}}', {
                        name: endpoint.name,
                      })}
                      disabled={
                        testMutation.isPending &&
                        testMutation.variables === endpoint.id
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
                      aria-label={t('Rotate signing secret for {{name}}', {
                        name: endpoint.name,
                      })}
                      disabled={rotateMutation.isPending}
                      onClick={() => setRotateTarget(endpoint)}
                    >
                      {rotateMutation.isPending &&
                      rotateMutation.variables === endpoint.id ? (
                        <Spinner />
                      ) : (
                        <HugeiconsIcon icon={RefreshIcon} />
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
                'Future asset status events will no longer be sent to this callback address.'
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

      <AlertDialog
        open={rotateTarget !== null}
        onOpenChange={(open) => !open && setRotateTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Rotate signing secret?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The current signing secret will stop working immediately. Update your receiver with the new secret.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              disabled={rotateMutation.isPending}
              onClick={() =>
                rotateTarget && rotateMutation.mutate(rotateTarget.id)
              }
            >
              {rotateMutation.isPending && <Spinner data-icon='inline-start' />}
              {t('Rotate secret')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
