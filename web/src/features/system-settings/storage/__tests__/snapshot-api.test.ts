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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  getRequestSnapshotStatus,
  getRequestSnapshotStoragePolicy,
  startRequestSnapshotCleanup,
  startRequestSnapshotMigration,
  updateRequestSnapshotStoragePolicy,
} from '../api'
import {
  requestSnapshotPolicyToForm,
  requestSnapshotPolicyToInput,
} from '../request-snapshot-schemas'
import { snapshotPolicy } from './snapshot-fixtures'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), put: vi.fn() },
}))

beforeEach(() => {
  vi.resetAllMocks()
  vi.mocked(api.get).mockResolvedValue({ data: { success: true, data: {} } })
  vi.mocked(api.post).mockResolvedValue({ data: { success: true, data: {} } })
  vi.mocked(api.put).mockResolvedValue({ data: { success: true, data: {} } })
})

describe('request snapshot management API', () => {
  test('reads and updates the dedicated snapshot policy endpoint', async () => {
    await getRequestSnapshotStoragePolicy()
    expect(api.get).toHaveBeenCalledWith(
      '/api/storage/policies/request-snapshots'
    )
    const input = requestSnapshotPolicyToInput(
      requestSnapshotPolicyToForm(snapshotPolicy()),
      'application/json'
    )
    await updateRequestSnapshotStoragePolicy(input)
    expect(api.put).toHaveBeenCalledWith(
      '/api/storage/policies/request-snapshots',
      input,
      expect.any(Object)
    )
  })

  test('loads current node status without placing its spool directory in policy settings', async () => {
    await getRequestSnapshotStatus()
    expect(api.get).toHaveBeenCalledWith(
      '/api/storage/request-snapshots/status',
      expect.any(Object)
    )
  })

  test('starts background migration with database-body clearing explicitly disabled', async () => {
    await startRequestSnapshotMigration()
    expect(api.post).toHaveBeenCalledWith(
      '/api/storage/request-snapshots/migrate',
      { batch_size: 50, clear_database_body: false },
      expect.any(Object)
    )
  })

  test('starts verified database cleanup independently from migration', async () => {
    await startRequestSnapshotCleanup()
    expect(api.post).toHaveBeenCalledWith(
      '/api/storage/request-snapshots/cleanup',
      { batch_size: 50 },
      expect.any(Object)
    )
  })
})
