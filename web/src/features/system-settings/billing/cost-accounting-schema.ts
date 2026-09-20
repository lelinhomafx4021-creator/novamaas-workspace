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
import { z } from 'zod'

const accountingDateTimePattern = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/
const costDiscountPattern = /^(?:0(?:\.\d{1,6})?|1(?:\.0{1,6})?)$/

const operationFields = {
  start_time: z
    .string()
    .regex(accountingDateTimePattern, 'Start time is required'),
  end_time: z.string().regex(accountingDateTimePattern, 'End time is required'),
  channel_id: z
    .number()
    .int('Channel ID must be a whole number')
    .min(0, 'Channel ID must be 0 or greater'),
  cost_discount: z
    .string()
    .trim()
    .refine(
      (value) => value === '' || costDiscountPattern.test(value),
      'Cost discount must be between 0 and 1 with up to 6 decimal places'
    ),
}

function validateAccountingRange(
  values: { start_time: string; end_time: string },
  context: z.RefinementCtx
) {
  const start = toAccountingTimestamp(values.start_time)
  const end = toAccountingTimestamp(values.end_time)
  if (start > 0 && end > 0 && end < start) {
    context.addIssue({
      code: z.ZodIssueCode.custom,
      path: ['end_time'],
      message: 'End time must not be earlier than start time',
    })
  }
}

export const costAccountingBackfillSchema = z
  .object({
    ...operationFields,
    limit: z
      .number()
      .int('Batch size must be a whole number')
      .min(1, 'Batch size must be between 1 and 5000')
      .max(5000, 'Batch size must be between 1 and 5000'),
  })
  .superRefine(validateAccountingRange)

export const costAccountingRepriceSchema = z
  .object({
    ...operationFields,
    limit: z
      .number()
      .int('Batch size must be a whole number')
      .min(1, 'Batch size must be between 1 and 1000')
      .max(1000, 'Batch size must be between 1 and 1000'),
    reason: z
      .string()
      .trim()
      .min(1, 'Adjustment reason is required')
      .max(500, 'Adjustment reason must not exceed 500 characters'),
  })
  .superRefine(validateAccountingRange)

export type CostAccountingBackfillValues = z.infer<
  typeof costAccountingBackfillSchema
>
export type CostAccountingRepriceValues = z.infer<
  typeof costAccountingRepriceSchema
>

export function toAccountingTimestamp(value: string): number {
  if (!accountingDateTimePattern.test(value)) return 0
  const timestamp = new Date(`${value}:00+08:00`).getTime()
  return Number.isFinite(timestamp) ? Math.floor(timestamp / 1000) : 0
}

export function getDefaultAccountingRange(now = Date.now()) {
  const chinaTime = new Date(now + 8 * 60 * 60 * 1000)
  const endTime = chinaTime.toISOString().slice(0, 16)
  chinaTime.setUTCDate(chinaTime.getUTCDate() - 7)
  chinaTime.setUTCHours(0, 0, 0, 0)
  return {
    start_time: chinaTime.toISOString().slice(0, 16),
    end_time: endTime,
  }
}
