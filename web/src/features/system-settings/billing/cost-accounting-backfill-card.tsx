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
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { formatLogQuota } from '@/lib/format'

import { backfillCostAccounting } from './cost-accounting-api'
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
  costAccountingBackfillSchema,
  getDefaultAccountingRange,
  toAccountingTimestamp,
  type CostAccountingBackfillValues,
} from './cost-accounting-schema'
import type {
  CostAccountingBackfillInput,
  CostAccountingBackfillResult,
} from './cost-accounting-types'

export function CostAccountingBackfillCard(props: {
  onRepriceExisting: (values: CostAccountingBackfillValues) => void
}) {
  const { t } = useTranslation()
  const id = useId()
  const [defaultValues] = useState<CostAccountingBackfillValues>(() => ({
    ...getDefaultAccountingRange(),
    channel_id: 0,
    cost_discount: '',
    limit: 1000,
  }))
  const [confirmOpen, setConfirmOpen] = useState(false)
  const form = useForm<CostAccountingBackfillValues>({
    resolver: zodResolver(costAccountingBackfillSchema),
    defaultValues,
  })
  const operation = useCostAccountingOperation<
    CostAccountingBackfillInput,
    CostAccountingBackfillResult
  >({
    mutationFn: backfillCostAccounting,
    appliedMessage: (result) =>
      t('Applied {{count}} cost snapshots.', { count: result.applied }),
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
    operationState.result.ready > 0
  )
  const shouldOfferReprice = Boolean(
    operationState &&
    stateIsCurrent &&
    operationState.phase === 'preview' &&
    operationState.result.ready === 0 &&
    operationState.result.existing > 0 &&
    operationState.result.unresolved === 0 &&
    operationState.result.existing === operationState.result.scanned
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
        <CardTitle>{t('Historical cost backfill')}</CardTitle>
        <CardDescription>
          {t(
            'Preview legacy consumption and refund logs without cost snapshots, then apply the reviewed batch.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form className='flex flex-col gap-5' onSubmit={preview}>
          <CommonOperationFields
            idPrefix={`${id}-backfill`}
            registrations={{
              start_time: form.register('start_time'),
              end_time: form.register('end_time'),
              channel_id: form.register('channel_id', { valueAsNumber: true }),
              cost_discount: form.register('cost_discount'),
              limit: form.register('limit', { valueAsNumber: true }),
            }}
            errors={form.formState.errors}
            maxBatchSize={5000}
          />

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
                label={t('Ready to apply')}
                value={operation.state.result.ready}
                emphasis
              />
              <ResultMetric
                label={t('Existing')}
                value={operation.state.result.existing}
              />
              <ResultMetric
                label={t('Unresolved')}
                value={operation.state.result.unresolved}
              />
              <ResultMetric
                label={t('Cost amount')}
                value={formatLogQuota(operation.state.result.cost_quota)}
              />
              <ResultMetric
                label={t('Turnover')}
                value={formatLogQuota(operation.state.result.revenue_quota)}
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

          {shouldOfferReprice && operation.state && (
            <Alert>
              <AlertTitle>{t('No new snapshots to backfill')}</AlertTitle>
              <AlertDescription className='flex flex-col items-start gap-3'>
                <span>
                  {t(
                    'All {{count}} matching logs already have cost snapshots, so there are no new snapshots to backfill. Continue with cost recalculation to append audited adjustments using the selected discount.',
                    { count: operation.state.result.existing }
                  )}
                </span>
                <Button
                  type='button'
                  variant='outline'
                  onClick={() => props.onRepriceExisting(form.getValues())}
                >
                  {t('Continue to cost recalculation')}
                </Button>
              </AlertDescription>
            </Alert>
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
            <AlertDialogTitle>
              {t('Apply cost snapshot backfill?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This writes {{count}} immutable cost snapshots for batch {{batchId}}. Existing snapshots are not overwritten.',
                {
                  count: operation.state?.result.ready ?? 0,
                  batchId: operation.state?.result.batch_id ?? '',
                }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={apply}>
              {t('Apply snapshots')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Card>
  )
}
