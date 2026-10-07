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

export function webhookFormSchema(t: TFunction) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, t('Endpoint name is required'))
      .max(
        64,
        t('Name must be between {{min}} and {{max}} characters', {
          min: 1,
          max: 64,
        })
      ),
    url: z
      .string()
      .trim()
      .max(2048, t('Enter a valid HTTPS callback URL'))
      .url(t('Enter a valid HTTPS callback URL'))
      .refine((value) => {
        try {
          const url = new URL(value)
          return (
            url.protocol === 'https:' &&
            !url.username &&
            !url.password &&
            !url.hash
          )
        } catch {
          return false
        }
      }, t('Enter a valid HTTPS callback URL')),
    event_types: z
      .array(z.enum(['asset.active', 'asset.failed', 'task.status_changed']))
      .min(1, t('Select at least one event')),
  })
}
