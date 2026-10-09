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
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'

import type { CorrectionPricingEvidence as PricingEvidence } from '../correction-api'

function readPricingEvidence(value?: string): PricingEvidence | undefined {
  if (!value) return undefined
  try {
    const parsed: unknown = JSON.parse(value)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return undefined
    }
    const evidence = parsed as Record<string, unknown>
    const result: PricingEvidence = {}
    if (typeof evidence.resolution === 'string') {
      result.resolution = evidence.resolution.trim() || undefined
    }
    if (typeof evidence.requested_resolution === 'string') {
      result.requested_resolution = evidence.requested_resolution
    }
    if (typeof evidence.resolution_source === 'string') {
      result.resolution_source = evidence.resolution_source
    }
    if (typeof evidence.has_video === 'boolean') {
      result.has_video = evidence.has_video
    }
    if (typeof evidence.per_call_billing === 'boolean') {
      result.per_call_billing = evidence.per_call_billing
    }
    if (
      evidence.pricing_mode === 'tokens' ||
      evidence.pricing_mode === 'per_call'
    ) {
      result.pricing_mode = evidence.pricing_mode
    }
    for (const name of [
      'model_price',
      'model_ratio',
      'total_tokens',
    ] as const) {
      const number = evidence[name]
      if (number == null) continue
      if (
        typeof number !== 'number' ||
        !Number.isFinite(number) ||
        (number < 0 &&
          !(
            name === 'model_price' &&
            number === -1 &&
            result.pricing_mode === 'tokens'
          ))
      ) {
        return undefined
      }
      if (name === 'total_tokens' && !Number.isSafeInteger(number)) {
        return undefined
      }
      result[name] = number
    }
    if (evidence.other_ratios != null) {
      if (
        typeof evidence.other_ratios !== 'object' ||
        Array.isArray(evidence.other_ratios)
      ) {
        return undefined
      }
      const ratios = evidence.other_ratios as Record<string, unknown>
      result.other_ratios = {}
      for (const [name, ratio] of Object.entries(ratios)) {
        if (
          typeof ratio !== 'number' ||
          !Number.isFinite(ratio) ||
          ratio <= 0
        ) {
          return undefined
        }
        result.other_ratios[name] = ratio
      }
    }
    if (Object.keys(result).length === 0) return undefined
    return result
  } catch {
    return undefined
  }
}

export function CorrectionPricingEvidence(props: { value?: string }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const pricing = readPricingEvidence(props.value)
  if (!pricing) {
    return (
      <span className='text-muted-foreground'>
        {t('Pricing evidence unavailable')}
      </span>
    )
  }
  const videoRatio = pricing.other_ratios?.video_input ?? 1
  const tokens =
    pricing.pricing_mode === 'tokens' ||
    (pricing.per_call_billing === false &&
      (pricing.model_ratio ?? 0) > 0 &&
      (pricing.total_tokens ?? 0) > 0)
  let unitPrice: number | undefined
  if (tokens && pricing.model_ratio != null) {
    unitPrice = pricing.model_ratio * 2 * videoRatio
  } else if (!tokens && pricing.model_price != null) {
    unitPrice = pricing.model_price * videoRatio
  }
  let videoInput = t('Unknown')
  if (pricing.has_video != null) {
    videoInput = pricing.has_video ? t('Yes') : t('No')
  }
  const rows = [
    { label: t('Resolution'), value: pricing.resolution ?? t('Unknown') },
    { label: t('Video input'), value: videoInput },
    {
      label: t('Total Tokens'),
      value: pricing.total_tokens?.toLocaleString(locale) ?? t('Unknown'),
    },
    {
      label: tokens ? t('Unit price per million tokens') : t('Model Price'),
      value:
        unitPrice == null || !Number.isFinite(unitPrice)
          ? t('Unknown')
          : formatBillingCurrencyFromUSD(unitPrice, {
              digitsLarge: 6,
              digitsSmall: 6,
              abbreviate: false,
            }),
    },
    {
      label: t('Video pricing multiplier'),
      value: `${videoRatio.toLocaleString(locale, { maximumFractionDigits: 6 })}x`,
    },
  ]
  if (pricing.requested_resolution != null || pricing.resolution_source) {
    rows.push({
      label: t('Requested resolution'),
      value: pricing.requested_resolution || t('Automatic'),
    })
  }
  if (pricing.resolution_source) {
    let source = t('Unknown')
    if (pricing.resolution_source === 'upstream_response') {
      source = t('Upstream task response')
    }
    if (pricing.resolution_source === 'archived_request') {
      source = t('Archived request')
    }
    rows.push({ label: t('Resolution evidence source'), value: source })
  }
  if (tokens && pricing.model_ratio != null) {
    rows.push({
      label: t('Model ratio'),
      value: `${pricing.model_ratio.toLocaleString(locale, { maximumFractionDigits: 6 })}x`,
    })
  }
  for (const [name, ratio] of Object.entries(pricing.other_ratios ?? {}).sort(
    ([left], [right]) => left.localeCompare(right)
  )) {
    if (name === 'video_input') continue
    rows.push({
      label:
        name === 'seconds'
          ? t('Duration multiplier')
          : t('Multiplier {{name}}', { name }),
      value: `${ratio.toLocaleString(locale, { maximumFractionDigits: 6 })}x`,
    })
  }
  return (
    <div className='flex flex-col gap-1'>
      <dl className='grid gap-1'>
        {rows.map((row) => (
          <div
            key={row.label}
            className='flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5'
          >
            <dt className='text-muted-foreground'>{row.label}</dt>
            <dd className='break-words'>{row.value}</dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground'>
        {t('Unit prices exclude the billing group discount.')}
      </p>
    </div>
  )
}
