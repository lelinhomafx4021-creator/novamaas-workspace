import { apiRequest } from './request'

export interface PlatformStatus {
  custom_currency_exchange_rate?: number
  custom_currency_symbol?: string
  display_in_currency?: boolean
  quota_display_type?: 'CNY' | 'CUSTOM' | 'TOKENS' | 'USD'
  quota_per_unit?: number
  setup: boolean
  system_name: string
  usd_exchange_rate?: number
  version: string
}

let statusRequest: Promise<PlatformStatus> | undefined

export function getPlatformStatus() {
  statusRequest ??= apiRequest<PlatformStatus>('/api/status', { auth: false })
    .finally(() => {
      statusRequest = undefined
    })
  return statusRequest
}
