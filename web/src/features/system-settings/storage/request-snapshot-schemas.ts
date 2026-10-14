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

import type { StoragePolicy, StoragePolicyInput, StorageProfile } from './types'

export const createRequestSnapshotPolicySchema = (
  t: (key: string) => string,
  profiles: StorageProfile[]
) =>
  z
    .object({
      enabled: z.boolean(),
      storage_profile_id: z.number().int().min(0),
      object_prefix: z
        .string()
        .trim()
        .min(1, t('Object prefix is required'))
        .refine(
          (value) => !value.includes('..') && !value.includes('\\'),
          t('Object prefix contains invalid characters')
        ),
      max_file_mib: z.number().min(1).max(512),
      spool_budget_mib: z.number().min(1).max(65536),
      retention_days: z.number().min(0).max(3650),
    })
    .superRefine((value, context) => {
      if (
        value.enabled &&
        !profiles.some(
          (profile) =>
            profile.id === value.storage_profile_id && profile.status === 1
        )
      ) {
        context.addIssue({
          code: 'custom',
          path: ['storage_profile_id'],
          message: t('Select an enabled storage profile'),
        })
      }
      if (value.spool_budget_mib < value.max_file_mib) {
        context.addIssue({
          code: 'custom',
          path: ['spool_budget_mib'],
          message: t('Total size must be at least the per-file size'),
        })
      }
    })

export type RequestSnapshotPolicyFormValues = z.infer<
  ReturnType<typeof createRequestSnapshotPolicySchema>
>

export function requestSnapshotPolicyToForm(
  policy: StoragePolicy
): RequestSnapshotPolicyFormValues {
  return {
    enabled: policy.enabled,
    storage_profile_id: policy.storage_profile_id,
    object_prefix: policy.object_prefix,
    max_file_mib: policy.max_file_bytes / 1024 / 1024,
    spool_budget_mib: policy.max_total_bytes / 1024 / 1024,
    retention_days: policy.retention_seconds / 86400,
  }
}

export function requestSnapshotPolicyToInput(
  values: RequestSnapshotPolicyFormValues,
  allowedMimeTypes: string
): StoragePolicyInput {
  return {
    enabled: values.enabled,
    storage_profile_id: values.storage_profile_id,
    object_prefix: values.object_prefix.trim().replaceAll(/^\/+|\/+$/g, ''),
    max_file_bytes: Math.round(values.max_file_mib * 1024 * 1024),
    max_total_bytes: Math.round(values.spool_budget_mib * 1024 * 1024),
    retention_seconds: Math.round(values.retention_days * 86400),
    max_files: 2,
    signed_url_ttl_seconds: 0,
    allowed_mime_types: allowedMimeTypes,
  }
}
