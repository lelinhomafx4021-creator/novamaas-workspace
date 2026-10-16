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
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SystemInfo } from '../../index'
import type { ResourceMonitorReport } from '../../types'
import { SystemResourceMonitor } from '../system-resource-monitor'

const resources = {
  timestamp: 1_792_035_600,
  process: {
    available: true,
    cpu_available: true,
    memory_available: true,
    error: '',
    cpu_percent: 12.5,
    rss_bytes: 134_217_728,
  },
  host: {
    available: true,
    cpu_available: true,
    memory_available: true,
    error: '',
    cpu_percent: 34,
    memory_total_bytes: 8_589_934_592,
    memory_used_bytes: 4_294_967_296,
    memory_percent: 50,
  },
  network: {
    available: true,
    error: '',
    received_bytes_per_second: 2048,
    sent_bytes_per_second: 4096,
    received_packets_per_second: 10,
    sent_packets_per_second: 20,
    receive_errors_per_second: 0,
    send_errors_per_second: 0,
    receive_drops_per_second: 0,
    send_drops_per_second: 0,
  },
}

function report(): ResourceMonitorReport {
  const current = structuredClone(resources)
  return {
    meta: {
      started: true,
      node_name: 'node-a',
      hostname: 'host-a',
      os: 'linux',
      arch: 'arm64',
      cpu_cores: 4,
      started_at: 1_792_032_000,
      sampled_at: resources.timestamp,
      requested_minutes: 60,
      window_start: 1_792_032_000,
      window_end: resources.timestamp,
      sample_interval_seconds: 5,
      aggregation_interval_seconds: 60,
      retention_hours: 24,
      storage: { mode: 'memory', capacity_points: 1440 },
    },
    current,
    history: [
      {
        timestamp: resources.timestamp - 60,
        resources: {
          ...structuredClone(resources),
          timestamp: resources.timestamp - 60,
        },
      },
      { timestamp: resources.timestamp, resources: structuredClone(resources) },
    ],
    traffic: [
      {
        scope: 'inbound',
        category: 'relay',
        requests: 42,
        completed: 40,
        errors: 2,
        active: 2,
        received_bytes: 1024,
        sent_bytes: 8192,
        redirects: 1,
        duration_ms: 2000,
        avg_duration_ms: 50,
        connections: 0,
      },
      {
        scope: 'outbound',
        category: 'relay',
        requests: 40,
        completed: 38,
        errors: 1,
        active: 2,
        received_bytes: 16384,
        sent_bytes: 2048,
        redirects: 0,
        duration_ms: 2000,
        avg_duration_ms: 50,
        connections: 0,
      },
      {
        scope: 'database',
        category: 'other',
        requests: 7,
        completed: 7,
        errors: 0,
        active: 0,
        received_bytes: 4096,
        sent_bytes: 1024,
        redirects: 0,
        duration_ms: 70,
        avg_duration_ms: 10,
        connections: 3,
      },
    ],
  }
}

let client: QueryClient

function showMonitor(active = true) {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={client}>
      <SystemResourceMonitor active={active} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
})

afterEach(() => {
  client?.clear()
  vi.useRealTimers()
})

