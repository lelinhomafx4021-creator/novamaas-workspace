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
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

import { webhookFormSchema } from '../lib/form-schema'
import type {
  WebhookEndpoint,
  WebhookEndpointInput,
  WebhookEventType,
} from '../types'

export function WebhookEndpointForm(props: {
  scope: 'assets' | 'tasks'
  enabledEvents: WebhookEventType[]
  endpoint: WebhookEndpoint | null
  atLimit: boolean
  saving: boolean
  onSave: (values: WebhookEndpointInput) => Promise<void>
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const defaults: WebhookEndpointInput = props.endpoint
    ? {
        name: props.endpoint.name,
        url: props.endpoint.url,
        event_types: props.endpoint.event_types.filter((event) =>
          props.enabledEvents.includes(event)
        ),
      }
    : {
        name: '',
        url: '',
        event_types:
          props.scope === 'tasks'
            ? ['task.status_changed']
            : ['asset.active', 'asset.failed'],
      }
  const form = useForm<WebhookEndpointInput>({
    resolver: zodResolver(webhookFormSchema(t)),
    defaultValues: defaults,
  })
  const disabled = props.saving || (!props.endpoint && props.atLimit)
  const events: { value: WebhookEventType; label: string }[] = [
    { value: 'asset.active', label: t('Asset becomes active') },
    { value: 'asset.failed', label: t('Asset fails review') },
  ]
  return (
    <form
      className='rounded-xl border p-3'
      onSubmit={form.handleSubmit(async (values) => {
        if (
          props.scope === 'assets' &&
          !values.event_types.some((event) => event.startsWith('asset.'))
        ) {
          form.setError('event_types', {
            message: t('Select at least one event'),
          })
          return
        }
        try {
          await props.onSave(values)
          form.reset(defaults)
        } catch {
          // The dialog reports API errors; retain the form for correction/retry.
        }
      })}
    >
      <FieldGroup className='gap-3'>
        <div className='grid gap-3 sm:grid-cols-2'>
          <Controller
            control={form.control}
            name='name'
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor='webhook-name'>
                  {t('Endpoint name')}
                </FieldLabel>
                <Input
                  {...field}
                  id='webhook-name'
                  maxLength={64}
                  placeholder={t('Example: production callback')}
                  aria-invalid={fieldState.invalid}
                  disabled={disabled}
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
                <FieldLabel htmlFor='webhook-url'>
                  {t('Callback URL')}
                </FieldLabel>
                <Input
                  {...field}
                  id='webhook-url'
                  type='url'
                  inputMode='url'
                  maxLength={2048}
                  placeholder='https://example.com/webhooks/events'
                  aria-invalid={fieldState.invalid}
                  disabled={disabled}
                />
                <FieldDescription>
                  {t('Must be a public HTTPS address.')}
                </FieldDescription>
                <FieldError errors={[fieldState.error]} />
              </Field>
            )}
          />
        </div>
        {props.scope === 'assets' && (
          <Controller
            control={form.control}
            name='event_types'
            render={({ field, fieldState }) => (
              <FieldSet>
                <FieldLegend variant='label'>{t('Events')}</FieldLegend>
                <FieldGroup className='flex flex-wrap gap-x-5 gap-y-2 sm:flex-row'>
                  {events.map((event) => (
                    <Field
                      key={event.value}
                      orientation='horizontal'
                      data-invalid={fieldState.invalid}
                      className='w-auto'
                    >
                      <Checkbox
                        id={`webhook-${event.value}`}
                        checked={field.value.includes(event.value)}
                        disabled={disabled}
                        aria-invalid={fieldState.invalid}
                        onCheckedChange={(checked) => {
                          field.onChange(
                            checked
                              ? [...field.value, event.value]
                              : field.value.filter(
                                  (value) => value !== event.value
                                )
                          )
                        }}
                      />
                      <FieldLabel htmlFor={`webhook-${event.value}`}>
                        {event.label}
                      </FieldLabel>
                    </Field>
                  ))}
                </FieldGroup>
                <FieldError errors={[fieldState.error]} />
              </FieldSet>
            )}
          />
        )}
        {props.endpoint?.event_types.some(
          (event) => !props.enabledEvents.includes(event)
        ) && (
          <FieldDescription>
            {t(
              'Subscriptions to disabled categories are removed when you save.'
            )}
          </FieldDescription>
        )}
        {props.endpoint?.event_types.some(
          (event) =>
            props.enabledEvents.includes(event) &&
            (props.scope === 'assets'
              ? event === 'task.status_changed'
              : event.startsWith('asset.'))
        ) && (
          <FieldDescription>
            {t(
              'This endpoint also delivers the other category. Name or URL changes affect both.'
            )}
          </FieldDescription>
        )}
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <FieldDescription>
            {t('Up to 5 active webhook endpoints per account.')}
          </FieldDescription>
          <div className='flex gap-2'>
            {props.endpoint && (
              <Button
                type='button'
                variant='outline'
                disabled={props.saving}
                onClick={props.onCancel}
              >
                {t('Cancel')}
              </Button>
            )}
            <Button type='submit' disabled={disabled}>
              {props.saving && <Spinner data-icon='inline-start' />}
              {props.endpoint ? t('Save') : t('Add endpoint')}
            </Button>
          </div>
        </div>
      </FieldGroup>
    </form>
  )
}
