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
import { useId } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Field,
  FieldContent,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import {
  createRequestSnapshotPolicySchema,
  requestSnapshotPolicyToForm,
  type RequestSnapshotPolicyFormValues,
} from './request-snapshot-schemas'
import type { StoragePolicy, StorageProfile } from './types'

export function RequestSnapshotPolicyForm(props: {
  policy: StoragePolicy
  profiles: StorageProfile[]
  isSaving: boolean
  onSave: (values: RequestSnapshotPolicyFormValues) => void
}) {
  const { t } = useTranslation()
  const formId = useId()
  const form = useForm<RequestSnapshotPolicyFormValues>({
    resolver: zodResolver(createRequestSnapshotPolicySchema(t, props.profiles)),
    defaultValues: requestSnapshotPolicyToForm(props.policy),
  })
  const numberFields = [
    {
      name: 'max_file_mib',
      label: t('Maximum snapshot size (MiB)'),
      min: 1,
      max: 512,
    },
    {
      name: 'spool_budget_mib',
      label: t('Local queue budget (MiB)'),
      description: t('Maximum queued snapshot data on each node.'),
      min: 1,
      max: 65536,
    },
    {
      name: 'retention_days',
      label: t('Snapshot retention (days)'),
      description: t('Set to 0 to retain snapshots permanently.'),
      min: 0,
      max: 3650,
    },
  ] as const

  return (
    <Card role='region' aria-labelledby={`${formId}-title`}>
      <CardHeader>
        <CardTitle id={`${formId}-title`}>
          {t('Snapshot persistence')}
        </CardTitle>
        <CardDescription>
          {t(
            'Archives original and upstream JSON media requests in private object storage for administrator auditing.'
          )}
        </CardDescription>
      </CardHeader>
      <form onSubmit={form.handleSubmit(props.onSave)}>
        <CardContent>
          <FieldSet disabled={props.isSaving}>
            <FieldGroup>
              <Controller
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <Field
                    orientation='horizontal'
                    data-disabled={props.isSaving}
                  >
                    <FieldContent>
                      <FieldLabel htmlFor={`${formId}-enabled`}>
                        {t('Enable snapshot persistence')}
                      </FieldLabel>
                      <FieldDescription>
                        {t(
                          'Enabled snapshot persistence removes the 2 MiB database snapshot limit. The limit still applies when persistence is disabled.'
                        )}
                      </FieldDescription>
                    </FieldContent>
                    <Switch
                      id={`${formId}-enabled`}
                      checked={field.value}
                      disabled={props.isSaving}
                      onCheckedChange={field.onChange}
                    />
                  </Field>
                )}
              />
              <Controller
                control={form.control}
                name='storage_profile_id'
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor={`${formId}-profile`}>
                      {t('Storage profile')}
                    </FieldLabel>
                    <NativeSelect
                      id={`${formId}-profile`}
                      className='w-full'
                      aria-invalid={fieldState.invalid}
                      aria-describedby={
                        fieldState.invalid
                          ? `${formId}-profile-error`
                          : undefined
                      }
                      value={String(field.value)}
                      onBlur={field.onBlur}
                      onChange={(event) =>
                        field.onChange(Number(event.target.value))
                      }
                    >
                      <NativeSelectOption value='0'>
                        {t('Select a profile')}
                      </NativeSelectOption>
                      {props.profiles.map((profile) => (
                        <NativeSelectOption
                          key={profile.id}
                          value={String(profile.id)}
                          disabled={profile.status !== 1}
                        >
                          {profile.name}
                          {profile.status !== 1 ? ` (${t('Disabled')})` : ''}
                        </NativeSelectOption>
                      ))}
                    </NativeSelect>
                    <FieldError
                      id={`${formId}-profile-error`}
                      errors={[fieldState.error]}
                    />
                  </Field>
                )}
              />
              <Controller
                control={form.control}
                name='object_prefix'
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor={`${formId}-prefix`}>
                      {t('Object prefix')}
                    </FieldLabel>
                    <Input
                      {...field}
                      id={`${formId}-prefix`}
                      aria-invalid={fieldState.invalid}
                      aria-describedby={
                        fieldState.invalid
                          ? `${formId}-prefix-error`
                          : undefined
                      }
                    />
                    <FieldError
                      id={`${formId}-prefix-error`}
                      errors={[fieldState.error]}
                    />
                  </Field>
                )}
              />
              <FieldGroup className='grid gap-4 sm:grid-cols-2'>
                {numberFields.map((setting) => (
                  <Controller
                    key={setting.name}
                    control={form.control}
                    name={setting.name}
                    render={({ field, fieldState }) => (
                      <Field data-invalid={fieldState.invalid}>
                        <FieldLabel htmlFor={`${formId}-${setting.name}`}>
                          {setting.label}
                        </FieldLabel>
                        <Input
                          {...field}
                          id={`${formId}-${setting.name}`}
                          type='number'
                          min={setting.min}
                          max={setting.max}
                          step={1}
                          aria-invalid={fieldState.invalid}
                          aria-describedby={
                            fieldState.invalid
                              ? `${formId}-${setting.name}-error`
                              : undefined
                          }
                          onChange={(event) =>
                            field.onChange(event.target.valueAsNumber)
                          }
                        />
                        {'description' in setting && (
                          <FieldDescription
                            id={`${formId}-${setting.name}-help`}
                          >
                            {setting.description}
                          </FieldDescription>
                        )}
                        <FieldError
                          id={`${formId}-${setting.name}-error`}
                          errors={[fieldState.error]}
                        />
                      </Field>
                    )}
                  />
                ))}
              </FieldGroup>
            </FieldGroup>
          </FieldSet>
        </CardContent>
        <CardFooter className='mt-4 justify-end'>
          <Button type='submit' disabled={props.isSaving}>
            {props.isSaving && (
              <Spinner data-icon='inline-start' aria-hidden='true' />
            )}
            {props.isSaving ? t('Saving...') : t('Save snapshot policy')}
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}
