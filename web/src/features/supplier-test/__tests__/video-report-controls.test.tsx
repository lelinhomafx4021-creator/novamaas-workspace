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
import { beforeEach, expect, test, vi } from 'vitest'

import { verifySupplierVideo } from '../api'
import { VideoReportControls } from '../components/video-report-controls'
import type { VideoTestRecord, VideoExchange } from '../types'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
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
  const [records, setRecords] = useState([record]),
    [checks, setChecks] = useState({ v3: false, materialLibrary: false })
  return (
    <>
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
      <output>{JSON.stringify(checks)}</output>
    </>
  )
}
beforeEach(() => {
  vi.mocked(verifySupplierVideo).mockReset()
})

test('capabilities remain unchecked until the user checks each option independently', async () => {
  const user = userEvent.setup()
  render(<Editor />)
  const v3 = screen.getByRole('checkbox', {
      name: 'Supplier has implemented the v3 API',
    }),
    library = screen.getByRole('checkbox', {
      name: 'Supplier has implemented a material library',
    })
  expect(v3).not.toBeChecked()
  expect(library).not.toBeChecked()
  await user.click(v3)
  expect(v3).toBeChecked()
  expect(library).not.toBeChecked()
  library.focus()
  await user.keyboard(' ')
  expect(library).toBeChecked()
  await user.click(v3)
  expect(v3).not.toBeChecked()
  expect(library).toBeChecked()
})

test('entering official credentials for an existing result does not create a query while typing, and explicit verification uses the original URL', async () => {
  vi.mocked(verifySupplierVideo)
    .mockResolvedValueOnce(exchange('{"Result":{"QueryID":"accepted-query"}}'))
    .mockResolvedValueOnce(
      exchange('{"Result":{"Status":"Success","IsOfficial":null}}')
    )
  const user = userEvent.setup()
  render(<Editor />)
  await user.type(
    screen.getByRole('textbox', { name: 'Official Access Key ID' }),
    'official-key'
  )
  await user.type(
    screen.getByLabelText('Official Secret Access Key'),
    'official-secret'
  )
  expect(verifySupplierVideo).not.toHaveBeenCalled()
  await user.click(
    screen.getByRole('button', { name: 'Verify or continue querying' })
  )
  await waitFor(() => expect(verifySupplierVideo).toHaveBeenCalledTimes(2))
  expect(
    vi.mocked(verifySupplierVideo).mock.calls.map(([request]) => request.action)
  ).toEqual(['create', 'query'])
  expect(vi.mocked(verifySupplierVideo).mock.calls[0][0].video_url).toBe(
    'https://cdn.example/original.mp4'
  )
  expect(vi.mocked(verifySupplierVideo).mock.calls[1][0].query_id).toBe(
    'accepted-query'
  )
  await waitFor(() =>
    expect(screen.getAllByText('Cannot confirm')).toHaveLength(2)
  )
})

test('an ambiguous creation is not submitted again', async () => {
  vi.mocked(verifySupplierVideo).mockRejectedValue(
    new Error('network interrupted')
  )
  const user = userEvent.setup()
  render(<Editor />)
  await user.click(
    screen.getByRole('checkbox', {
      name: 'Automatically verify completed videos',
    })
  )
  await user.type(
    screen.getByRole('textbox', { name: 'Official Access Key ID' }),
    'official-key'
  )
  await user.type(
    screen.getByLabelText('Official Secret Access Key'),
    'official-secret'
  )
  await user.click(
    screen.getByRole('button', { name: 'Verify or continue querying' })
  )
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Verify or continue querying' })
    ).toBeDisabled()
  )
  expect(verifySupplierVideo).toHaveBeenCalledTimes(1)
})

test('a failed result query can be resumed by QueryID without creating another query', async () => {
  vi.mocked(verifySupplierVideo)
    .mockResolvedValueOnce(exchange('{"Result":{"QueryID":"accepted-query"}}'))
    .mockRejectedValueOnce(new Error('result connection interrupted'))
    .mockResolvedValueOnce(
      exchange('{"Result":{"Status":"Success","IsOfficial":false}}')
    )
  const user = userEvent.setup()
  render(<Editor />)
  await user.type(
    screen.getByRole('textbox', { name: 'Official Access Key ID' }),
    'official-key'
  )
  await user.type(
    screen.getByLabelText('Official Secret Access Key'),
    'official-secret'
  )
  const verify = screen.getByRole('button', {
    name: 'Verify or continue querying',
  })
  await user.click(verify)
  await screen.findByText('result connection interrupted')
  expect(verify).toBeEnabled()
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
