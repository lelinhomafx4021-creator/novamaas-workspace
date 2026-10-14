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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useVideoStudioStore } from '@/stores/video-studio-store'

import { listReferences, saveReference } from '../api'
import { ReferencePanel } from '../components/reference-panel'
import type { StudioReference } from '../types'

vi.mock('../api', () => ({
  listReferences: vi.fn(),
  saveReference: vi.fn(),
  uploadReference: vi.fn(),
}))
vi.mock('@/components/ui/scroll-area', () => ({
  ScrollArea: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}))
vi.mock('@/features/assets/api', () => ({ listMediaAssets: vi.fn() }))
vi.mock('../components/reference-thumbnail', () => ({
  ReferenceThumbnail: ({ label }: { label: string }) => <span>{label}</span>,
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))

const temporary: StudioReference = {
  id: 'ref_1',
  file_name: 'photo.png',
  content_type: 'image/png',
  size: 80,
  sha256: 'sha256',
  saved: false,
  expires_at: 2_000_000_000,
  created_at: 1_900_000_000,
}

beforeEach(() => {
  vi.clearAllMocks()
  useVideoStudioStore.setState({ selectedNodeId: null })
})

describe('reference library', () => {
  test('restores temporary uploads from the server and moves a saved file into the library', async () => {
    const saved = { ...temporary, saved: true, expires_at: 0 }
    vi.mocked(listReferences)
      .mockResolvedValueOnce({
        success: true,
        data: { items: [temporary], total: 1, page: 1, page_size: 100 },
      })
      .mockResolvedValue({
        success: true,
        data: { items: [saved], total: 1, page: 1, page_size: 100 },
      })
    vi.mocked(saveReference).mockResolvedValue({ success: true, data: saved })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })

    render(
      <QueryClientProvider client={client}>
        <ReferencePanel
          supportsAssets={false}
          defaultRole='reference_image'
          maxReferences={9}
        />
      </QueryClientProvider>
    )

    expect(await screen.findByText('photo.png')).toBeVisible()
    expect(listReferences).toHaveBeenCalledWith(1, 100, true)
    fireEvent.click(screen.getByRole('button', { name: 'Save to library' }))
    await waitFor(() => expect(listReferences).toHaveBeenCalledTimes(2))
    fireEvent.click(screen.getByRole('tab', { name: 'Library' }))
    expect(await screen.findByText('photo.png')).toBeVisible()
    expect(screen.getByText('Saved')).toBeVisible()
  })
})
