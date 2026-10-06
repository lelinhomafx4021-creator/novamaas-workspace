import { describe, expect, it } from 'vitest'

import {
  defaultQuotaDisplayConfig,
  formatBillingCurrencyFromUSD,
  formatCurrencyFromUSD,
  formatDuration,
  formatLocalCurrencyAmount,
  formatQuota,
  formatTime,
  getTodayRange,
  getYesterdayRange,
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

describe('device-local time display and usage ranges', () => {
  it('formats Unix seconds in the device time zone with fixed-width fields', () => {
    expect(formatTime(new Date(2026, 9, 5, 0, 0, 0).getTime() / 1000)).toBe('2026-10-05 00:00:00')
    expect(formatTime(new Date(2026, 9, 5, 23, 59, 59).getTime() / 1000)).toBe('2026-10-05 23:59:59')
    expect(formatTime(0)).toBe('—')
  })

  it('uses complete device-local calendar days across a month boundary', () => {
    const now = new Date(2026, 9, 1, 8, 15).getTime()
    const todayStart = new Date(2026, 9, 1).getTime()
    expect(getTodayRange(now)).toEqual({
      startTimestamp: todayStart / 1000,
      endTimestamp: now / 1000,
    })
    expect(getYesterdayRange(now)).toEqual({
      startTimestamp: new Date(2026, 8, 30).getTime() / 1000,
      endTimestamp: todayStart / 1000 - 1,
    })
  })

  it('uses local date boundaries when a time zone changes daylight saving time', () => {
    const todayStart = new Date(2026, 2, 9).getTime()
    expect(getYesterdayRange(new Date(2026, 2, 9, 12).getTime())).toEqual({
      startTimestamp: new Date(2026, 2, 8).getTime() / 1000,
      endTimestamp: todayStart / 1000 - 1,
    })
  })
})
