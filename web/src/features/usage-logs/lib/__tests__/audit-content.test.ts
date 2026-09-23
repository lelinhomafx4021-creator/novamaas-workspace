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
import { describe, expect, test, vi } from 'vitest'

import { renderAuditContent } from '../format'

describe('asset library audit content', () => {
  test('renders an AK/SK upload as a localized usage-log operation', () => {
    const t = vi.fn(() => 'localized upload')

    const result = renderAuditContent(
      {
        op: {
          action: 'asset.upload_aksk',
          params: {
            id: 'asset-123',
            name: 'product photo',
            accessKeyName: 'production uploader',
          },
        },
      },
      t
    )

    expect(result).toBe('localized upload')
    expect(t).toHaveBeenCalledWith(
      'Uploaded asset {{name}} via AK/SK access key {{accessKeyName}} (ID: {{id}})',
      {
        id: 'asset-123',
        name: 'product photo',
        accessKeyName: 'production uploader',
      }
    )
  })
})
