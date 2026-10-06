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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'

type SMSFields = {
  enabled: boolean
  sign_name: string
  template_code: string
  code_parameter: string
  access_key_id_configured: boolean
  access_key_secret_configured: boolean
  security_token_configured: boolean
}
export type SMSConfiguration = {
  settings: SMSFields
  effective: SMSFields & { ready: boolean; credentials_complete: boolean }
  environment_overrides: Record<string, string>
}
type SMSFormValues = {
  enabled: boolean
  sign_name: string
  template_code: string
  code_parameter: string
  access_key_id: string
  access_key_secret: string
  security_token: string
  clear_security_token: boolean
}

function SMSConfigurationForm({
  configuration,
}: {
  configuration: SMSConfiguration
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const overrides = configuration.environment_overrides
  const defaults: SMSFormValues = {
    enabled: overrides.enabled
      ? configuration.effective.enabled
      : configuration.settings.enabled,
    sign_name: overrides.sign_name
      ? configuration.effective.sign_name
      : configuration.settings.sign_name,
    template_code: overrides.template_code
      ? configuration.effective.template_code
      : configuration.settings.template_code,
    code_parameter: overrides.code_parameter
      ? configuration.effective.code_parameter
      : configuration.settings.code_parameter,
    access_key_id: '',
    access_key_secret: '',
    security_token: '',
    clear_security_token: false,
  }
  const form = useForm<SMSFormValues>({ defaultValues: defaults })
  const save = useMutation({
    mutationFn: async (values: SMSFormValues) => {
      const payload: Partial<SMSFormValues> = {}
      for (const field of [
        'enabled',
        'sign_name',
        'template_code',
        'code_parameter',
      ] as const) {
        if (!overrides[field]) {
          Object.assign(payload, { [field]: values[field] })
        }
      }
      for (const field of [
        'access_key_id',
        'access_key_secret',
        'security_token',
      ] as const) {
        if (
          !overrides[field] &&
          values[field].trim() &&
          !(field === 'security_token' && values.clear_security_token)
        ) {
          payload[field] = values[field].trim()
        }
      }
      if (!overrides.security_token) {
        payload.clear_security_token = values.clear_security_token
      }
      return (
        await api.put<{ data: SMSConfiguration }>('/api/option/sms', payload)
      ).data.data
    },
    onSuccess: (data) => {
      // Clear write-only inputs after saving. The server only returns presence.
      form.reset({
        enabled: data.environment_overrides.enabled
          ? data.effective.enabled
          : data.settings.enabled,
        sign_name: data.environment_overrides.sign_name
          ? data.effective.sign_name
          : data.settings.sign_name,
        template_code: data.environment_overrides.template_code
          ? data.effective.template_code
          : data.settings.template_code,
        code_parameter: data.environment_overrides.code_parameter
          ? data.effective.code_parameter
          : data.settings.code_parameter,
        access_key_id: '',
        access_key_secret: '',
        security_token: '',
        clear_security_token: false,
      })
      queryClient.setQueryData(['sms-configuration'], data)
      void queryClient.invalidateQueries({ queryKey: ['system-options'] })
      void queryClient.invalidateQueries({ queryKey: ['status'] })
      void queryClient.invalidateQueries({ queryKey: ['phone-binding'] })
      try {
        window.localStorage.removeItem('status')
      } catch {
        /* empty */
      }
      toast.success(t('Setting updated successfully'))
    },
    onError: (error) => {
      const code = (error as { response?: { data?: { code?: string } } })
        .response?.data?.code
      let message = t('Failed to update setting')
      if (code === 'SMS_CONFIG_INCOMPLETE') {
        message = t(
          'Complete the access keys, SMS signature and template before enabling SMS.'
        )
      }
      if (code === 'SMS_CONFIG_INVALID') {
        message = t('Check the SMS settings and template variable name.')
      }
      toast.error(message)
    },
  })
  const submit = form.handleSubmit((values) => save.mutate(values))
  let serviceState = t('Configuration incomplete')
  if (configuration.effective.credentials_complete) serviceState = t('Disabled')
  if (configuration.effective.ready) serviceState = t('Ready')
  return (
    <Form {...form}>
      <FormNavigationGuard when={form.formState.isDirty && !save.isPending} />
      <SettingsForm onSubmit={submit}>
        <SettingsPageFormActions
          onSave={submit}
          isSaving={save.isPending}
          isSaveDisabled={!form.formState.isDirty}
        />
        <p className='text-muted-foreground text-sm'>
          {t(
            'Configure Aliyun SMS for phone verification, SMS sign-in and MFA verification. Use an approved signature and verification-code template.'
          )}
        </p>
        <p role='status' className='text-sm'>
          {t('SMS service status')}:{' '}
          <span
            className={
              configuration.effective.ready
                ? 'text-green-600 dark:text-green-400'
                : undefined
            }
          >
            {serviceState}
          </span>
        </p>
        {Object.keys(overrides).length > 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t(
              'Environment variables override saved settings. Change these variables on the server to edit the locked fields.'
            )}
          </p>
        ) : null}
        <FormField
          control={form.control}
          name='enabled'
          render={({ field }) => (
            <SettingsSwitchItem>
              <SettingsSwitchContent>
                <FormLabel>{t('Enable SMS service')}</FormLabel>
                <FormDescription>
                  {overrides.enabled
                    ? t('Configured by {{variable}}', {
                        variable: overrides.enabled,
                      })
                    : t('Only verified phone numbers can sign in with SMS.')}
                </FormDescription>
              </SettingsSwitchContent>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  disabled={save.isPending || !!overrides.enabled}
                />
              </FormControl>
            </SettingsSwitchItem>
          )}
        />
        {(
          [
            {
              name: 'access_key_id',
              label: 'AccessKey ID',
              secret: true,
              limit: 128,
            },
            {
              name: 'access_key_secret',
              label: 'AccessKey Secret',
              secret: true,
              limit: 256,
            },
            {
              name: 'security_token',
              label: 'STS security token (optional)',
              secret: true,
              limit: 4096,
            },
            {
              name: 'sign_name',
              label: 'SMS signature',
              secret: false,
              limit: 128,
            },
            {
              name: 'template_code',
              label: 'SMS template code',
              secret: false,
              limit: 128,
            },
            {
              name: 'code_parameter',
              label: 'Verification-code template variable',
              secret: false,
              limit: 32,
            },
          ] as const
        ).map((item) => {
          let description = ''
          if (item.name === 'code_parameter') {
            description = t(
              'Match the variable in the approved template, for example code. Only this variable is sent.'
            )
          }
          if (item.secret) {
            description = t(
              'Credentials are write-only. Leave blank to keep the saved value.'
            )
          }
          if (overrides[item.name]) {
            description = t('Configured by {{variable}}', {
              variable: overrides[item.name],
            })
          }
          return (
            <FormField
              key={item.name}
              control={form.control}
              name={item.name}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t(item.label)}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type={item.secret ? 'password' : 'text'}
                      autoComplete={item.secret ? 'new-password' : 'off'}
                      maxLength={item.limit}
                      disabled={
                        save.isPending ||
                        !!overrides[item.name] ||
                        (item.name === 'security_token' &&
                          form.watch('clear_security_token'))
                      }
                      placeholder={
                        item.secret &&
                        configuration.effective[
                          `${item.name}_configured` as keyof SMSFields
                        ]
                          ? t(
                              'Configured. Leave blank to keep the saved value.'
                            )
                          : undefined
                      }
                      pattern={
                        item.name === 'code_parameter'
                          ? '[A-Za-z][A-Za-z0-9_]{0,31}'
                          : undefined
                      }
                    />
                  </FormControl>
                  {description ? (
                    <FormDescription>{description}</FormDescription>
                  ) : null}
                </FormItem>
              )}
            />
          )
        })}
        {configuration.settings.security_token_configured &&
        !overrides.security_token ? (
          <FormField
            control={form.control}
            name='clear_security_token'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Remove saved STS token')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Use permanent RAM access keys after removing the temporary STS token.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={save.isPending}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        ) : null}
      </SettingsForm>
    </Form>
  )
}

export function SMSSection() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['sms-configuration'],
    queryFn: async () =>
      (await api.get<{ data: SMSConfiguration }>('/api/option/sms')).data.data,
  })
  return (
    <SettingsSection title={t('SMS service')}>
      {query.isPending ? <p>{t('Loading...')}</p> : null}
      {query.isError ? (
        <div role='alert'>
          <p>{t('Failed to load SMS configuration.')}</p>
          <Button variant='outline' onClick={() => query.refetch()}>
            {t('Retry')}
          </Button>
        </div>
      ) : null}
      {query.data ? <SMSConfigurationForm configuration={query.data} /> : null}
    </SettingsSection>
  )
}
