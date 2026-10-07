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
import { WebhookIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { useWebhookCapabilities } from '../hooks/use-webhook-capabilities'

export function WebhookSettingsButton(props: {
  scope: 'assets' | 'tasks'
  size?: 'default' | 'sm'
  onClick: () => void
}) {
  const { t } = useTranslation()
  const capabilities = useWebhookCapabilities()
  const enabled =
    props.scope === 'assets'
      ? capabilities.data?.asset_library_enabled
      : capabilities.data?.media_tasks_enabled
  if (capabilities.error || (!enabled && !capabilities.data?.manual_enabled)) {
    return null
  }
  return (
    <Button size={props.size} variant='outline' onClick={props.onClick}>
      <HugeiconsIcon icon={WebhookIcon} data-icon='inline-start' />
      {t('Webhooks')}
    </Button>
  )
}
