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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { getServerErrorMessageKey } from '@/lib/server-error-message'

import { downloadStatement } from '../api'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))

describe('statement downloads', () => {
  let downloadedName = ''

  beforeEach(() => {
    downloadedName = ''
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      function (this: HTMLAnchorElement) {
        downloadedName = this.download
      }
    )
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:statement'),
      revokeObjectURL: vi.fn(),
    })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
  })

  test.each([
    ['pdf', 'demo_customer_2026-08_月度对账单_V01.pdf'],
    ['receipt', 'demo_customer_2026-08_对账确认回执_V01.pdf'],
  ])(
    'uses the server customer and billing month filename for %s',
    async (kind, name) => {
      vi.mocked(api.get).mockResolvedValue({
        data: new Blob(['PDF'], { type: 'application/pdf' }),
        headers: {
          'content-disposition': `attachment; filename="fallback.pdf"; filename*=UTF-8''${encodeURIComponent(name)}`,
        },
      })

      await downloadStatement('internal-uuid', kind)

      expect(downloadedName).toBe(name)
      expect(downloadedName).not.toContain('internal-uuid')
      expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:statement')
    }
  )

  test('honors quoted filenames for non-PDF evidence', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: new Blob(['{}']),
      headers: {
        'content-disposition':
          'attachment; filename="statement-2026-08-r1-manifest-0.json"',
      },
    })
    await downloadStatement('internal-uuid', 'manifest')
    expect(downloadedName).toBe('statement-2026-08-r1-manifest-0.json')
  })

  test('falls back without leaking the UUID when a filename header is malformed', async () => {
    vi.spyOn(Date, 'now').mockReturnValue(1788364800000)
    vi.mocked(api.get).mockResolvedValue({
      data: new Blob(['PDF']),
      headers: { 'content-disposition': "attachment; filename*=UTF-8''%ZZ" },
    })
    await downloadStatement('internal-uuid', 'pdf')
    expect(downloadedName).toMatch(/_1788364800000\.pdf$/)
    expect(downloadedName).not.toContain('internal-uuid')
  })

  test('explains which settings to check when PDF branding cannot be loaded', () => {
    expect(
      getServerErrorMessageKey({
        code: 'BILLING_BRANDING_INVALID',
        message: 'internal logo URL or network information',
      })
    ).toBe(
      'Unable to prepare PDF branding. Check the system Logo URL and Footer settings, then try again.'
    )
  })

  test('explains DNS and proxy checks when the PDF logo fetch is blocked', () => {
    expect(
      getServerErrorMessageKey({
        code: 'BILLING_BRANDING_FETCH_BLOCKED',
        message: 'private IP address not allowed: internal detail',
      })
    ).toBe(
      'The server blocked a PDF logo download. Check the server DNS and proxy settings so the configured image domain resolves to a public IP.'
    )
  })
})
