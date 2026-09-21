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
import { fireEvent, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import zh from '@/i18n/locales/zh.json'

import { testChannelNetwork } from '../../../api'
import { ChannelNetworkTestPanel } from '../channel-network-test-panel'

vi.mock('../../../api', () => ({
  testChannelNetwork: vi.fn(),
}))

describe('ChannelNetworkTestPanel', () => {
  beforeEach(() => {
    vi.mocked(testChannelNetwork).mockReset()
  })

  test('shows the channel address and measured network phases', async () => {
    vi.mocked(testChannelNetwork).mockResolvedValue({
      success: true,
      data: {
        target_url: 'https://video.example.com',
        resolved_addresses: ['203.0.113.10'],
        remote_address: '203.0.113.10:443',
        dns_ms: 3,
        connect_ms: 7,
        tls_ms: 9,
        ttfb_ms: 21,
        total_ms: 27,
        http_status: 200,
        protocol: 'HTTP/2.0',
        connection_reused: false,
        via_proxy: true,
      },
    })

    render(<ChannelNetworkTestPanel channelId={12} />)
    fireEvent.click(
      screen.getByRole('button', { name: 'Test network latency' })
    )

    expect(await screen.findByText('https://video.example.com')).toBeVisible()
    expect(screen.getByText('203.0.113.10:443')).toBeVisible()
    expect(screen.getByText('27 ms')).toBeVisible()
    expect(testChannelNetwork).toHaveBeenCalledWith(12)
  })

  test('renders network diagnostics with the active Chinese locale', async () => {
    const i18n = createInstance()
    await i18n.use(initReactI18next).init({
      lng: 'zh',
      resources: { zh: { translation: zh.translation } },
    })

    render(
      <I18nextProvider i18n={i18n}>
        <ChannelNetworkTestPanel channelId={12} />
      </I18nextProvider>
    )

    expect(screen.getByText('网络诊断')).toBeVisible()
    expect(screen.getByRole('button', { name: '测试网络延迟' })).toBeVisible()
  })
})
