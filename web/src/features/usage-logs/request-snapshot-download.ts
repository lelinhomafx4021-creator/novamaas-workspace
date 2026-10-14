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
import i18next from 'i18next'

import { api } from '@/lib/api'

export function parseRequestSnapshotDownloadURL(value: string): string | null {
  if (!value.startsWith('/api/') || value.includes('#')) return null
  const url = new URL(value, 'https://snapshot.invalid')
  if (
    /^\/api\/task\/[^/?#\\]+\/request-snapshots\/(original|upstream)$/.test(
      url.pathname
    )
  ) {
    return url.search === '' ? url.pathname : null
  }
  if (
    !/^\/api\/log\/request-snapshots\/(original|upstream)$/.test(url.pathname)
  ) {
    return null
  }
  const keys = [...url.searchParams.keys()]
  if (keys.length === 0 || new Set(keys).size !== keys.length) return null
  if (
    keys.some(
      (key) =>
        !['task_id', 'request_id'].includes(key) ||
        !url.searchParams.get(key)?.trim()
    )
  ) {
    return null
  }
  return url.pathname + url.search
}

export function requestSnapshotPageURL(value: string): string | null {
  const endpoint = parseRequestSnapshotDownloadURL(value)
  if (!endpoint) return null
  const pageURL = new URL('/request-snapshots', window.location.origin)
  pageURL.searchParams.set('url', endpoint)
  return pageURL.href
}

export async function downloadRequestSnapshot(value: string): Promise<void> {
  const endpoint = parseRequestSnapshotDownloadURL(value)
  if (!endpoint) throw new Error(i18next.t('Invalid request snapshot link'))
  const response = await api.get<Blob>(endpoint, {
    responseType: 'blob',
    disableDuplicate: true,
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  const blobURL = URL.createObjectURL(response.data)
  try {
    const anchor = document.createElement('a')
    anchor.href = blobURL
    anchor.download = endpoint.split('?')[0].endsWith('/upstream')
      ? 'upstream-request.json'
      : 'original-request.json'
    anchor.click()
  } finally {
    URL.revokeObjectURL(blobURL)
  }
}
