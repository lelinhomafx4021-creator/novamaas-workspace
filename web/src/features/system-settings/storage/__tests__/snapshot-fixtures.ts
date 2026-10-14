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
import type {
  RequestSnapshotStatus,
  StoragePolicy,
  StorageProfile,
} from '../types'

export const snapshotProfile: StorageProfile = {
  id: 7,
  name: 'Private audit archive',
  provider_type: 'test_provider',
  status: 1,
  endpoint: 'https://storage.example.test',
  region: 'test-region',
  bucket: 'audit-snapshots',
  auth_type: 'environment',
  credential_configured: true,
  access_key_hint: '',
  created_at: 0,
  updated_at: 0,
}

export function snapshotPolicy(
  overrides: Partial<StoragePolicy> = {}
): StoragePolicy {
  return {
    id: 1,
    key: 'request_snapshot',
    name: 'Request snapshot storage',
    purpose: 'request_snapshot',
    storage_profile_id: 7,
    object_prefix: 'audit/request-snapshots',
    signed_url_ttl_seconds: 0,
    retention_seconds: 0,
    max_file_bytes: 128 * 1024 * 1024,
    max_total_bytes: 2 * 1024 * 1024 * 1024,
    max_files: 2,
    allowed_mime_types: 'application/json',
    enabled: true,
    created_at: 0,
    updated_at: 0,
    ...overrides,
  }
}

export function snapshotStatus(
  overrides: Partial<RequestSnapshotStatus> = {}
): RequestSnapshotStatus {
  return {
    node_name: 'dev-node-1',
    spool_directory: '/private/audit-spool',
    workers: 2,
    pending_files: 3,
    pending_bytes: 5 * 1024 * 1024,
    migration: {
      status: 'idle',
      mode: 'migrate',
      cursor: 0,
      processed: 0,
      failed: 0,
    },
    ...overrides,
  }
}