describe('resource monitoring', () => {
  test('shows current node resources and selected-window application counters in separate scopes', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: report() } })
    showMonitor()
    expect(await screen.findByText('node-a')).toBeInTheDocument()
    expect(screen.getByText('12.5%')).toBeInTheDocument()
    expect(screen.getByText('128 MiB')).toBeInTheDocument()
    const relayRow = within(screen.getByRole('table')).getByRole('row', {
      name: /Relay/,
    })
    expect(within(relayRow).getByText('42')).toBeInTheDocument()
    expect(within(relayRow).getByText('8 KiB')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Database' }))
    const otherRow = within(screen.getByRole('table')).getByRole('row', {
      name: /Other/,
    })
    expect(within(otherRow).getByText('7')).toBeInTheDocument()
    expect(screen.getByText(/Database and cache bytes/)).toBeInTheDocument()
    expect(get).toHaveBeenCalledTimes(1)
  })

  test('loads the chosen history period and keeps the selection accessible', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: report() } })
    showMonitor()
    await screen.findByText('node-a')
    fireEvent.click(screen.getByRole('button', { name: '24 hours' }))
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        '/api/system-info/monitor',
        expect.objectContaining({ params: { minutes: 1440 } })
      )
    )
    expect(screen.getByRole('button', { name: '24 hours' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  test('shows unavailable hardware as unavailable while retaining valid process memory', async () => {
    const data = report()
    data.current.process.cpu_available = false
    data.current.network.available = false
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    showMonitor()
    await screen.findByText('node-a')
    expect(screen.getByText('128 MiB')).toBeInTheDocument()
    expect(screen.queryByText('12.5%')).not.toBeInTheDocument()
    expect(screen.getAllByText('Unavailable').length).toBeGreaterThan(0)
  })

  test('shows fresh-session collection and memory-only history without a disk warning after a restart', async () => {
    const data = report()
    data.history = []
    data.traffic = []
    data.meta.started_at = data.meta.sampled_at
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    showMonitor()
    expect(
      await screen.findByText('Collecting resource history')
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'History is stored only in memory and is cleared when this node restarts.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: '7 days' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '24 hours' })).toBeInTheDocument()
    expect(
      screen.getByText('No application traffic recorded in this period.')
    ).toBeInTheDocument()
  })

  test('offers seven days only when the reported memory window supports it and retains the selected range across page tabs', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (_, config) => {
      const data = report()
      data.meta.retention_hours = 168
      data.meta.requested_minutes = config?.params?.minutes ?? 60
      return { data: { success: true, data } }
    })
    const view = showMonitor()
    expect(
      screen.queryByRole('button', { name: '7 days' })
    ).not.toBeInTheDocument()
    fireEvent.click(await screen.findByRole('button', { name: '7 days' }))
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        '/api/system-info/monitor',
        expect.objectContaining({ params: { minutes: 10080 } })
      )
    )
    view.rerender(
      <QueryClientProvider client={client}>
        <SystemResourceMonitor active={false} />
      </QueryClientProvider>
    )
    view.rerender(
      <QueryClientProvider client={client}>
        <SystemResourceMonitor active />
      </QueryClientProvider>
    )
    expect(screen.getByRole('button', { name: '7 days' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })

  test('reduces an unsupported selected window when the node reports a shorter memory retention', async () => {
    let retentionHours = 168
    const get = vi.spyOn(api, 'get').mockImplementation(async (_, config) => {
      const data = report()
      data.meta.retention_hours = retentionHours
      data.meta.requested_minutes = Math.min(
        config?.params?.minutes ?? 60,
        retentionHours * 60
      )
      return { data: { success: true, data } }
    })
    showMonitor()
    fireEvent.click(await screen.findByRole('button', { name: '7 days' }))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Refresh resource monitoring' })
      ).toBeEnabled()
    )
    retentionHours = 24
    fireEvent.click(
      screen.getByRole('button', { name: 'Refresh resource monitoring' })
    )
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: '7 days' })
      ).not.toBeInTheDocument()
    )
    expect(screen.getByRole('button', { name: '24 hours' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        '/api/system-info/monitor',
        expect.objectContaining({ params: { minutes: 1440 } })
      )
    )
  })

  test('keeps the one-hour selection available at the minimum memory retention', async () => {
    const data = report()
    data.meta.retention_hours = 1
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    showMonitor()
    await screen.findByText('node-a')
    expect(screen.getByRole('button', { name: '1 hour' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(
      screen.queryByRole('button', { name: '6 hours' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: '24 hours' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: '7 days' })
    ).not.toBeInTheDocument()
  })

  test('shows a retryable request failure without presenting an empty successful report', async () => {
    vi.spyOn(api, 'get').mockRejectedValue(new Error('unavailable'))
    showMonitor()
    expect(
      await screen.findByText('Unable to load resource monitoring')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(
      screen.queryByText('No application traffic recorded in this period.')
    ).not.toBeInTheDocument()
  })

  test('shows a stopped sampler without presenting zero-valued live metrics', async () => {
    const data = report()
    data.meta.started = false
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    showMonitor()
    expect(
      await screen.findByText(
        'Resource monitoring is not running on this node.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText('Active requests')).not.toBeInTheDocument()
  })

  test('requests nothing while its page tab is inactive', () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: report() } })
    showMonitor(false)
    expect(get).not.toHaveBeenCalled()
  })

  test('opens monitoring first and fetches existing system panels only after their tabs are selected', async () => {
    const get = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: url === '/api/system-info/monitor' ? report() : [],
      },
    }))
    client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    render(
      <QueryClientProvider client={client}>
        <SystemInfo />
      </QueryClientProvider>
    )
    await screen.findByText('node-a')
    expect(
      screen.getByRole('tab', { name: 'Resource monitoring' })
    ).toHaveAttribute('aria-selected', 'true')
    expect(get).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('tab', { name: 'Instances' }))
    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/system-info/instances')
    )
    expect(
      screen.getByRole('tab', { name: 'Resource monitoring' })
    ).toHaveAttribute('aria-selected', 'false')
    fireEvent.click(screen.getByRole('tab', { name: 'Background tasks' }))
    await waitFor(() => expect(get).toHaveBeenCalledTimes(3))
    expect(
      screen.getByRole('tab', { name: 'Background tasks' })
    ).toHaveAttribute('aria-selected', 'true')
  })

  test('polls every 15 seconds only while the page and monitoring tab are visible', async () => {
    vi.useFakeTimers()
    let visibility: DocumentVisibilityState = 'visible'
    vi.spyOn(document, 'visibilityState', 'get').mockImplementation(
      () => visibility
    )
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: report() } })
    const view = showMonitor()
    await act(() => vi.advanceTimersByTimeAsync(1))
    expect(get).toHaveBeenCalledTimes(1)
    await act(() => vi.advanceTimersByTimeAsync(15_000))
    expect(get).toHaveBeenCalledTimes(2)
    visibility = 'hidden'
    fireEvent(document, new Event('visibilitychange'))
    await act(() => vi.advanceTimersByTimeAsync(30_000))
    expect(get).toHaveBeenCalledTimes(2)
    visibility = 'visible'
    fireEvent(document, new Event('visibilitychange'))
    await act(() => vi.advanceTimersByTimeAsync(1))
    expect(get).toHaveBeenCalledTimes(3)
    view.rerender(
      <QueryClientProvider client={client}>
        <SystemResourceMonitor active={false} />
      </QueryClientProvider>
    )
    await act(() => vi.advanceTimersByTimeAsync(30_000))
    expect(get).toHaveBeenCalledTimes(3)
  })
})
