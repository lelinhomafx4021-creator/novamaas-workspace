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
import { render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, expect, test, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import type { ResourcePoint } from '../../types'
import { ResourceMonitorCharts } from '../resource-monitor-charts'
import { ResourceTrafficPanel } from '../resource-traffic-panel'

afterEach(async () => {
  await i18next.changeLanguage('en')
  i18next.removeResourceBundle('zh', 'translation')
})

test('uses network byte and inflight meanings for Chinese traffic column labels', async () => {
  i18next.addResourceBundle('zh', 'translation', zh.translation)
  await i18next.changeLanguage('zh')
  render(
    <ResourceTrafficPanel
      traffic={[
        {
          scope: 'inbound',
          category: 'relay',
          requests: 1,
          completed: 0,
          errors: 0,
          active: 1,
          received_bytes: 16,
          sent_bytes: 32,
          redirects: 0,
          duration_ms: 0,
          avg_duration_ms: 0,
          connections: 0,
        },
      ]}
    />
  )
  expect(
    screen.getByRole('columnheader', { name: '接收字节' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('columnheader', { name: '发送字节' })
  ).toBeInTheDocument()
  expect(
    screen.getByRole('columnheader', { name: '活跃请求' })
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('columnheader', { name: '获得' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('columnheader', { name: '生效' })
  ).not.toBeInTheDocument()
})

test('uses network receive and send meanings for the Chinese throughput legend', async () => {
  i18next.addResourceBundle('zh', 'translation', zh.translation)
  await i18next.changeLanguage('zh')
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
    x: 0,
    y: 0,
    width: 640,
    height: 224,
    top: 0,
    left: 0,
    bottom: 224,
    right: 640,
    toJSON: () => ({}),
  })
  const resource: ResourcePoint = {
    timestamp: 120,
    activity: { available: true, active_requests: 3, connections: 12 },
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
  render(
    <ResourceMonitorCharts
      minutes={60}
      history={[
        { timestamp: 120, resources: resource },
        { timestamp: 180, resources: { ...resource, timestamp: 180 } },
      ]}
    />
  )
  expect(await screen.findByText('网络接收')).toBeInTheDocument()
  expect(screen.getByText('网络发送')).toBeInTheDocument()
  expect(screen.queryByText('获得')).not.toBeInTheDocument()
  const activityChart = screen.getByRole('img', { name: '请求与连接' })
  expect(activityChart).toBeInTheDocument()
  expect(within(activityChart).getByText('活跃请求')).toBeInTheDocument()
  expect(within(activityChart).getByText('连接数')).toBeInTheDocument()
  expect(screen.getByText('分钟采样峰值')).toBeInTheDocument()
})
