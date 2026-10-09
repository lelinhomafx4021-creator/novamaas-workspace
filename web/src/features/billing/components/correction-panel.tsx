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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'
import {
  SecureVerificationDialog,
  useSecureVerification,
} from '@/features/auth/secure-verification'
import { handleServerError } from '@/lib/handle-server-error'

import { billingToday, billingPreviousMonth } from '../api'
import {
  actOnCorrection,
  correctionGroups,
  getCorrection,
  listCorrections,
  previewCorrection,
  type CorrectionBatch,
  type CorrectionInput,
} from '../correction-api'
import { CorrectionHistory } from './correction-history'
import { CorrectionReview } from './correction-review'

const schema = z
  .object({
    mode: z.enum(['group_rate', 'model_pricing']),
    start: z.string().min(10),
    end: z.string().min(10),
    models: z.string().trim().min(1).max(5100),
    group: z.string(),
    reason: z.string().trim().min(4).max(1000),
  })
  .superRefine((value, context) => {
    if (value.mode === 'group_rate' && !value.group) {
      context.addIssue({
        code: 'custom',
        path: ['group'],
        message: 'Select a billing group',
      })
    }
  })
type Values = z.infer<typeof schema>

function correctionSelection(userId: number, value: Values): CorrectionInput {
  return {
    mode: value.mode,
    user_id: userId,
    start_at: Date.parse(`${value.start}T00:00:00+08:00`) / 1000,
    end_at: Date.parse(`${value.end}T00:00:00+08:00`) / 1000 + 86400,
    models: [
      ...new Set(
        value.models
          .split(/[,\n]/)
          .map((model) => model.trim())
          .filter(Boolean)
      ),
    ].sort(),
    target_group: value.mode === 'group_rate' ? value.group : '',
    reason: value.reason.trim(),
  }
}

