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
import { Loader2, RotateCcw } from 'lucide-react'
import { useId, useState } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

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
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldError, FieldLabel } from '@/components/ui/field'
import { Textarea } from '@/components/ui/textarea'
import { formatLogQuota } from '@/lib/format'

import { repriceCostAccounting } from './cost-accounting-api'
import {
  operationFingerprint,
  useCostAccountingOperation,
} from './cost-accounting-operation'
import {
  CommonOperationFields,
  OperationResult,
  ResultMetric,
} from './cost-accounting-operation-ui'
import {
  costAccountingRepriceSchema,
  getDefaultAccountingRange,
  toAccountingTimestamp,
  type CostAccountingRepriceValues,
} from './cost-accounting-schema'
import type {
  CostAccountingRepriceInput,
  CostAccountingRepriceResult,
} from './cost-accounting-types'

export function CostAccountingRepriceCard(props: {
  initialValues?: CostAccountingRepriceValues
}) {
  const { t } = useTranslation()
  const id = useId()
  const [defaultValues] = useState<CostAccountingRepriceValues>(() =>
    props.initialValues
      ? props.initialValues
      : {
          ...getDefaultAccountingRange(),
          channel_id: 0,
          cost_discount: '',
          limit: 500,
          reason: '',
        }
  )
  const [confirmOpen, setConfirmOpen] = useState(false)
  const form = useForm<CostAccountingRepriceValues>({
    resolver: zodResolver(costAccountingRepriceSchema),
    defaultValues,
  })
  const operation = useCostAccountingOperation<
    CostAccountingRepriceInput,
    CostAccountingRepriceResult
  >({
    mutationFn: repriceCostAccounting,
    appliedMessage: (result) =>
      t('Applied {{count}} cost adjustments.', { count: result.applied }),
  })
  const operationState = operation.state
  const watchedValues = useWatch({ control: form.control })
  const currentFingerprint = operationFingerprint(watchedValues)
  const stateIsCurrent = operationState?.fingerprint === currentFingerprint
  const canContinue = Boolean(stateIsCurrent && operationState?.result.has_more)
  const canApply = Boolean(
    operationState &&
    stateIsCurrent &&
    operationState.phase === 'preview' &&
    operationState.result.changed > 0
  )

  const preview = form.handleSubmit((values) => {
    const fingerprint = operationFingerprint(form.getValues())
    const continueBatch =
      operationState?.fingerprint === fingerprint &&
      operationState.result.has_more
    operation.previewMutation.mutate({
      fingerprint,
      request: {
        start_timestamp: toAccountingTimestamp(values.start_time),
        end_timestamp: toAccountingTimestamp(values.end_time),
        channel_id: values.channel_id,
        cost_discount: values.cost_discount,
        offset: continueBatch ? operationState.result.next_offset : 0,
        limit: values.limit,
        apply: false,
        batch_id: continueBatch ? operationState.result.batch_id : '',
        reason: values.reason,
      },
    })
  })

  const apply = () => {
    if (!canApply || !operationState) return
    setConfirmOpen(false)
    operation.applyMutation.mutate({
      fingerprint: operationState.fingerprint,
      request: {
        ...operationState.request,
        apply: true,
        batch_id: operationState.result.batch_id,
      },
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Reprice cost snapshots')}</CardTitle>
        <CardDescription>
          {t(
            'Recalculate existing snapshot costs with a new discount. Applying creates append-only adjustments and preserves the original snapshots.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className='flex flex-col gap-5' onSubmit={preview}>
          <CommonOperationFields
            idPrefix={`${id}-reprice`}
            registrations={{
              start_time: form.register('start_time'),
              end_time: form.register('end_time'),
              channel_id: form.register('channel_id', { valueAsNumber: true }),
              cost_discount: form.register('cost_discount'),
              limit: form.register('limit', { valueAsNumber: true }),
            }}
            errors={form.formState.errors}
            maxBatchSize={1000}
          />

          <Field data-invalid={Boolean(form.formState.errors.reason)}>
            <FieldLabel htmlFor={`${id}-reason`}>
              {t('Adjustment reason')}
            </FieldLabel>
            <Textarea
              id={`${id}-reason`}
              maxLength={500}
              rows={3}
              placeholder={t(
                'Explain why the historical cost needs adjustment.'
              )}
              aria-invalid={Boolean(form.formState.errors.reason)}
              aria-describedby={`${id}-reason-error`}
              {...form.register('reason')}
            />
            <FieldError id={`${id}-reason-error`}>
              {form.formState.errors.reason?.message
                ? t(form.formState.errors.reason.message)
                : undefined}
            </FieldError>
          </Field>

          {operation.state && (
            <OperationResult
              phase={operation.state.phase}
              batchId={operation.state.result.batch_id}
              isCurrent={stateIsCurrent}
              hasMore={operation.state.result.has_more}
            >
              <ResultMetric
                label={t('Scanned')}
                value={operation.state.result.scanned}
              />
              <ResultMetric
                label={t('Changed')}
                value={operation.state.result.changed}
                emphasis
              />
              <ResultMetric
                label={t('Unchanged')}
                value={operation.state.result.unchanged}
              />
              <ResultMetric
                label={t('Current cost')}
                value={formatLogQuota(
                  operation.state.result.current_cost_quota
                )}
              />
              <ResultMetric
                label={t('New cost')}
                value={formatLogQuota(operation.state.result.new_cost_quota)}
              />
              <ResultMetric
                label={t('Cost change')}
                value={formatLogQuota(operation.state.result.delta_cost_quota)}
                emphasis
              />
              {operation.state.phase === 'applied' && (
                <ResultMetric
                  label={t('Applied')}
                  value={operation.state.result.applied}
                  emphasis
                />
              )}
            </OperationResult>
          )}

          <div className='flex flex-wrap justify-end gap-2'>
            {operation.state && (
              <Button
                type='button'
                variant='ghost'
                disabled={operation.isPending}
                onClick={() => operation.setState(undefined)}
              >
                <RotateCcw />
                {t('Restart batch')}
              </Button>
            )}
            <Button
              type='submit'
              variant='outline'
              disabled={operation.isPending}
            >
              {operation.previewMutation.isPending && (
                <Loader2 className='animate-spin' />
              )}
              {canContinue ? t('Preview next batch') : t('Preview')}
            </Button>
            <Button
              type='button'
              disabled={!canApply || operation.isPending}
              onClick={() => setConfirmOpen(true)}
            >
              {operation.applyMutation.isPending && (
                <Loader2 className='animate-spin' />
              )}
              {t('Apply this batch')}
            </Button>
          </div>
        </form>
      </CardContent>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Apply cost adjustments?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This appends {{count}} cost adjustments for batch {{batchId}}. The original snapshots remain unchanged.',
                {
                  count: operation.state?.result.changed ?? 0,
                  batchId: operation.state?.result.batch_id ?? '',
                }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={apply}>
              {t('Apply adjustments')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
