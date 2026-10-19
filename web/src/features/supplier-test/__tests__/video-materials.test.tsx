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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { VideoMaterials } from '../components/video-materials'
import { DEFAULT_VIDEO_FORM } from '../constants'
import type { VideoForm } from '../types'
import { buildVideoRequestPayload } from '../video-json'

vi.mock('@/lib/api', () => ({ api: { request: vi.fn() } }))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
const usePlatform = vi.fn()
function Editor() {
  const [video, setVideo] = useState<VideoForm>(DEFAULT_VIDEO_FORM)
  const [busy, setBusy] = useState(false)
  return (
    <>
      <VideoMaterials
        video={video}
        onChange={setVideo}
        busy={busy}
        onBusyChange={setBusy}
        onUsePlatform={usePlatform}
        stopVersion={0}
      />
      <output data-testid='request'>
        {JSON.stringify(buildVideoRequestPayload('model', video))}
      </output>
      <output data-testid='evidence'>
        {JSON.stringify(video.materialEvidence || [])}
      </output>
    </>
  )
}
beforeEach(() => {
  vi.mocked(api.request).mockReset()
  usePlatform.mockReset()
})
afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

test('URL, Base64 files and asset IDs preserve material order and allow reordering', async () => {
  const user = userEvent.setup()
  render(<Editor />)
  const input = screen.getByRole('textbox', {
    name: 'Material URL or asset ID',
  })
  await user.type(input, 'https://cdn.example/first.png')
  await user.click(screen.getByRole('button', { name: 'Add material' }))
  await user.upload(screen.getByLabelText('Select images'), [
    new File(['one'], 'one.png', { type: 'image/png' }),
    new File(['two'], 'two.png', { type: 'image/png' }),
  ])
  await waitFor(() =>
    expect(
      JSON.parse(screen.getByTestId('request').textContent || '{}').content
    ).toHaveLength(3)
  )
  await user.type(input, 'asset://asset-example')
  await user.click(screen.getByRole('button', { name: 'Add material' }))
  expect(usePlatform).toHaveBeenCalledTimes(1)
  let content = JSON.parse(
    screen.getByTestId('request').textContent || '{}'
  ).content
  expect(
    content.map((item: { image_url: { url: string } }) => item.image_url.url)
  ).toEqual([
    'https://cdn.example/first.png',
    'data:image/png;base64,b25l',
    'data:image/png;base64,dHdv',
    'asset://asset-example',
  ])
  expect(
    screen.getAllByRole('button', { name: 'Move material up' })[0]
  ).toBeDisabled()
  await user.click(
    screen.getAllByRole('button', { name: 'Move material up' })[3]
  )
  content = JSON.parse(
    screen.getByTestId('request').textContent || '{}'
  ).content
  expect(content[2].image_url.url).toBe('asset://asset-example')
  expect(api.request).not.toHaveBeenCalled()
})

test('library upload reuses the default group and keeps one readiness response for repeated queries', async () => {
  vi.stubGlobal('crypto', {
    ...crypto,
    randomUUID: () => 'image-ui-id',
    subtle: { digest: async () => new Uint8Array(32).buffer },
  })
  const user = userEvent.setup()
  let queries = 0
  vi.mocked(api.request).mockImplementation(async (config) => {
    let data: unknown = [{ id: 'default-group', name: 'default' }]
    if (config.method === 'POST') {
      data = {
        id: 'asset-one',
        name: 'one',
        type: 'image',
        status: 'processing',
      }
    } else if (config.url?.includes('/assets?')) {
      data = {
        items: [
          {
            id: 'asset-one',
            name: 'one',
            type: 'image',
            status: ++queries < 2 ? 'processing' : 'ready',
          },
        ],
      }
    }
    return {
      data: JSON.stringify({ success: true, data, unknown: false, zero: 0 }),
      status: 200,
      headers: { 'x-request-id': 'material-request' },
      config: { headers: { Authorization: 'private' }, ...config },
    } as Awaited<ReturnType<typeof api.request>>
  })
  render(<Editor />)
  await user.click(
    screen.getByRole('checkbox', {
      name: 'Upload new images to material library',
    })
  )
  const file = new File(['one'], 'one.png', { type: 'image/png' })
  Object.defineProperty(file, 'arrayBuffer', {
    value: async () => new TextEncoder().encode('one').buffer,
  })
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  fireEvent.change(screen.getByLabelText('Select images'), {
    target: { files: [file] },
  })
  await act(async () => {
    await vi.advanceTimersByTimeAsync(2000)
  })
  await act(async () => {
    await vi.advanceTimersByTimeAsync(2000)
  })
  expect(screen.getByTestId('request')).toHaveTextContent('asset://asset-one')
  expect(
    vi.mocked(api.request).mock.calls.map(([config]) => config.method)
  ).toEqual(['GET', 'POST', 'GET', 'GET'])
  const evidence = JSON.parse(
    screen.getByTestId('evidence').textContent || '[]'
  )
  expect(evidence).toHaveLength(3)
  expect(evidence[2].response).toContain('ready')
  expect(evidence[2].response).toContain('"zero":0')
  expect(evidence[1].request).toContain('default-group')
  expect(evidence[1].request).toContain('sha256')
  expect(evidence[1].headers).not.toContain('private')
  vi.unstubAllGlobals()
})