export function CorrectionPanel(props: {
  userId: number
  actorId: number
  canManage: boolean
}) {
  const { t } = useTranslation()
  const id = useId()
  const active = useRef(true)
  useEffect(() => {
    active.current = true
    return () => {
      active.current = false
    }
  }, [])
  const queryClient = useQueryClient()
  const today = billingToday()
  const end = new Date(Date.parse(`${today.slice(0, 7)}-01T00:00:00+08:00`) - 1)
    .toISOString()
    .slice(0, 10)
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      mode: 'group_rate',
      start: `${billingPreviousMonth()}-01`,
      end,
      models: '',
      group: '',
      reason: '',
    },
  })
  const values = form.watch()
  const [result, setResult] = useState<{
    batch: CorrectionBatch
    signature: string
  } | null>(null)
  const [confirm, setConfirm] = useState('')
  const [reverseReason, setReverseReason] = useState('')
  const groups = useQuery({
    queryKey: ['billing', 'correction-groups', props.userId],
    queryFn: () => correctionGroups(props.userId),
    enabled: props.canManage && values.mode === 'group_rate',
  })
  const history = useQuery({
    queryKey: ['billing', 'corrections', props.userId],
    queryFn: () => listCorrections(props.userId),
  })
  const verification = useSecureVerification()
  const preview = useMutation({
    mutationFn: (value: Values) =>
      previewCorrection(correctionSelection(props.userId, value)),
    onSuccess: (batch, value) => {
      setResult({
        batch,
        signature: JSON.stringify(correctionSelection(props.userId, value)),
      })
      setConfirm('')
      void history.refetch()
    },
    onError: handleServerError,
  })
  const load = useMutation({
    mutationFn: getCorrection,
    onSuccess: (batch) => {
      const value: Values = {
        mode: batch.mode ?? 'group_rate',
        start: new Date((batch.start_at + 8 * 3600) * 1000)
          .toISOString()
          .slice(0, 10),
        end: new Date((batch.end_at - 1 + 8 * 3600) * 1000)
          .toISOString()
          .slice(0, 10),
        models: (JSON.parse(batch.models) as string[]).join('\n'),
        group: batch.target_group,
        reason: batch.reason,
      }
      form.reset(value)
      setResult({
        batch,
        signature: JSON.stringify({
          ...correctionSelection(props.userId, value),
          start_at: batch.start_at,
          end_at: batch.end_at,
        }),
      })
      setConfirm('')
      setReverseReason('')
    },
    onError: handleServerError,
  })
  const action = useMutation({
    mutationFn: (input: {
      batch: CorrectionBatch
      reverse: boolean
      proof?: string
    }) =>
      actOnCorrection(
        input.batch.id,
        {
          action: input.reverse ? 'reverse' : 'apply',
          sha256: input.batch.sha256,
          confirm_user_id: props.userId,
          confirm_net_delta:
            input.reverse && input.batch.net_delta !== 0
              ? -input.batch.net_delta
              : input.batch.net_delta,
          reason: input.reverse ? reverseReason : '',
        },
        input.proof
      ),
    onSuccess: (batch) => {
      setResult({ batch, signature: '' })
      setConfirm('')
      setReverseReason('')
      toast.success(t('Adjustment completed'))
      void queryClient.invalidateQueries()
    },
    onError: handleServerError,
  })
  const pending =
    preview.isPending ||
    action.isPending ||
    verification.isLoading ||
    verification.open ||
    load.isPending
  const batch = result?.batch
  const current =
    result?.signature ===
    JSON.stringify(correctionSelection(props.userId, values))
  const canAct = Boolean(
    props.canManage &&
    batch &&
    batch.user_id === props.userId &&
    batch.created_by === props.actorId &&
    batch.can_apply &&
    confirm === String(props.userId) &&
    !pending &&
    ((batch.status === 'preview' &&
      current &&
      batch.expires_at > Date.now() / 1000) ||
      (batch.status === 'applied' && reverseReason.trim().length >= 4))
  )
  function execute() {
    if (!batch || !canAct) return
    void verification
      .startVerification(
        async (proof) => {
          if (
            !active.current ||
            (batch.status === 'preview' &&
              result?.signature !==
                JSON.stringify(
                  correctionSelection(props.userId, form.getValues())
                ))
          ) {
            throw new Error(t('Selection changed. Preview again.'))
          }
          return action.mutateAsync({
            batch,
            reverse: batch.status === 'applied',
            proof,
          })
        },
        {
          scope: 'billing.correct',
          title: 'Confirm billing adjustment',
          description: t(
            'The wallet changes only by the net sales adjustment. Cost corrections do not add wallet charges.'
          ),
        }
      )
      .catch(handleServerError)
  }
  return (
    <div className='flex flex-col gap-4'>
      <h2 className='font-semibold'>{t('Billing adjustments')}</h2>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Adjust completed wallet tasks by billing group or current model pricing. Original evidence is preserved; users see effective amounts.'
        )}
      </p>
      <p className='text-sm'>
        {t(
          'Corrections belong to the original consumption or refund month. Void the current statement, then create a new draft from the latest accounting data.'
        )}
      </p>
      <p className='text-sm'>
        {t(
          'Include the full consumption and refund lifecycle. Up to 20 models, 93 days and 1000 records per batch.'
        )}
      </p>
      {props.canManage && (
        <form
          className='flex flex-col gap-4'
          onSubmit={form.handleSubmit((value) => preview.mutate(value))}
        >
          <FieldGroup className='grid gap-4 sm:grid-cols-2'>
            <Field className='sm:col-span-2'>
              <FieldLabel htmlFor={`${id}-mode`}>
                {t('Adjustment type')}
              </FieldLabel>
              <NativeSelect
                id={`${id}-mode`}
                className='w-full'
                disabled={pending}
                {...form.register('mode')}
              >
                <NativeSelectOption value='group_rate'>
                  {t('Billing group adjustment')}
                </NativeSelectOption>
                <NativeSelectOption value='model_pricing'>
                  {t('Model pricing adjustment')}
                </NativeSelectOption>
              </NativeSelect>
              <p className='text-muted-foreground text-sm'>
                {values.mode === 'model_pricing'
                  ? t(
                      'Recalculate selected models using current pricing and existing billing groups. Resolution and input-video conditions come from saved task evidence.'
                    )
                  : t(
                      'Change the billing group discount while keeping the historical model pricing. The upstream channel is unchanged.'
                    )}
              </p>
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.start)}>
              <FieldLabel htmlFor={`${id}-start`}>{t('Start date')}</FieldLabel>
              <Input
                id={`${id}-start`}
                aria-invalid={Boolean(form.formState.errors.start)}
                type='date'
                disabled={pending}
                {...form.register('start')}
              />
            </Field>
            <Field data-invalid={Boolean(form.formState.errors.end)}>
              <FieldLabel htmlFor={`${id}-end`}>{t('End date')}</FieldLabel>
              <Input
                id={`${id}-end`}
                aria-invalid={Boolean(form.formState.errors.end)}
                type='date'
                disabled={pending}
                {...form.register('end')}
              />
            </Field>
            {values.mode === 'group_rate' && (
              <Field data-invalid={Boolean(form.formState.errors.group)}>
                <FieldLabel htmlFor={`${id}-group`}>
                  {t('Target billing group')}
                </FieldLabel>
                <NativeSelect
                  id={`${id}-group`}
                  aria-invalid={Boolean(form.formState.errors.group)}
                  className='w-full'
                  disabled={pending || groups.isPending}
                  {...form.register('group')}
                >
                  <NativeSelectOption value=''>
                    {t('Select a billing group')}
                  </NativeSelectOption>
                  {groups.data?.map((item) => (
                    <NativeSelectOption key={item.group} value={item.group}>
                      {item.group} / {(Number(item.rate) * 100).toFixed(2)}%
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              </Field>
            )}
            <Field data-invalid={Boolean(form.formState.errors.models)}>
              <FieldLabel htmlFor={`${id}-models`}>
                {t('Models to correct')}
              </FieldLabel>
              <Textarea
                id={`${id}-models`}
                aria-invalid={Boolean(form.formState.errors.models)}
                disabled={pending}
                placeholder={t('One model per line, or separated by commas')}
                {...form.register('models')}
              />
            </Field>
            <Field
              className='sm:col-span-2'
              data-invalid={Boolean(form.formState.errors.reason)}
            >
              <FieldLabel htmlFor={`${id}-reason`}>
                {t('Adjustment reason')}
              </FieldLabel>
              <Textarea
                id={`${id}-reason`}
                aria-invalid={Boolean(form.formState.errors.reason)}
                disabled={pending}
                {...form.register('reason')}
              />
              <FieldError>
                {form.formState.errors.reason &&
                  t(
                    'Provide an adjustment reason of at least four characters.'
                  )}
              </FieldError>
            </Field>
          </FieldGroup>
          {groups.isError && <p role='alert'>{t('Failed to load data')}</p>}
          <Button
            type='submit'
            className='self-start'
            disabled={pending || groups.isError}
          >
            {t('Preview adjustment')}
          </Button>
        </form>
      )}
      {batch && (
        <>
          <CorrectionReview key={batch.id} batch={batch} />
          {props.canManage && batch.status === 'preview' && !current && (
            <p role='alert'>{t('Selection changed. Preview again.')}</p>
          )}
          {props.canManage && batch.status !== 'reversed' && (
            <FieldGroup>
              {batch.status === 'applied' && (
                <Field>
                  <FieldLabel htmlFor={`${id}-reverse`}>
                    {t('Reversal reason')}
                  </FieldLabel>
                  <Textarea
                    id={`${id}-reverse`}
                    disabled={pending}
                    maxLength={1000}
                    value={reverseReason}
                    onChange={(event) => setReverseReason(event.target.value)}
                  />
                </Field>
              )}
              <Field>
                <FieldLabel htmlFor={`${id}-confirm`}>
                  {t('Type the account ID to confirm')}
                </FieldLabel>
                <Input
                  id={`${id}-confirm`}
                  disabled={pending}
                  value={confirm}
                  onChange={(event) => setConfirm(event.target.value)}
                  placeholder={String(props.userId)}
                />
              </Field>
              <Button
                type='button'
                variant='destructive'
                className='self-start'
                disabled={!canAct}
                onClick={execute}
              >
                {batch.status === 'applied'
                  ? t('Reverse adjustment')
                  : t('Apply adjustment')}
              </Button>
            </FieldGroup>
          )}
        </>
      )}
      <CorrectionHistory
        items={history.data ?? []}
        loading={history.isPending}
        failed={history.isError}
        refreshing={history.isFetching}
        disabled={pending}
        onRefresh={() => void history.refetch()}
        onSelect={(id) => load.mutate(id)}
      />
      <SecureVerificationDialog
        open={verification.open}
        onOpenChange={verification.setOpen}
        methods={verification.methods}
        state={verification.state}
        onVerify={(method, code) => {
          void verification
            .executeVerification(method, code)
            .catch(() => undefined)
        }}
        onCancel={verification.cancel}
        onCodeChange={verification.setCode}
        onMethodChange={verification.switchMethod}
      />
    </div>
  )
}
