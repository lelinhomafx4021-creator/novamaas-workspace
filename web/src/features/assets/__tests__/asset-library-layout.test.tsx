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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { getMediaAssetPreview, listAssetGroups, listMediaAssets } from '../api'
import { AssetLibrary } from '../asset-library'
import { AssetUploadDialog } from '../components/asset-upload-dialog'

vi.mock('../api', () => ({
  createAssetGroup: vi.fn(),
  deleteAssetGroup: vi.fn(),
  deleteMediaAsset: vi.fn(),
  getMediaAssetPreview: vi.fn(),
  listAssetGroups: vi.fn(),
  listMediaAssets: vi.fn(),
  uploadMediaAsset: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }))

let queryClient: QueryClient

function renderLibrary() {
  queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <AssetLibrary />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  Object.defineProperty(URL, 'createObjectURL', {
    configurable: true,
    value: vi.fn(() => 'blob:test-preview'),
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    configurable: true,
    value: vi.fn(),
  })
  vi.mocked(listAssetGroups).mockResolvedValue({
    success: true,
    data: [
      {
        id: 'group-1',
        owner_user_id: 1,
        name: 'Campaign assets',
        description: '',
        status: 'ready',
        created_at: 1,
        updated_at: 1,
      },
    ],
  })
  vi.mocked(listMediaAssets).mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 'asset-20260922120000-abcde',
          group_id: 'group-1',
          owner_user_id: 1,
          owner_name: 'asset-owner',
          name: 'Hero image',
          type: 'image',
          content_type: 'image/png',
          size: 2048,
          sha256: 'checksum',
          status: 'ready',
          created_at: 1,
          updated_at: 1,
        },
      ],
      total: 1,
      page: 1,
      page_size: 40,
    },
  })
  vi.mocked(getMediaAssetPreview).mockResolvedValue({
    success: true,
    data: { url: 'https://example.com/hero.png', expires_at: 100 },
  })
})

afterEach(() => {
  queryClient?.clear()
  vi.unstubAllGlobals()
})

describe('asset library business layout', () => {
  test('uses a compact asset grid without exposing synchronization controls', async () => {
    renderLibrary()

    expect(await screen.findByText('Hero image')).toBeVisible()
    expect(screen.getByTestId('asset-grid')).toHaveAttribute(
      'data-density',
      'compact'
    )
    expect(
      screen.queryByRole('button', { name: /sync/i })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByText('No asset channels are enabled')
    ).not.toBeInTheDocument()
    expect(screen.getByText(/asset-owner/)).toBeVisible()
    expect(screen.getByText(/Uploaded at/)).toBeVisible()
  })

  test('keeps the grid bounded with server pagination', async () => {
    vi.mocked(listMediaAssets).mockResolvedValueOnce({
      success: true,
      data: {
        items: [],
        total: 41,
        page: 1,
        page_size: 40,
      },
    })
    renderLibrary()

    const nextButton = await screen.findByRole('button', { name: 'Next' })
    fireEvent.click(nextButton)

    await waitFor(() =>
      expect(listMediaAssets).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, pageSize: 40 })
      )
    )
  })

  test('requests a preview only when its card approaches the viewport', async () => {
    let intersectionCallback: IntersectionObserverCallback | undefined
    const disconnect = vi.fn()
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(callback: IntersectionObserverCallback) {
          intersectionCallback = callback
        }

        observe() {}
        disconnect() {
          disconnect()
        }
      }
    )
    renderLibrary()

    expect(await screen.findByText('Hero image')).toBeVisible()
    expect(getMediaAssetPreview).not.toHaveBeenCalled()

    act(() => {
      intersectionCallback?.(
        [{ isIntersecting: true } as IntersectionObserverEntry],
        {} as IntersectionObserver
      )
    })

    await waitFor(() => expect(getMediaAssetPreview).toHaveBeenCalledOnce())
    expect(disconnect).toHaveBeenCalled()
  })

  test('infers the asset type from the selected file before upload', async () => {
    const onSubmit = vi.fn()
    render(
      <AssetUploadDialog
        open
        onOpenChange={vi.fn()}
        groups={[
          {
            id: 'group-1',
            owner_user_id: 1,
            name: 'Campaign assets',
            description: '',
            status: 'ready',
            created_at: 1,
            updated_at: 1,
          },
        ]}
        selectedGroup='group-1'
        onSubmit={onSubmit}
        pending={false}
        progress={0}
      />
    )

    const fileInput = await waitFor(() => {
      const input = document.querySelector('input[type="file"]')
      expect(input).toBeInstanceOf(HTMLInputElement)
      return input
    })
    expect(fileInput).toBeInstanceOf(HTMLInputElement)
    const file = new File(['png'], 'sample.png', { type: 'image/png' })
    fireEvent.change(fileInput as HTMLInputElement, {
      target: { files: [file] },
    })
    expect(await screen.findByText('sample.png')).toBeVisible()

    fireEvent.click(screen.getByRole('button', { name: 'Upload' }))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce())
    const formData = onSubmit.mock.calls[0][0] as FormData
    expect(formData.get('type')).toBe('image')
    expect(formData.get('file')).toBe(file)
  })
})
