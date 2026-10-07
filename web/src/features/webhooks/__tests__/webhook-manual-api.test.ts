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
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { downloadWebhookManual } from '../api'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))

test.each(['asset_library', 'media_tasks'] as const)(
  '%s manual downloads only its selected category',
  async (scope) => {
    const blob = new Blob(['%PDF-1.7'], { type: 'application/pdf' })
    vi.mocked(api.get).mockResolvedValueOnce({ data: blob })
    expect(await downloadWebhookManual(scope)).toBe(blob)
    expect(api.get).toHaveBeenCalledWith('/api/webhooks/manual.pdf', {
      responseType: 'blob',
      params: { scope },
    })
  }
)
