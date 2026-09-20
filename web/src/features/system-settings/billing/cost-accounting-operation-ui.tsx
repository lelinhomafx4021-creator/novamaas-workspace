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
import type { ReactNode } from 'react'
import type {
  FieldError as ReactHookFormFieldError,
  UseFormRegisterReturn,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

type CommonFieldName =
  | 'start_time'
  | 'end_time'
  | 'channel_id'
  | 'cost_discount'
  | 'limit'

type CommonFieldRegistrations = Record<
  CommonFieldName,
  UseFormRegisterReturn<CommonFieldName>
>

type CommonFieldErrors = Partial<
  Record<CommonFieldName, ReactHookFormFieldError>
>

export function CommonOperationFields(props: {
  idPrefix: string
  registrations: CommonFieldRegistrations
  errors: CommonFieldErrors
  maxBatchSize: number
}) {
  const { t } = useTranslation()
  const errorText = (error?: ReactHookFormFieldError) =>
    error?.message ? t(error.message) : undefined

  return (
    <FieldGroup className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
      <Field data-invalid={Boolean(props.errors.start_time)}>
        <FieldLabel htmlFor={`${props.idPrefix}-start`}>
          {t('Start Time')}
        </FieldLabel>
        <Input
          id={`${props.idPrefix}-start`}
          type='datetime-local'
          aria-invalid={Boolean(props.errors.start_time)}
          aria-describedby={`${props.idPrefix}-start-error`}
          {...props.registrations.start_time}
        />
        <FieldError id={`${props.idPrefix}-start-error`}>
          {errorText(props.errors.start_time)}
        </FieldError>
      </Field>

      <Field data-invalid={Boolean(props.errors.end_time)}>
        <FieldLabel htmlFor={`${props.idPrefix}-end`}>
          {t('End Time')}
        </FieldLabel>
        <Input
          id={`${props.idPrefix}-end`}
          type='datetime-local'
          aria-invalid={Boolean(props.errors.end_time)}
          aria-describedby={`${props.idPrefix}-end-error`}
          {...props.registrations.end_time}
        />
        <FieldError id={`${props.idPrefix}-end-error`}>
          {errorText(props.errors.end_time)}
        </FieldError>
      </Field>

      <Field data-invalid={Boolean(props.errors.channel_id)}>
        <FieldLabel htmlFor={`${props.idPrefix}-channel`}>
          {t('Channel ID')}
        </FieldLabel>
        <Input
          id={`${props.idPrefix}-channel`}
          type='number'
          min={0}
          step={1}
          aria-invalid={Boolean(props.errors.channel_id)}
          aria-describedby={`${props.idPrefix}-channel-help ${props.idPrefix}-channel-error`}
          {...props.registrations.channel_id}
        />
        <FieldDescription id={`${props.idPrefix}-channel-help`}>
          {t('Use 0 to include all channels with the same discount.')}
        </FieldDescription>
        <FieldError id={`${props.idPrefix}-channel-error`}>
          {errorText(props.errors.channel_id)}
        </FieldError>
      </Field>

      <Field data-invalid={Boolean(props.errors.cost_discount)}>
        <FieldLabel htmlFor={`${props.idPrefix}-discount`}>
          {t('Upstream cost discount')}
        </FieldLabel>
        <Input
          id={`${props.idPrefix}-discount`}
          inputMode='decimal'
          placeholder={t('Leave empty for zero-profit accounting')}
          aria-invalid={Boolean(props.errors.cost_discount)}
          aria-describedby={`${props.idPrefix}-discount-help ${props.idPrefix}-discount-error`}
          {...props.registrations.cost_discount}
        />
        <FieldDescription id={`${props.idPrefix}-discount-help`}>
          {t(
            'Enter a decimal from 0 to 1 with up to 6 decimal places, or leave empty to make cost equal turnover.'
          )}
        </FieldDescription>
        <FieldError id={`${props.idPrefix}-discount-error`}>
          {errorText(props.errors.cost_discount)}
        </FieldError>
      </Field>

      <Field data-invalid={Boolean(props.errors.limit)}>
        <FieldLabel htmlFor={`${props.idPrefix}-limit`}>
          {t('Batch size')}
        </FieldLabel>
        <Input
          id={`${props.idPrefix}-limit`}
          type='number'
          min={1}
          max={props.maxBatchSize}
          step={1}
          aria-invalid={Boolean(props.errors.limit)}
          aria-describedby={`${props.idPrefix}-limit-help ${props.idPrefix}-limit-error`}
          {...props.registrations.limit}
        />
        <FieldDescription id={`${props.idPrefix}-limit-help`}>
          {t('Maximum {{count}} records per request.', {
            count: props.maxBatchSize,
          })}
        </FieldDescription>
        <FieldError id={`${props.idPrefix}-limit-error`}>
          {errorText(props.errors.limit)}
        </FieldError>
      </Field>
    </FieldGroup>
  )
}

export function ResultMetric(props: {
  label: ReactNode
  value: ReactNode
  emphasis?: boolean
}) {
  return (
    <div className='bg-muted/30 flex min-w-0 flex-col gap-1 rounded-lg border p-3'>
      <span className='text-muted-foreground text-xs'>{props.label}</span>
      <span
        className={
          props.emphasis
            ? 'text-primary truncate text-base font-semibold'
            : 'truncate text-base font-semibold'
        }
      >
        {props.value}
      </span>
    </div>
  )
}

export function OperationResult(props: {
  phase: 'preview' | 'applied'
  batchId: string
  isCurrent: boolean
  hasMore: boolean
  children: ReactNode
}) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-3 rounded-xl border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex items-center gap-2'>
          <span className='text-sm font-medium'>{t('Batch result')}</span>
          <Badge variant={props.phase === 'applied' ? 'default' : 'secondary'}>
            {props.phase === 'applied' ? t('Applied') : t('Preview only')}
          </Badge>
        </div>
        <span className='text-muted-foreground max-w-full truncate font-mono text-xs'>
          {t('Batch ID')}: {props.batchId}
        </span>
      </div>
      {!props.isCurrent && (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Inputs changed after this preview. Run a new preview before applying.'
            )}
          </AlertDescription>
        </Alert>
      )}
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
        {props.children}
      </div>
      <p className='text-muted-foreground text-xs'>
        {props.hasMore
          ? t('More records are available. Continue with the next batch.')
          : t('This batch reached the end of the selected range.')}
      </p>
    </div>
  )
}
