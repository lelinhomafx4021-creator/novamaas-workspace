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
import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { VideoReportPanel } from '../components/video-report-panel'
import type { VideoRunReport } from '../types'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('../api', () => ({
  getVerificationSettings: vi.fn().mockResolvedValue({ configured: true }),
  downloadVideoEvidence: vi.fn(),
  saveVerificationSettings: vi.fn(),
}))

test('a single report shows both generation and verification without a report switch', async () => {
  const run: VideoRunReport = {
    id: 'run-one',
    model: 'model',
    task_id: 'task-one',
    query_id: 'query-one',
    base_url: 'https://gateway.example',
    status: 'completed',
    started_at: 1,
    assets: [],
    video_url: 'https://cdn.example/video.mp4',
    video_polling: { count: 6, elapsed_ms: 200 },
    verification_polling: { count: 2, elapsed_ms: 80 },
    stages: [
      {
        id: 'video_submit',
        purpose: '',
        status: 'pass',
        request_body: '{"prompt":"generation-body"}',
        response_body: '{"id":"task-one"}',
        method: 'POST',
        elapsed_ms: 1,
        conclusion: 'Task accepted',
      },
      {
        id: 'verification_submit',
        purpose: '',
        status: 'pass',
        request_body: '{"ContentURL":"verification-body"}',
        response_body: '{"QueryID":"query-one"}',
        method: 'POST',
        elapsed_ms: 1,
        conclusion: 'Verification accepted',
      },
    ],
  }
  const props = { run, busy: false, verify: true, onVerifyChange: vi.fn() }
  const view = render(<VideoReportPanel {...props} />)
  await screen.findByRole('button', {
    name: 'Replace verification credentials',
  })
  expect(screen.getByText(/generation-body/)).toBeInTheDocument()
  expect(screen.getByText(/verification-body/)).toBeInTheDocument()
  expect(screen.getAllByText(/Polling summary/)).toHaveLength(1)
  expect(
    screen.queryByRole('textbox', { name: 'Run ID' })
  ).not.toBeInTheDocument()
  view.rerender(<VideoReportPanel {...props} busy />)
  expect(screen.getByText(/generation-body/)).toBeInTheDocument()
  expect(screen.getByText(/verification-body/)).toBeInTheDocument()
  expect(screen.getAllByText(/Polling summary/)).toHaveLength(1)
  expect(
    screen.getByRole('button', { name: 'Continue official verification' })
  ).toBeDisabled()
})
