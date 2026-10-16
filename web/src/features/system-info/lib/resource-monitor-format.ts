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
import type { ResourceHistoryPoint } from '../types'

export function formatResourceBytes(bytes: number | null | undefined): string {
  if (bytes == null || !Number.isFinite(bytes) || bytes < 0) return '—'
  if (bytes === 0) return '0 B'
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  const index = Math.min(
    Math.max(0, Math.floor(Math.log(bytes) / Math.log(1024))),
    units.length - 1
  )
  return `${new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(bytes / 1024 ** index)} ${units[index]}`
}

export function formatResourceNumber(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '—'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 }).format(
    value
  )
}

type ResourceChartPoint = {
  timestamp: number
  received: number | null
  sent: number | null
  cpu: number | null
  hostMemory: number | null
  rss: number | null
  activeRequests: number | null
  connections: number | null
}

export function buildResourceHistory(
  history: ResourceHistoryPoint[],
  minutes = 60
): ResourceChartPoint[] {
  const data: ResourceChartPoint[] = []
  const bucketSeconds = Math.max(60, (minutes * 60) / 360)
  let previousTimestamp: number | null = null
  for (const point of history) {
    if (
      previousTimestamp !== null &&
      point.timestamp - previousTimestamp > bucketSeconds * 2
    ) {
      data.push({
        timestamp: (previousTimestamp + point.timestamp) / 2,
        received: null,
        sent: null,
        cpu: null,
        hostMemory: null,
        rss: null,
        activeRequests: null,
        connections: null,
      })
    }
    data.push({
      timestamp: point.timestamp,
      received: point.resources.network.available
        ? point.resources.network.received_bytes_per_second
        : null,
      sent: point.resources.network.available
        ? point.resources.network.sent_bytes_per_second
        : null,
      cpu: point.resources.process.cpu_available
        ? point.resources.process.cpu_percent
        : null,
      hostMemory: point.resources.host.memory_available
        ? point.resources.host.memory_percent
        : null,
      rss: point.resources.process.memory_available
        ? point.resources.process.rss_bytes
        : null,
      activeRequests: point.resources.activity?.available
        ? point.resources.activity.active_requests
        : null,
      connections: point.resources.activity?.available
        ? point.resources.activity.connections
        : null,
    })
    previousTimestamp = point.timestamp
  }
  return data
}
