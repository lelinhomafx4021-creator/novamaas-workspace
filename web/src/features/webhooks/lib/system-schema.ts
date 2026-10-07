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
import type { TFunction } from 'i18next'
import * as z from 'zod'

import { webhookFormSchema } from './form-schema'

export function systemWebhookSchema(t: TFunction) {
  const urlSchema = webhookFormSchema(t).shape.url
  const target = z
    .object({ enabled: z.boolean(), url: z.string().trim() })
    .superRefine((value, ctx) => {
      if (!value.url) return
      if (!urlSchema.safeParse(value.url).success) {
        ctx.addIssue({
          code: 'custom',
          path: ['url'],
          message: t('Enter a valid HTTPS callback URL'),
        })
      }
    })
  return z.object({
    asset_library: target,
    media_tasks: target,
    manual_enabled: z.boolean(),
  })
}
