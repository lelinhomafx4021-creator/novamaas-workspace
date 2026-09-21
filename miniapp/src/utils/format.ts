export interface QuotaDisplayConfig {
  customCurrencyExchangeRate: number
  customCurrencySymbol: string
  quotaDisplayType: 'CNY' | 'CUSTOM' | 'TOKENS' | 'USD'
  quotaPerUnit: number
  usdExchangeRate: number
}

export interface CurrencyFormatOptions {
  digitsLarge?: number
  digitsSmall?: number
}

export const defaultQuotaDisplayConfig: QuotaDisplayConfig = {
  customCurrencyExchangeRate: 1,
  customCurrencySymbol: '¤',
  quotaDisplayType: 'USD',
  quotaPerUnit: 500_000,
  usdExchangeRate: 1,
}

function formatDecimal(value: number, options: CurrencyFormatOptions = {}) {
  const digits = Math.abs(value) >= 1
    ? (options.digitsLarge ?? 2)
    : (options.digitsSmall ?? 6)
  return value.toFixed(digits).replace(/\.?0+$/, '')
}

function formatCurrencyValue(
  value: number,
  config: QuotaDisplayConfig,
  options: CurrencyFormatOptions = {}
) {
  if (config.quotaDisplayType === 'CNY') {
    return `¥${formatDecimal(value, options)}`
  }
  if (config.quotaDisplayType === 'CUSTOM') {
    return `${config.customCurrencySymbol} ${formatDecimal(value, options)}`
  }
  return `$${formatDecimal(value, options)}`
}

export function formatCurrencyFromUSD(
  value: number | undefined,
  config: QuotaDisplayConfig = defaultQuotaDisplayConfig,
  options: CurrencyFormatOptions = {}
) {
  if (!Number.isFinite(value)) return '—'
  if (config.quotaDisplayType === 'TOKENS') {
    return formatNumber(Number(value) * config.quotaPerUnit)
  }
  const exchangeRate = config.quotaDisplayType === 'CNY'
    ? config.usdExchangeRate
    : config.quotaDisplayType === 'CUSTOM'
      ? config.customCurrencyExchangeRate
      : 1
  return formatCurrencyValue(Number(value) * exchangeRate, config, options)
}

export function formatBillingCurrencyFromUSD(
  value: number | undefined,
  config: QuotaDisplayConfig = defaultQuotaDisplayConfig,
  options: CurrencyFormatOptions = {}
) {
  if (!Number.isFinite(value)) return '—'
  if (config.quotaDisplayType === 'TOKENS') {
    return `$${formatDecimal(Number(value), options)}`
  }
  return formatCurrencyFromUSD(value, config, options)
}

export function formatLocalCurrencyAmount(
  value: number | undefined,
  config: QuotaDisplayConfig = defaultQuotaDisplayConfig,
  options: CurrencyFormatOptions = {}
) {
  if (!Number.isFinite(value)) return '—'
  if (config.quotaDisplayType === 'TOKENS') {
    return `$${formatDecimal(Number(value), options)}`
  }
  return formatCurrencyValue(Number(value), config, options)
}

export function formatQuota(
  value: number | undefined,
  config: QuotaDisplayConfig = defaultQuotaDisplayConfig
) {
  if (!Number.isFinite(value)) return '—'
  if (config.quotaDisplayType === 'TOKENS') return formatNumber(value)

  const quotaPerUnit = config.quotaPerUnit > 0
    ? config.quotaPerUnit
    : defaultQuotaDisplayConfig.quotaPerUnit
  return formatCurrencyFromUSD(Number(value) / quotaPerUnit, config)
}

export function formatNumber(value: number | undefined) {
  if (!Number.isFinite(value)) return '—'
  const [integer, decimal] = String(value ?? 0).split('.')
  const formatted = integer.replace(/\B(?=(\d{3})+(?!\d))/g, ',')
  return decimal ? `${formatted}.${decimal}` : formatted
}

export function formatTime(timestamp: number | undefined) {
  if (!timestamp) return '—'
  return new Date(timestamp * 1000).toLocaleString()
}

export function formatDuration(seconds: number | undefined) {
  if (!Number.isFinite(seconds) || Number(seconds) < 0) return '—'
  const value = Number(seconds)
  if (value < 1) return `${Math.round(value * 1000)} ms`
  if (value < 60) return `${formatDecimal(value)} s`
  const minutes = Math.floor(value / 60)
  const remainder = Math.floor(value % 60)
  return `${minutes}m ${remainder}s`
}

export function getTodayRange() {
  const start = new Date()
  start.setHours(0, 0, 0, 0)
  return {
    startTimestamp: Math.floor(start.getTime() / 1000),
    endTimestamp: Math.floor(Date.now() / 1000),
  }
}

export function getRecentRange(days: number) {
  return {
    startTimestamp: Math.floor((Date.now() - (days - 1) * 86_400_000) / 1000),
    endTimestamp: Math.floor(Date.now() / 1000),
  }
}
