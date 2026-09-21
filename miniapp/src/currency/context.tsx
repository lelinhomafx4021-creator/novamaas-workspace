import { createContext, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react'

import { getPlatformStatus } from '@/api/status'
import { quotaDisplayConfigFromStatus } from '@/currency/config'
import {
  defaultQuotaDisplayConfig,
  formatBillingCurrencyFromUSD,
  formatCurrencyFromUSD,
  formatLocalCurrencyAmount,
  formatQuota,
  type CurrencyFormatOptions,
  type QuotaDisplayConfig,
} from '@/utils/format'

const QuotaDisplayContext = createContext<QuotaDisplayConfig | null>(null)

export function QuotaDisplayProvider({ children }: PropsWithChildren) {
  const [config, setConfig] = useState<QuotaDisplayConfig | null>(null)

  useEffect(() => {
    let active = true
    void getPlatformStatus()
      .then((status) => {
        if (active) setConfig(quotaDisplayConfigFromStatus(status))
      })
      .catch(() => undefined)
    return () => {
      active = false
    }
  }, [])

  return <QuotaDisplayContext.Provider value={config}>{children}</QuotaDisplayContext.Provider>
}

export function useQuotaDisplay() {
  const config = useContext(QuotaDisplayContext)
  return useMemo(
    () => ({
      config: config ?? defaultQuotaDisplayConfig,
      ready: config !== null,
      formatBillingCurrencyFromUSD: (
        value: number | undefined,
        options?: CurrencyFormatOptions
      ) => config ? formatBillingCurrencyFromUSD(value, config, options) : '—',
      formatCurrencyFromUSD: (
        value: number | undefined,
        options?: CurrencyFormatOptions
      ) => config ? formatCurrencyFromUSD(value, config, options) : '—',
      formatLocalCurrencyAmount: (
        value: number | undefined,
        options?: CurrencyFormatOptions
      ) => config ? formatLocalCurrencyAmount(value, config, options) : '—',
      formatQuota: (value: number | undefined) => config ? formatQuota(value, config) : '—',
    }),
    [config]
  )
}
