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
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import {
  getSystemWebhookSettings,
  saveSystemWebhookSettings,
  testSystemWebhook,
} from '@/features/webhooks/api'
import { WebhookManualButton } from '@/features/webhooks/components/webhook-manual-button'
import { WebhookTestResult } from '@/features/webhooks/components/webhook-test-result'
import { webhookFormSchema } from '@/features/webhooks/lib/form-schema'
import {
  assertWebhookSuccess,
  webhookErrorMessage,
} from '@/features/webhooks/lib/response'
import { systemWebhookSchema } from '@/features/webhooks/lib/system-schema'
import type {
  SystemWebhookSettings,
  SystemWebhookTopic,
  WebhookProbeResult,
} from '@/features/webhooks/types'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

function SystemWebhookForm(props: { initial: SystemWebhookSettings }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [results, setResults] = useState<
    Partial<Record<SystemWebhookTopic, WebhookProbeResult>>
  >({})
  const form = useForm<SystemWebhookSettings>({
    resolver: zodResolver(systemWebhookSchema(t)),
    defaultValues: props.initial,
  })
  const [assetEnabled, taskEnabled] = useWatch({
    control: form.control,
    name: ['asset_library.enabled', 'media_tasks.enabled'],
  })
  const save = useMutation({
    mutationFn: async (values: SystemWebhookSettings) =>
      assertWebhookSuccess(await saveSystemWebhookSettings(values)),
    onSuccess: async (values) => {
      await queryClient.cancelQueries({ queryKey: ['webhook-capabilities'] })
      queryClient.setQueryData(['system-webhooks'], values)
      queryClient.setQueryData(['webhook-capabilities'], {
        asset_library_enabled: values.asset_library.enabled,
        media_tasks_enabled: values.media_tasks.enabled,
        manual_enabled: values.manual_enabled,
      })
      form.reset(values)
      toast.success(t('System webhook settings saved'))
    },
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })
  const probe = useMutation({
    mutationFn: async (input: { topic: SystemWebhookTopic; url: string }) =>
      assertWebhookSuccess(await testSystemWebhook(input.topic, input.url)),
    onSuccess: (result, input) =>
      setResults((current) => ({ ...current, [input.topic]: result })),
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })
  const topics: {
    id: SystemWebhookTopic
    label: string
    description: string
    enabled: boolean
  }[] = [
    {
      id: 'asset_library',
      label: t('Asset library webhook'),
      description: t(
        'Allow users to configure asset webhooks and receive asset review events.'
      ),
      enabled: assetEnabled,
    },
    {
      id: 'media_tasks',
      label: t('Media task webhook'),
      description: t(
        'Allow users to configure media webhooks and receive task status events.'
      ),
      enabled: taskEnabled,
    },
  ]
  const submit = form.handleSubmit((values) => save.mutate(values))
  return (
    <form onSubmit={submit}>
      <SettingsPageFormActions
        onSave={submit}
        isSaving={save.isPending}
        isSaveDisabled={!form.formState.isDirty}
      />
      <FieldGroup className='gap-4'>
        <div
          role='group'
          aria-label={t('Webhook categories')}
          className='grid gap-4 xl:grid-cols-2'
        >
          {topics.map((topic) => {
            const result = results[topic.id]
            return (
              <Card
                key={topic.id}
                role='group'
                aria-labelledby={`system-${topic.id}-title`}
              >
                <CardHeader>
                  <CardTitle id={`system-${topic.id}-title`}>
                    {topic.label}
                  </CardTitle>
                  <CardDescription>{topic.description}</CardDescription>
                  <CardAction>
                    <Controller
                      control={form.control}
                      name={`${topic.id}.enabled`}
                      render={({ field }) => (
                        <Switch
                          id={`system-${topic.id}-enabled`}
                          aria-label={t('Enable {{name}}', {
                            name: topic.label,
                          })}
                          checked={field.value}
                          onCheckedChange={(enabled) => {
                            field.onChange(enabled)
                            form.clearErrors(`${topic.id}.url`)
                            setResults((current) => ({
                              ...current,
                              [topic.id]: undefined,
                            }))
                          }}
                          disabled={save.isPending || probe.isPending}
                        />
                      )}
                    />
                  </CardAction>
                </CardHeader>
                <CardContent>
                  <FieldGroup className='gap-3'>
                    <Controller
                      control={form.control}
                      name={`${topic.id}.url`}
                      render={({ field, fieldState }) => (
                        <Field data-invalid={fieldState.invalid}>
                          <FieldLabel htmlFor={`system-${topic.id}-url`}>
                            {t('Callback address for {{name}}', {
                              name: topic.label,
                            })}
                          </FieldLabel>
                          <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]'>
                            <Input
                              {...field}
                              onChange={(event) => {
                                field.onChange(event)
                                setResults((current) => ({
                                  ...current,
                                  [topic.id]: undefined,
                                }))
                              }}
                              id={`system-${topic.id}-url`}
                              type='url'
                              maxLength={2048}
                              placeholder='https://example.com/webhooks/events'
                              aria-invalid={fieldState.invalid}
                              disabled={
                                !topic.enabled ||
                                save.isPending ||
                                probe.isPending
                              }
                            />
                            <Button
                              type='button'
                              variant='outline'
                              aria-label={t('Test {{name}}', {
                                name: topic.label,
                              })}
                              disabled={
                                !topic.enabled ||
                                !field.value.trim() ||
                                probe.isPending ||
                                save.isPending
                              }
                              onClick={() => {
                                const url = form.getValues(`${topic.id}.url`)
                                if (
                                  !webhookFormSchema(t).shape.url.safeParse(url)
                                    .success
                                ) {
                                  form.setError(`${topic.id}.url`, {
                                    message: t(
                                      'Enter a valid HTTPS callback URL'
                                    ),
                                  })
                                  return
                                }
                                form.clearErrors(`${topic.id}.url`)
                                probe.mutate({ topic: topic.id, url })
                              }}
                            >
                              {probe.isPending &&
                                probe.variables?.topic === topic.id && (
                                  <Spinner data-icon='inline-start' />
                                )}
                              {t('Test connection')}
                            </Button>
                          </div>
                          <FieldDescription>
                            {t(
                              'Optional public HTTPS address for events from all accounts.'
                            )}
                          </FieldDescription>
                          <FieldError errors={[fieldState.error]} />
                        </Field>
                      )}
                    />
                    {topic.enabled && result && (
                      <WebhookTestResult result={result} />
                    )}
                  </FieldGroup>
                </CardContent>
              </Card>
            )
          })}
        </div>
        <Card>
          <CardHeader>
            <CardTitle>{t('Webhook API manual')}</CardTitle>
            <CardDescription>
              {t(
                'Enable PDF generation and download in user webhook settings. The manual uses the current document branding.'
              )}
            </CardDescription>
            <CardAction>
              <Controller
                control={form.control}
                name='manual_enabled'
                render={({ field }) => (
                  <Switch
                    id='webhook-manual-enabled'
                    aria-label={t(
                      'Allow users to download the Webhook API manual'
                    )}
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={save.isPending}
                  />
                )}
              />
            </CardAction>
          </CardHeader>
          {props.initial.manual_enabled && (
            <CardContent>
              <div className='flex flex-wrap gap-2'>
                <WebhookManualButton scope='asset_library' />
                <WebhookManualButton scope='media_tasks' />
              </div>
            </CardContent>
          )}
        </Card>
      </FieldGroup>
    </form>
  )
}

export function SystemWebhooksSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['system-webhooks'],
    queryFn: async () => assertWebhookSuccess(await getSystemWebhookSettings()),
    refetchOnWindowFocus: false,
  })
  return (
    <SettingsSection title={t('System Webhooks')} className='w-full min-w-0'>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Category switches control user configuration and all deliveries. An optional system address receives events from every account.'
        )}
      </p>
      {query.isLoading && <Spinner aria-label={t('Loading webhooks')} />}
      {query.error && (
        <p role='alert' className='text-destructive'>
          {webhookErrorMessage(query.error)}
        </p>
      )}
      {query.data && <SystemWebhookForm initial={query.data} />}
    </SettingsSection>
  )
}
