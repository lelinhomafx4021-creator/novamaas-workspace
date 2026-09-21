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
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { describe, expect, test } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import {
  formatTaskRequestBytes,
  formatTaskRequestDuration,
} from '../../../lib/task-request-metrics-format'
import { TaskRequestMetricsCell } from '../task-request-metrics-cell'

const requestMetrics = {
  request_body_bytes: 1536,
  upstream_body_bytes: 1024,
  body_read_ms: 2,
  request_preparation_ms: 11,
  temporary_storage_ms: 310,
  upstream_request_ms: 805,
  total_ms: 1250,
  attempts: 1,
}

describe('TaskRequestMetricsCell', () => {
  test('formats byte sizes and durations for compact task rows', () => {
    expect(formatTaskRequestBytes(1536)).toBe('1.5 KB')
    expect(formatTaskRequestBytes(2 * 1024 * 1024)).toBe('2.0 MB')
    expect(formatTaskRequestDuration(1250)).toBe('1.25 s')
  })

  test('shows total request duration and original body size', () => {
    render(<TaskRequestMetricsCell metrics={requestMetrics} />)

    expect(screen.getByText('1.25 s')).toBeVisible()
    expect(screen.getByText('Request body: 1.5 KB')).toBeVisible()
  })

  test('renders request metrics with the active Chinese locale', async () => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'zh',
      resources: { zh: { translation: zh.translation } },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <TaskRequestMetricsCell metrics={requestMetrics} />
      </I18nextProvider>
    )

    expect(screen.getByText('请求体: 1.5 KB')).toBeVisible()
  })
})
