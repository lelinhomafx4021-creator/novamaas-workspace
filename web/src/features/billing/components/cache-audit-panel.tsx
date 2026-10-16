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
import { useMutation, useQuery } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { handleServerError } from '@/lib/handle-server-error'

import { billingToday } from '../api'
import {
  exportCacheAudit,
  runCacheAudit,
  searchCacheAuditKeys,
  type CacheAuditReport,
  type CacheAuditRequest,
} from '../cache-audit-api'
import {
  CACHE_AUDIT_STATUS_LABELS,
  cacheAuditRequest,
  cacheAuditSchema,
  type CacheAuditValues,
} from '../lib/cache-audit'
import { CacheAuditCompensation } from './cache-audit-compensation'
import { CacheAuditDetails } from './cache-audit-details'
import { CacheAuditSummary } from './cache-audit-summary'

export function CacheAuditPanel(props: { userId: number }) {
  const { t } = useTranslation()
  const id = useId()
  const today = billingToday()
  const form = useForm<CacheAuditValues>({
    resolver: zodResolver(cacheAuditSchema),
    defaultValues: {
      start: `${today.slice(0, 7)}-01`,
      end: today,
      tokenId: 0,
      model: '',
      fixedAt: '',
      status: '',
    },
  })
  const [result, setResult] = useState<{
    report: CacheAuditReport
    request: CacheAuditRequest
  } | null>(null)
  const [prices, setPrices] = useState<Record<string, string>>({})
  const keys = useQuery({
    queryKey: ['billing', 'cache-audit-keys', props.userId],
    queryFn: () => searchCacheAuditKeys(props.userId, ''),
  })
  const audit = useMutation({
    mutationFn: (request: CacheAuditRequest) => runCacheAudit(request),
    onSuccess: (report, request) => setResult({ report, request }),
    onError: handleServerError,
  })
  const download = useMutation({
    mutationFn: (format: 'csv' | 'json') => {
      if (!result) throw new Error(t('Run the audit first'))
      return exportCacheAudit(result.request, format)
    },
    onError: handleServerError,
  })
  const errors = form.formState.errors
  const pending = audit.isPending || download.isPending
  return (
    <div className='flex flex-col gap-4'>
      <h2 className='font-semibold'>{t('Cache usage audit')}</h2>
      <p className='text-muted-foreground text-sm'>
        {t(
          'For /v1/messages requests served by OpenAI-compatible upstreams, input_tokens used to include cache tokens that were also reported in the cache fields. Platform charges were correct; this report gives customers the duplicated token counts for their own billing.'
        )}
      </p>
      <form
        className='flex flex-col gap-4'
        onSubmit={form.handleSubmit((value) =>
          audit.mutate({
            ...cacheAuditRequest(props.userId, value),
            downstream_input_prices: prices,
          })
        )}
      >
        <FieldGroup className='grid gap-4 sm:grid-cols-3'>
          <Field data-invalid={Boolean(errors.start)}>
            <FieldLabel htmlFor={`${id}-start`}>{t('Start date')}</FieldLabel>
            <Input
              id={`${id}-start`}
              type='date'
              max={today}
              aria-invalid={Boolean(errors.start)}
              disabled={pending}
              {...form.register('start')}
            />
          </Field>
          <Field data-invalid={Boolean(errors.end)}>
            <FieldLabel htmlFor={`${id}-end`}>{t('End date')}</FieldLabel>
            <Input
              id={`${id}-end`}
              type='date'
              max={today}
              aria-invalid={Boolean(errors.end)}
              disabled={pending}
              {...form.register('end')}
            />
            <FieldError>{errors.end?.message && t(errors.end.message)}</FieldError>
          </Field>
          <Field>
            <FieldLabel htmlFor={`${id}-key`}>{t('API key')}</FieldLabel>
            <NativeSelect
              id={`${id}-key`}
              className='w-full'
              disabled={pending || keys.isPending}
              {...form.register('tokenId', { valueAsNumber: true })}
            >
              <NativeSelectOption value={0}>{t('All keys')}</NativeSelectOption>
              {keys.data?.map((key) => (
                <NativeSelectOption key={key.id} value={key.id}>
                  {key.name || '-'} (#{key.id})
                  {key.deleted ? ` ${t('(deleted)')}` : ''}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </Field>
          <Field>
            <FieldLabel htmlFor={`${id}-model`}>{t('Model')}</FieldLabel>
            <Input
              id={`${id}-model`}
              placeholder={t('All models')}
              disabled={pending}
              {...form.register('model')}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`${id}-fixed`}>
              {t('Fix went live at')}
            </FieldLabel>
            <Input
              id={`${id}-fixed`}
              type='datetime-local'
              disabled={pending}
              {...form.register('fixedAt')}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`${id}-status`}>{t('Detail filter')}</FieldLabel>
            <NativeSelect
              id={`${id}-status`}
              className='w-full'
              disabled={pending}
              {...form.register('status')}
            >
              <NativeSelectOption value=''>{t('All records')}</NativeSelectOption>
              {Object.entries(CACHE_AUDIT_STATUS_LABELS).map(([value, label]) => (
                <NativeSelectOption key={value} value={value}>
                  {t(label)}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </Field>
        </FieldGroup>
        {keys.isError && <p role='alert'>{t('Failed to load data')}</p>}
        <div className='flex flex-wrap gap-2'>
          <Button type='submit' disabled={pending}>
            {t('Run audit')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={!result || pending}
            onClick={() => download.mutate('csv')}
          >
            {t('Export CSV')}
          </Button>
          <Button
            type='button'
            variant='outline'
            disabled={!result || pending}
            onClick={() => download.mutate('json')}
          >
            {t('Export JSON')}
          </Button>
        </div>
      </form>
      {result && (
        <>
          <CacheAuditCompensation
            report={result.report}
            prices={prices}
            pending={pending}
            onPriceChange={(model, value) =>
              setPrices((current) => ({ ...current, [model]: value }))
            }
            onRecalculate={() =>
              audit.mutate({
                ...result.request,
                downstream_input_prices: prices,
              })
            }
          />
          <CacheAuditSummary report={result.report} />
          <CacheAuditDetails report={result.report} />
        </>
      )}
    </div>
  )
}
