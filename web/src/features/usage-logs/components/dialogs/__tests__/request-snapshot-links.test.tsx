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

import { RequestSnapshotsViewer } from '../request-snapshots-viewer'

const writeText = vi.fn()
const previousClipboard = Object.getOwnPropertyDescriptor(
  navigator,
  'clipboard'
)
const previousAdapter = api.defaults.adapter
const requests: Array<{
  url?: string
  authorization: unknown
  responseType?: string
}> = []
const downloads: Array<{ href: string; filename: string }> = []

beforeEach(() => {
  writeText.mockReset().mockResolvedValue(undefined)
  Object.defineProperty(navigator, 'clipboard', {
    configurable: true,
    value: { writeText },
  })
  requests.length = 0
  downloads.length = 0
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 1, username: 'admin', role: 10 },
      accessToken: 'admin-snapshot-token',
    },
  }))
  api.defaults.adapter = async (config) => {
    requests.push({
      url: config.url,
      authorization: config.headers.Authorization,
      responseType: config.responseType,
    })
    return {
      data: new Blob(['{"prompt":"archived"}'], { type: 'application/json' }),
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
  vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:snapshot')
  vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
    function (this: HTMLAnchorElement) {
      downloads.push({ href: this.href, filename: this.download })
    }
  )
})
afterEach(() => {
  if (previousClipboard) {
    Object.defineProperty(navigator, 'clipboard', previousClipboard)
  } else {
    Reflect.deleteProperty(navigator, 'clipboard')
  }
  api.defaults.adapter = previousAdapter
  useAuthStore.getState().auth.reset('idle')
  vi.restoreAllMocks()
})

describe('permanent request snapshot links', () => {
  test('downloads original and upstream JSON through their stable authenticated endpoints', async () => {
    render(
      <RequestSnapshotsViewer
        original={{ prompt: 'client request' }}
        upstream={{ model: 'provider-model' }}
        originalUrl='/api/task/task_video/request-snapshots/original'
        upstreamUrl='/api/task/task_video/request-snapshots/upstream'
      />
    )
    const originalDownload = await screen.findByRole('button', {
      name: 'Download archived JSON',
    })
    fireEvent.click(originalDownload)
    await waitFor(() =>
      expect(requests).toEqual([
        {
          url: '/api/task/task_video/request-snapshots/original',
          authorization: 'Bearer admin-snapshot-token',
          responseType: 'blob',
        },
      ])
    )
    await waitFor(() =>
      expect(downloads).toEqual([
        { href: 'blob:snapshot', filename: 'original-request.json' },
      ])
    )
    fireEvent.click(screen.getByRole('button', { name: 'Copy permanent link' }))
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(
        `${window.location.origin}/request-snapshots?url=%2Fapi%2Ftask%2Ftask_video%2Frequest-snapshots%2Foriginal`
      )
    )
    fireEvent.click(
      screen.getByRole('tab', { name: 'Actual Upstream Request' })
    )
    const upstreamDownload = await screen.findByRole('button', {
      name: 'Download archived JSON',
    })
    fireEvent.click(upstreamDownload)
    await waitFor(() =>
      expect(requests[1]).toEqual({
        url: '/api/task/task_video/request-snapshots/upstream',
        authorization: 'Bearer admin-snapshot-token',
        responseType: 'blob',
      })
    )
    await waitFor(() =>
      expect(downloads[1]).toEqual({
        href: 'blob:snapshot',
        filename: 'upstream-request.json',
      })
    )
    expect(
      screen.getByText(
        'The permanent link opens a download page after administrator sign-in.'
      )
    ).toBeVisible()
  })

  test('retains legacy body-only display when no permanent URLs are available', async () => {
    render(<RequestSnapshotsViewer original={{ prompt: 'legacy request' }} />)
    fireEvent.click(await screen.findByRole('button', { name: 'Copy code' }))
    await waitFor(() =>
      expect(writeText).toHaveBeenCalledWith(
        '{\n  "prompt": "legacy request"\n}'
      )
    )
    expect(
      screen.queryByRole('button', { name: 'Download archived JSON' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Copy permanent link' })
    ).not.toBeInTheDocument()
  })

  test('omits oversized JSON previews while preserving complete content copying and permanent downloads', async () => {
    const original = `{"image":"data:image/png;base64,${'A'.repeat(4 * 1024 * 1024)}"}`
    render(
      <RequestSnapshotsViewer
        original={original}
        originalUrl='/api/task/task_large/request-snapshots/original'
      />
    )
    expect(await screen.findByText('Large request snapshot')).toBeVisible()
    expect(screen.queryByText('original-request.json')).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Download archived JSON' })
    ).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Copy complete JSON' }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(original))
  })

  test('shows a download error and permits retry through the authenticated client', async () => {
    api.defaults.adapter = async () => {
      throw new Error('Snapshot archive unavailable')
    }
    render(
      <RequestSnapshotsViewer
        original={{ prompt: 'retry' }}
        originalUrl='/api/task/task_video/request-snapshots/original'
      />
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Download archived JSON' })
    )
    expect(
      await screen.findByText('Snapshot archive unavailable')
    ).toBeVisible()
    expect(
      screen.getByRole('button', { name: 'Download archived JSON' })
    ).toBeEnabled()
    expect(downloads).toEqual([])
  })

  test('disables the download action while its authenticated request is pending', async () => {
    let completeDownload: () => void = () => undefined
    api.defaults.adapter = (config) =>
      new Promise((resolve) => {
        completeDownload = () =>
          resolve({
            data: new Blob(['{}']),
            status: 200,
            statusText: 'OK',
            headers: {},
            config,
          })
      })
    render(
      <RequestSnapshotsViewer
        original={{ prompt: 'pending download' }}
        originalUrl='/api/task/task_video/request-snapshots/original'
      />
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Download archived JSON' })
    )
    expect(
      await screen.findByRole('button', { name: 'Downloading...' })
    ).toBeDisabled()
    completeDownload()
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Download archived JSON' })
      ).toBeEnabled()
    )
    expect(downloads).toHaveLength(1)
  })
})
