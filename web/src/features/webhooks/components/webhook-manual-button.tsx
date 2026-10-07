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
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import { downloadWebhookManual } from '../api'
import { useWebhookCapabilities } from '../hooks/use-webhook-capabilities'
import { webhookErrorMessage } from '../lib/response'
import type { SystemWebhookTopic } from '../types'

export function WebhookManualButton(props: {
  scope: SystemWebhookTopic
  active?: boolean
}) {
  const { t } = useTranslation()
  const capabilities = useWebhookCapabilities(props.active ?? true)
  const download = useMutation({
    mutationFn: () => downloadWebhookManual(props.scope),
    onSuccess: (blob) => {
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download =
        props.scope === 'asset_library'
          ? 'asset-library-webhook-api-manual.pdf'
          : 'media-task-webhook-api-manual.pdf'
      link.click()
      URL.revokeObjectURL(url)
    },
    onError: (error) => toast.error(webhookErrorMessage(error)),
  })
  if (capabilities.error || !capabilities.data?.manual_enabled) return null
  return (
    <Button
      type='button'
      variant='outline'
      disabled={download.isPending}
      onClick={() => download.mutate()}
    >
      {download.isPending && <Spinner data-icon='inline-start' />}
      {props.scope === 'asset_library'
        ? t('Download asset webhook manual')
        : t('Download media task webhook manual')}
    </Button>
  )
}
