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
import { expect, test } from 'vitest'

import type { ResourceHistoryPoint, ResourcePoint } from '../../types'
import {
  buildResourceHistory,
  formatResourceBytes,
} from '../resource-monitor-format'

test('keeps the API history timestamps and separates unavailable metrics from measured zero', () => {
  const history: ResourceHistoryPoint[] = [
    {
      timestamp: 120,
      resources: {
        timestamp: 120,
        process: {
          available: true,
          cpu_available: false,
          memory_available: true,
          cpu_percent: 0,
          rss_bytes: 1024,
        },
        host: {
          available: false,
          cpu_available: false,
          memory_available: false,
          cpu_percent: 0,
          memory_percent: 0,
          memory_used_bytes: 0,
          memory_total_bytes: 0,
        },
        network: {
          available: false,
          received_bytes_per_second: 0,
          sent_bytes_per_second: 0,
          received_packets_per_second: 0,
          sent_packets_per_second: 0,
          receive_errors_per_second: 0,
          send_errors_per_second: 0,
          receive_drops_per_second: 0,
          send_drops_per_second: 0,
        },
      },
    },
  ]
  expect(buildResourceHistory(history)).toEqual([
    {
      timestamp: 120,
      cpu: null,
      rss: 1024,
      hostMemory: null,
      received: null,
      sent: null,
      activeRequests: null,
      connections: null,
    },
  ])
  history[0].resources.process.cpu_available = true
  history[0].resources.network.available = true
  expect(buildResourceHistory(history)[0]).toEqual({
    timestamp: 120,
    cpu: 0,
    rss: 1024,
    hostMemory: null,
    received: 0,
    sent: 0,
    activeRequests: null,
    connections: null,
  })
})

test('formats sub-byte rates using bytes without an invalid unit', () => {
  expect(formatResourceBytes(0.5)).toBe('0.5 B')
  expect(formatResourceBytes(0)).toBe('0 B')
  expect(formatResourceBytes(null)).toBe('—')
})

test('shows a sampling gap instead of interpolating traffic across unobserved time', () => {
  const resource = {
    timestamp: 120,
    process: {
      available: true,
      cpu_available: true,
      memory_available: true,
      cpu_percent: 1,
      rss_bytes: 1024,
    },
    host: {
      available: true,
      cpu_available: true,
      memory_available: true,
      cpu_percent: 2,
      memory_percent: 50,
      memory_used_bytes: 2048,
      memory_total_bytes: 4096,
    },
    network: {
      available: true,
      received_bytes_per_second: 100,
      sent_bytes_per_second: 200,
      received_packets_per_second: 1,
      sent_packets_per_second: 2,
      receive_errors_per_second: 0,
      send_errors_per_second: 0,
      receive_drops_per_second: 0,
      send_drops_per_second: 0,
    },
  }
  const history: ResourceHistoryPoint[] = [
    { timestamp: 120, resources: resource },
    { timestamp: 180, resources: { ...resource, timestamp: 180 } },
    { timestamp: 780, resources: { ...resource, timestamp: 780 } },
  ]
  const result = buildResourceHistory(history, 60)
  expect(result).toHaveLength(4)
  expect(result[2]).toEqual({
    timestamp: 480,
    cpu: null,
    rss: null,
    hostMemory: null,
    received: null,
    sent: null,
    activeRequests: null,
    connections: null,
  })
  expect(result[3]).toMatchObject({ timestamp: 780, received: 100, sent: 200 })
})

test('preserves ordinary seven-day bucket spacing without introducing a gap', () => {
  const resource = {
    timestamp: 100,
    process: {
      available: false,
      cpu_available: false,
      memory_available: false,
      cpu_percent: 0,
      rss_bytes: 0,
    },
    host: {
      available: false,
      cpu_available: false,
      memory_available: false,
      cpu_percent: 0,
      memory_percent: 0,
      memory_used_bytes: 0,
      memory_total_bytes: 0,
    },
    network: {
      available: false,
      received_bytes_per_second: 0,
      sent_bytes_per_second: 0,
      received_packets_per_second: 0,
      sent_packets_per_second: 0,
      receive_errors_per_second: 0,
      send_errors_per_second: 0,
      receive_drops_per_second: 0,
      send_drops_per_second: 0,
    },
  }
  const history = [
    { timestamp: 100, resources: resource },
    { timestamp: 1780, resources: { ...resource, timestamp: 1780 } },
  ]
  expect(buildResourceHistory(history, 10080)).toHaveLength(2)
})

test('preserves sampled activity peaks and leaves missing activity and sampling gaps unavailable', () => {
  const resource: ResourcePoint = {
    timestamp: 120,
    process: {
      available: false,
      cpu_available: false,
      memory_available: false,
      cpu_percent: 0,
      rss_bytes: 0,
    },
    host: {
      available: false,
      cpu_available: false,
      memory_available: false,
      cpu_percent: 0,
      memory_percent: 0,
      memory_used_bytes: 0,
      memory_total_bytes: 0,
    },
    network: {
      available: false,
      received_bytes_per_second: 0,
      sent_bytes_per_second: 0,
      received_packets_per_second: 0,
      sent_packets_per_second: 0,
      receive_errors_per_second: 0,
      send_errors_per_second: 0,
      receive_drops_per_second: 0,
      send_drops_per_second: 0,
    },
    activity: { available: true, active_requests: 3, connections: 12 },
  }
  const history = [
    { timestamp: 120, resources: resource },
    { timestamp: 180, resources: { ...resource, activity: undefined } },
    {
      timestamp: 780,
      resources: {
        ...resource,
        activity: { available: true, active_requests: 0, connections: 0 },
      },
    },
  ]
  const result = buildResourceHistory(history, 60)
  expect(result[0]).toMatchObject({ activeRequests: 3, connections: 12 })
  expect(result[1]).toMatchObject({ activeRequests: null, connections: null })
  expect(result[2]).toMatchObject({
    timestamp: 480,
    activeRequests: null,
    connections: null,
  })
  expect(result[3]).toMatchObject({ activeRequests: 0, connections: 0 })
  const unavailable = {
    ...resource,
    activity: { available: false, active_requests: 99, connections: 99 },
  }
  expect(
    buildResourceHistory([{ timestamp: 120, resources: unavailable }])[0]
  ).toMatchObject({ activeRequests: null, connections: null })
})
