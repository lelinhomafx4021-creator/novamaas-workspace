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
import { formatQuotaWithCurrency } from '@/lib/currency'

export function getCorrectionCostDelta(value: {
  current_cost_quota?: number
  corrected_cost_quota?: number
  cost_delta?: number
}): number | undefined {
  if (value.current_cost_quota == null || value.corrected_cost_quota == null) {
    return undefined
  }
  return (
    value.cost_delta ?? value.corrected_cost_quota - value.current_cost_quota
  )
}

export function formatCorrectionAmount(
  value: number | undefined,
  direction = 1
): string {
  if (value == null || !Number.isFinite(value)) return '—'
  const amount = value === 0 ? 0 : value * direction
  return formatQuotaWithCurrency(amount, {
    digitsLarge: 6,
    digitsSmall: 6,
    abbreviate: false,
  })
}
