import type { PlatformStatus } from '@/api/status'
import {
  defaultQuotaDisplayConfig,
  type QuotaDisplayConfig,
} from '@/utils/format'

function toPositiveNumber(value: unknown, fallback: number) {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : fallback
}

export function quotaDisplayConfigFromStatus(status: PlatformStatus): QuotaDisplayConfig {
  const displayType = status.quota_display_type ??
    (status.display_in_currency === false ? 'TOKENS' : 'USD')
  return {
    customCurrencyExchangeRate: toPositiveNumber(
      status.custom_currency_exchange_rate,
      defaultQuotaDisplayConfig.customCurrencyExchangeRate
    ),
    customCurrencySymbol:
      status.custom_currency_symbol?.trim() || defaultQuotaDisplayConfig.customCurrencySymbol,
    quotaDisplayType:
      displayType === 'CNY' || displayType === 'CUSTOM' || displayType === 'TOKENS' || displayType === 'USD'
        ? displayType
        : defaultQuotaDisplayConfig.quotaDisplayType,
    quotaPerUnit: toPositiveNumber(status.quota_per_unit, defaultQuotaDisplayConfig.quotaPerUnit),
    usdExchangeRate: toPositiveNumber(
      status.usd_exchange_rate,
      defaultQuotaDisplayConfig.usdExchangeRate
    ),
  }
}
