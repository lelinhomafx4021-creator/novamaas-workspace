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
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { uploadVideoMaterial } from '../api'
import { VideoMaterialPanel } from '../components/video-material-panel'
import { DEFAULT_VIDEO_FORM } from '../constants'
import {
  buildVideoRequestPayload,
  parseVideoPayloadToForm,
} from '../video-json'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('../api', () => ({
  queryVideoMaterials: vi.fn(),
  previewVideoMaterial: vi.fn(),
  uploadVideoMaterial: vi.fn(),
}))

function MaterialEditor() {
  const [video, setVideo] = useState({
    ...DEFAULT_VIDEO_FORM,
    prompt: 'animate',
  })
  return (
    <>
      <VideoMaterialPanel
        video={video}
        busy={false}
        baseUrl='https://gateway.example'
        apiKey='test-key'
        onChange={setVideo}
      />
      <output data-testid='payload'>
        {JSON.stringify(buildVideoRequestPayload('model', video))}
      </output>
    </>
  )
}

test('adding, reordering and removing images updates every material without losing the other images', async () => {
  const user = userEvent.setup()
  render(<MaterialEditor />)
  const url = screen.getByRole('textbox', { name: 'Material URL or asset ID' })
  await user.type(url, 'https://cdn.example/a.png')
  await user.click(screen.getByRole('button', { name: 'Add material' }))
  await user.type(url, 'https://cdn.example/b.png')
  await user.click(screen.getByRole('button', { name: 'Add material' }))
  expect(
    screen.getAllByRole('button', { name: 'Move material up' })[0]
  ).toBeDisabled()
  await user.click(
    screen.getAllByRole('button', { name: 'Move material up' })[1]
  )
  let payload = JSON.parse(screen.getByTestId('payload').textContent || '{}')
  expect(
    payload.content
      .filter((item: { type: string }) => item.type === 'image_url')
      .map((item: { image_url: { url: string } }) => item.image_url.url)
  ).toEqual(['https://cdn.example/b.png', 'https://cdn.example/a.png'])
  await user.click(
    screen.getAllByRole('button', { name: 'Remove material {{index}}' })[0]
  )
  payload = JSON.parse(screen.getByTestId('payload').textContent || '{}')
  expect(
    payload.content.filter(
      (item: { type: string }) => item.type === 'image_url'
    )
  ).toHaveLength(1)
  expect(payload.content[1].image_url.url).toBe('https://cdn.example/a.png')
})

test('native JSON round-trips multiple images and unknown interleaved content without stale form fields', () => {
  const native = {
    model: 'doubao-seedance-2-5',
    content: [
      { type: 'text', text: 'new prompt', custom: 'kept' },
      {
        type: 'image_url',
        role: 'reference_image',
        image_url: { url: 'https://cdn.example/a.png', custom: 0 },
      },
      {
        type: 'audio_url',
        audio_url: { url: 'https://cdn.example/audio.mp3' },
        role: 'reference_audio',
      },
      {
        type: 'image_url',
        role: 'reference_image',
        image_url: { url: 'data:image/png;base64,aGVsbG8=' },
        custom: false,
      },
    ],
    seed: 0,
    watermark: false,
    duration: 0,
  }
  const parsed = parseVideoPayloadToForm(JSON.stringify(native))
  expect(parsed.success).toBe(true)
  const form = {
    ...DEFAULT_VIDEO_FORM,
    hasLastFrame: true,
    lastFrameUrl: 'https://stale.example',
    ...parsed.videoPatch,
  }
  expect(buildVideoRequestPayload(parsed.model || '', form)).toEqual(native)
})

test('library upload needs no group selection and uses the returned material ID', async () => {
  vi.mocked(uploadVideoMaterial).mockResolvedValue({
    asset: {
      id: 'asset-default',
      name: 'photo.png',
      group_id: 'default-group',
      owner_user_id: 1,
      owner_name: 'tester',
      type: 'image',
      content_type: 'image/png',
      size: 7,
      sha256: 'fixture',
      status: 'processing',
      created_at: 1,
      updated_at: 1,
    },
    preparation_run_id: 'prepared-default',
  })
  const user = userEvent.setup()
  render(<MaterialEditor />)
  await user.click(
    screen.getByRole('checkbox', { name: 'Platform integration test' })
  )
  await user.click(
    screen.getByRole('checkbox', {
      name: 'Upload new images to material library',
    })
  )
  expect(
    screen.queryByRole('textbox', { name: 'Material group ID' })
  ).toBeNull()
  await user.upload(
    screen.getByLabelText('Select images'),
    new File(['fixture'], 'photo.png', { type: 'image/png' })
  )
  await waitFor(() => expect(uploadVideoMaterial).toHaveBeenCalledTimes(1))
  const form = vi.mocked(uploadVideoMaterial).mock.calls[0][0]
  expect(form.get('group_id')).toBeNull()
  expect(form.get('api_key')).toBe('test-key')
  await waitFor(() =>
    expect(screen.getByTestId('payload').textContent).toContain(
      'asset://asset-default'
    )
  )
})

test.each(['base64', 'library'] as const)(
  'resetting the file input preserves all selected files during asynchronous %s uploads',
  async (mode) => {
    vi.mocked(uploadVideoMaterial).mockReset()
    vi.mocked(uploadVideoMaterial).mockImplementation(async (form) => {
      const file = form.get('file') as File
      return {
        asset: {
          id: file.name,
          name: file.name,
          group_id: 'default-group',
          owner_user_id: 1,
          owner_name: 'tester',
          type: 'image',
          content_type: 'image/png',
          size: file.size,
          sha256: 'fixture',
          status: 'ready',
          created_at: 1,
          updated_at: 1,
        },
        preparation_run_id: `prepared-${file.name}`,
      }
    })
    const user = userEvent.setup()
    render(<MaterialEditor />)
    if (mode === 'library') {
      await user.click(
        screen.getByRole('checkbox', { name: 'Platform integration test' })
      )
      await user.click(
        screen.getByRole('checkbox', {
          name: 'Upload new images to material library',
        })
      )
    }
    const input = screen.getByLabelText<HTMLInputElement>('Select images')
    const selected = [
      new File(['first'], 'first.png', { type: 'image/png' }),
      new File(['second'], 'second.png', { type: 'image/png' }),
    ]
    // Browsers expose a live FileList: resetting the input clears an iterator that is still in use.
    const liveFiles = {
      get length() {
        return selected.length
      },
      item(index: number) {
        return selected[index] || null
      },
      [Symbol.iterator]() {
        return selected[Symbol.iterator]()
      },
    } as FileList
    Object.defineProperty(input, 'files', {
      configurable: true,
      get: () => liveFiles,
    })
    Object.defineProperty(input, 'value', {
      configurable: true,
      get: () => '',
      set: (value) => {
        if (value === '') selected.length = 0
      },
    })
    fireEvent.change(input)
    const expected =
      mode === 'library'
        ? ['asset://first.png', 'asset://second.png']
        : ['data:image/png;base64,Zmlyc3Q=', 'data:image/png;base64,c2Vjb25k']
    await waitFor(() => {
      const payload = JSON.parse(
        screen.getByTestId('payload').textContent || '{}'
      )
      expect(
        payload.content
          .filter((item: { type: string }) => item.type === 'image_url')
          .map((item: { image_url: { url: string } }) => item.image_url.url)
      ).toEqual(expected)
    })
    if (mode === 'library') expect(uploadVideoMaterial).toHaveBeenCalledTimes(2)
  }
)
