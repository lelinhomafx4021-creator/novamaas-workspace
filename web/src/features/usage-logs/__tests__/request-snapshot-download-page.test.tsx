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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { RequestSnapshotDownloadPage } from '../request-snapshot-download-page'

beforeEach(() => {
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 1, username: 'admin', role: 10 },
      accessToken: 'admin-token',
    },
  }))
  vi.spyOn(api, 'get').mockResolvedValue({
    data: new Blob(['{"prompt":"archive"}']),
  })
  vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:snapshot')
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
    () => undefined
  )
})
afterEach(() => {
  useAuthStore.getState().auth.reset('idle')
  vi.restoreAllMocks()
})

describe('permanent browser snapshot download page', () => {
  test('allows an authenticated administrator to repeatedly download a log-linked snapshot', async () => {
    render(
      <RequestSnapshotDownloadPage url='/api/log/request-snapshots/upstream?request_id=request_1' />
    )
    expect(
      screen.getByText(
        'This link can be used again after administrator sign-in. Downloads use your current authenticated session.'
      )
    ).toBeVisible()
    fireEvent.click(
      screen.getByRole('button', { name: 'Download archived JSON' })
    )
    await waitFor(() =>
      expect(api.get).toHaveBeenCalledWith(
        '/api/log/request-snapshots/upstream?request_id=request_1',
        expect.objectContaining({ responseType: 'blob' })
      )
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Download archived JSON' })
      ).toBeEnabled()
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Download archived JSON' })
    )
    await waitFor(() => expect(api.get).toHaveBeenCalledTimes(2))
  })

  test.each([
    'https://outside.example/api/task/task_1/request-snapshots/original',
    '//outside.example/api/task/task_1/request-snapshots/original',
    '/api/storage/profiles/7',
    '/api/task/../request-snapshots/original',
    '/api/task/task_1/request-snapshots/original?access_token=secret',
    '/api/log/request-snapshots/upstream?request_id=r1&redirect=https%3A%2F%2Foutside.example',
    '/api/log/request-snapshots/upstream?request_id=r1&request_id=r2',
    '/api/log/request-snapshots/upstream',
  ])(
    'rejects unsupported URL %s without sending an authenticated request',
    (url) => {
      render(<RequestSnapshotDownloadPage url={url} />)
      expect(screen.getByText('Invalid request snapshot link')).toBeVisible()
      expect(
        screen.queryByRole('button', { name: 'Download archived JSON' })
      ).not.toBeInTheDocument()
      expect(api.get).not.toHaveBeenCalled()
    }
  )

  test('requires administrator access before exposing a download action', () => {
    useAuthStore.getState().auth.setUser({ id: 2, username: 'user', role: 1 })
    render(
      <RequestSnapshotDownloadPage url='/api/task/task_1/request-snapshots/original' />
    )
    expect(screen.getByText('Administrator access required')).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Download archived JSON' })
    ).not.toBeInTheDocument()
    expect(api.get).not.toHaveBeenCalled()
  })
})
