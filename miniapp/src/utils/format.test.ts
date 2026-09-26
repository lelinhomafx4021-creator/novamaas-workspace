import { describe, expect, it } from 'vitest'

import {
  defaultQuotaDisplayConfig,
  formatBillingCurrencyFromUSD,
  formatCurrencyFromUSD,
  formatDuration,
  formatLocalCurrencyAmount,
  formatQuota,
} from './format'

describe('quota and duration formatting', () => {
  it('formats raw quota with the platform currency semantics used by Web', () => {
    expect(formatQuota(500_000)).toBe('$1')
    expect(
      formatQuota(500_000, {
        ...defaultQuotaDisplayConfig,
        quotaDisplayType: 'CNY',
        usdExchangeRate: 7,
      })
    ).toBe('¥7')
    expect(
      formatQuota(500_000, {
        ...defaultQuotaDisplayConfig,
        quotaDisplayType: 'TOKENS',
      })
    ).toBe('500,000')
  })

  it('formats request and task durations without losing sub-second values', () => {
    expect(formatDuration(0.25)).toBe('250 ms')
    expect(formatDuration(2.5)).toBe('2.5 s')
    expect(formatDuration(65)).toBe('1m 5s')
  })

  it('uses the configured display currency for USD and local payment amounts', () => {
    const cny = {
      ...defaultQuotaDisplayConfig,
      quotaDisplayType: 'CNY' as const,
      usdExchangeRate: 7,
    }
    expect(formatCurrencyFromUSD(2, cny)).toBe('¥14')
    expect(formatBillingCurrencyFromUSD(0.004, cny)).toBe('¥0.028')
    expect(formatLocalCurrencyAmount(12.5, cny)).toBe('¥12.5')
    expect(
      formatBillingCurrencyFromUSD(2, {
        ...defaultQuotaDisplayConfig,
        quotaDisplayType: 'TOKENS',
      })
    ).toBe('$2')
  })
})
