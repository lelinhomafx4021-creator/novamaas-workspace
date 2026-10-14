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
import { describe, expect, test } from 'vitest'

import {
  createRequestSnapshotPolicySchema,
  requestSnapshotPolicyToForm,
} from '../request-snapshot-schemas'
import { snapshotPolicy, snapshotProfile } from './snapshot-fixtures'

const t = (key: string) => key

describe('request snapshot policy validation', () => {
  test('accepts permanent retention and a disabled unconfigured destination', () => {
    const values = requestSnapshotPolicyToForm(
      snapshotPolicy({ enabled: false, storage_profile_id: 0 })
    )
    expect(
      createRequestSnapshotPolicySchema(t, []).safeParse(values).success
    ).toBe(true)
  })

  test('rejects an enabled policy that references a disabled profile', () => {
    const values = requestSnapshotPolicyToForm(snapshotPolicy())
    expect(
      createRequestSnapshotPolicySchema(t, [
        { ...snapshotProfile, status: 0 },
      ]).safeParse(values).success
    ).toBe(false)
  })

  test.each([
    ['max_file_mib', 513],
    ['max_file_mib', 0],
    ['spool_budget_mib', 65537],
    ['spool_budget_mib', 64],
    ['retention_days', -1],
    ['retention_days', 3651],
  ])('rejects out-of-range %s value %s', (field, value) => {
    const values = {
      ...requestSnapshotPolicyToForm(snapshotPolicy()),
      [field]: value,
    }
    expect(
      createRequestSnapshotPolicySchema(t, [snapshotProfile]).safeParse(values)
        .success
    ).toBe(false)
  })
})
