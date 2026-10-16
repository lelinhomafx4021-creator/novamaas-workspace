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
export type SystemInstanceStatus = 'online' | 'stale'

export type SystemInstanceInfo = {
  schema_version?: number
  node?: {
    name?: string
    source?: string
    manually_configured?: boolean
    should_configure_manually?: boolean
    [key: string]: unknown
  }
  role?: {
    is_master?: boolean
    [key: string]: unknown
  }
  runtime?: {
    version?: string
    goos?: string
    goarch?: string
    started_at?: number
    [key: string]: unknown
  }
  host?: {
    hostname?: string
    [key: string]: unknown
  }
  resources?: {
    cpu?: {
      usage_percent?: number
      [key: string]: unknown
    }
    memory?: {
      usage_percent?: number
      [key: string]: unknown
    }
    storage?: {
      total_bytes?: number
      used_bytes?: number
      free_bytes?: number
      used_percent?: number
      [key: string]: unknown
    }
    [key: string]: unknown
  }
  [key: string]: unknown
}

export type SystemInstance = {
  node_name: string
  status: SystemInstanceStatus
  stale_after_seconds: number
  started_at: number
  last_seen_at: number
  info?: SystemInstanceInfo
}

export type SystemInstanceListResponse = {
  success: boolean
  message: string
  data?: SystemInstance[]
}

export type SystemInstanceDeleteResponse = {
  success: boolean
  message: string
  data?: {
    deleted_count: number
  }
}

export type ResourceMonitorPeriod = 60 | 360 | 1440 | 10080
export type ResourceTrafficScope = 'inbound' | 'outbound' | 'database' | 'cache'
export type ResourceTrafficCategory =
  | 'relay'
  | 'video'
  | 'assets'
  | 'storage'
  | 'webhook'
  | 'admin'
  | 'other'

export type ResourcePoint = {
  timestamp: number
  activity?: {
    available: boolean
    active_requests: number
    connections: number
  }
  process: {
    available: boolean
    cpu_available: boolean
    memory_available: boolean
    error?: string
    cpu_percent: number
    rss_bytes: number
  }
  host: {
    available: boolean
    cpu_available: boolean
    memory_available: boolean
    error?: string
    cpu_percent: number
    memory_total_bytes: number
    memory_used_bytes: number
    memory_percent: number
    disk_available?: boolean
    disk_total_bytes?: number
    disk_used_bytes?: number
    disk_percent?: number
  }
  network: {
    available: boolean
    error?: string
    received_bytes_per_second: number
    sent_bytes_per_second: number
    received_packets_per_second: number
    sent_packets_per_second: number
    receive_errors_per_second: number
    send_errors_per_second: number
    receive_drops_per_second: number
    send_drops_per_second: number
  }
}

export type ResourceHistoryPoint = {
  timestamp: number
  resources: ResourcePoint
}

export type ResourceTrafficRow = {
  scope: ResourceTrafficScope
  category: ResourceTrafficCategory
  requests: number
  completed: number
  errors: number
  active: number
  received_bytes: number
  sent_bytes: number
  redirects: number
  duration_ms: number
  avg_duration_ms: number
  connections: number
}

export type ResourceMonitorReport = {
  meta: {
    started: boolean
    node_name: string
    hostname: string
    os: string
    arch: string
    cpu_cores: number
    network_interfaces?: string[]
    started_at: number
    sampled_at: number
    requested_minutes: number
    window_start: number
    window_end: number
    sample_interval_seconds: number
    aggregation_interval_seconds: number
    retention_hours: number
    storage: {
      mode: 'memory'
      capacity_points: number
    }
  }
  current: ResourcePoint
  traffic: ResourceTrafficRow[]
  history: ResourceHistoryPoint[]
}
