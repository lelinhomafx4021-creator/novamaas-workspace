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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import { Tabs } from '@/components/ui/tabs'

import { querySupplierVideoTask } from '../api'
import { VideoPanel } from '../components/video-panel'
import { DEFAULT_VIDEO_FORM } from '../constants'
import type { VideoForm } from '../types'

vi.mock('../api', () => ({ querySupplierVideoTask: vi.fn() }))

test.each(['upstream error', 'network error'])(
  'manual query forwards %s to the report with the requested task and supplier',
  async (failure) => {
    const onResult = vi.fn()
    const response = {
      success: false,
      message: 'Query unavailable',
      raw_response: '{"error":"unavailable","retry":false}',
    }
    if (failure === 'network error') {
      vi.mocked(querySupplierVideoTask).mockRejectedValueOnce(
        new Error(response.message)
      )
    } else {
      vi.mocked(querySupplierVideoTask).mockResolvedValueOnce(response)
    }
    function Editor() {
      const [video, setVideo] = useState<VideoForm>({
        ...DEFAULT_VIDEO_FORM,
        taskId: 'task-one',
      })
      return (
        <Tabs defaultValue='video'>
          <VideoPanel
            video={video}
            onVideoChange={setVideo}
            busy={false}
            baseUrl='https://supplier.example'
            videoChecks={[]}
            videoMetrics={null}
            onUsePlatform={() => {}}
            onMaterialBusyChange={() => {}}
            stopVersion={0}
            onManualQueryResult={onResult}
          />
        </Tabs>
      )
    }
    render(<Editor />)
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Query Status Now' }))
    await waitFor(() =>
      expect(onResult).toHaveBeenCalledWith(
        expect.objectContaining({
          success: false,
          message: response.message,
          base_url: 'https://supplier.example',
          task_id: 'task-one',
        })
      )
    )
    if (failure === 'upstream error') {
      expect(onResult.mock.calls[0][0].raw_response).toBe(response.raw_response)
    } else {
      expect(onResult.mock.calls[0][0].exchange).toBeUndefined()
    }
  }
)
