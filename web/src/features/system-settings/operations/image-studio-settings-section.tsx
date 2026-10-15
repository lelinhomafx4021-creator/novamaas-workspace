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
import { Link } from '@tanstack/react-router'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

import { Form, FormControl, FormField, FormLabel } from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({ enabled: z.boolean() })
type Values = z.infer<typeof schema>

export function ImageStudioSettingsSection(props: { defaultEnabled: boolean }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const defaults = { enabled: props.defaultEnabled }
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: defaults,
  })
  useResetForm(form, defaults)
  const submit = form.handleSubmit(async (values) => {
    if (values.enabled === props.defaultEnabled) return
    await update.mutateAsync({
      key: 'image_studio_setting.enabled',
      value: values.enabled,
    })
  })

  return (
    <SettingsSection title={t('Image generation')}>
      <p className='text-muted-foreground text-sm'>
        {t(
          'Generated images are stored in OSS for 30 days, then automatically deleted. Enable the asset library storage policy before turning it on.'
        )}{' '}
        <Link
          className='text-primary underline underline-offset-4'
          to='/system-settings/storage/$section'
          params={{ section: 'asset-library' }}
        >
          {t('Permanent asset storage')}
        </Link>
      </p>
      <Form {...form}>
        <SettingsForm onSubmit={submit}>
          <SettingsPageFormActions
            onSave={submit}
            isSaving={update.isPending}
          />
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable image generation')}</FormLabel>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
