import { describe, expect, it } from 'vitest'

import { quotaDisplayConfigFromStatus } from './config'

describe('quota display status mapping', () => {
  it('uses the platform currency configuration returned by the public status API', () => {
    expect(
      quotaDisplayConfigFromStatus({
        custom_currency_exchange_rate: 0.9,
        custom_currency_symbol: '€',
        quota_display_type: 'CNY',
        quota_per_unit: 1_000_000,
        setup: true,
        system_name: 'Example',
        usd_exchange_rate: 7.2,
        version: 'test',
      })
    ).toEqual({
      customCurrencyExchangeRate: 0.9,
      customCurrencySymbol: '€',
      quotaDisplayType: 'CNY',
      quotaPerUnit: 1_000_000,
      usdExchangeRate: 7.2,
    })
  })
})
