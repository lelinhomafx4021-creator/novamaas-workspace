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
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import { verifySupplierVideo } from '../api'
import { VideoReportControls } from '../components/video-report-controls'
import { DEFAULT_VIDEO_REPORT_CHECKS } from '../constants'
import type { VideoTestRecord, VideoExchange } from '../types'

vi.mock('../api', () => ({ verifySupplierVideo: vi.fn() }))
const record: VideoTestRecord = {
  id: 'one',
  baseUrl: 'https://supplier.example',
  model: 'model',
  startedAt: 1,
  outcome: 'Succeeded',
  materials: [],
  metrics: {
    task_id: 'video-one',
    status: 'succeeded',
    elapsed_ms: 1,
    video_url: 'https://cdn.example/original.mp4',
  },
}
const exchange = (response: string): VideoExchange => ({
  method: 'POST',
  url: 'https://ark.cn-beijing.volcengineapi.com/',
  headers: 'Authorization: [REDACTED]',
  request: '{}',
  response,
  status: 200,
  request_id: 'request',
  elapsed_ms: 1,
})
function Editor() {
  const [records, setRecords] = useState([record])
  const [checks, setChecks] = useState(DEFAULT_VIDEO_REPORT_CHECKS)
  return (
    <VideoReportControls
      records={records}
      busy={false}
      checks={checks}
      onChecksChange={setChecks}
      onUpdate={(id, value) =>
        setRecords((current) =>
          current.map((item) =>
            item.id === id ? { ...item, verification: value } : item
          )
        )
      }
      onClear={() => setRecords([])}
      onBusyChange={() => {}}
      stopVersion={0}
    />
  )
}
async function enterCredentials(user: ReturnType<typeof userEvent.setup>) {
  await user.type(
    screen.getByRole('textbox', { name: 'Official Access Key ID' }),
    'official-key'
  )
  await user.type(
    screen.getByLabelText('Official Secret Access Key'),
    'official-secret'
  )
}
beforeEach(() => {
  vi.mocked(verifySupplierVideo).mockReset()
})

test('four capabilities start unchecked and can be independently toggled with mouse and keyboard', async () => {
  const user = userEvent.setup()
  render(<Editor />)
  const options = [
    'Supplier has implemented the v3 API',
    'Supplier has implemented a material library',
    'Returns the upstream Task ID',
    'Returns the upstream Volcengine URL',
  ].map((name) => screen.getByRole('checkbox', { name }))
  for (const option of options) expect(option).not.toBeChecked()
  for (const option of options) {
    await user.click(option)
    expect(option).toBeChecked()
    for (const other of options.filter((item) => item !== option)) {
      expect(other).not.toBeChecked()
    }
    option.focus()
    await user.keyboard(' ')
    expect(option).not.toBeChecked()
  }
})

test('credentials do not submit while typing; verification uses the original URL and resumes by QueryID after a query error', async () => {
  vi.mocked(verifySupplierVideo)
    .mockResolvedValueOnce(exchange('{"Result":{"QueryID":"accepted-query"}}'))
    .mockRejectedValueOnce(new Error('result connection interrupted'))
    .mockResolvedValueOnce(
      exchange('{"Result":{"Status":"Success","IsOfficial":false}}')
    )
  const user = userEvent.setup()
  render(<Editor />)
  await enterCredentials(user)
  expect(verifySupplierVideo).not.toHaveBeenCalled()
  const verify = screen.getByRole('button', {
    name: 'Verify or continue querying',
  })
  await user.click(verify)
  await screen.findByText('result connection interrupted')
  expect(verify).toBeEnabled()
  expect(vi.mocked(verifySupplierVideo).mock.calls[0][0].video_url).toBe(
    'https://cdn.example/original.mp4'
  )
  await user.click(verify)
  await screen.findByText('No features detected')
  expect(
    vi
      .mocked(verifySupplierVideo)
      .mock.calls.map(([request]) => [request.action, request.query_id])
  ).toEqual([
    ['create', undefined],
    ['query', 'accepted-query'],
    ['query', 'accepted-query'],
  ])
})

test('an ambiguous creation is not submitted again', async () => {
  vi.mocked(verifySupplierVideo).mockRejectedValue(
    new Error('network interrupted')
  )
  const user = userEvent.setup()
  render(<Editor />)
  await enterCredentials(user)
  await user.click(
    screen.getByRole('button', { name: 'Verify or continue querying' })
  )
  await screen.findByText('network interrupted')
  expect(
    screen.getByRole('button', { name: 'Verify or continue querying' })
  ).toBeDisabled()
  expect(verifySupplierVideo).toHaveBeenCalledTimes(1)
})
