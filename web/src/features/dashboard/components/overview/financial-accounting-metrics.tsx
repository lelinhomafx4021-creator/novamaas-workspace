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
import { CircleDollarSign, TrendingUp, type LucideIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type {
  FinancialAccountingOverview,
  FinancialAccountingPeriodSummary,
} from '@/features/dashboard/api'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

function AccountingMetric(props: {
  icon: LucideIcon
  label: string
  value: string
  title?: string
  tone: 'neutral' | 'turnover' | 'profit' | 'pending'
}) {
  const Icon = props.icon
  return (
    <div className='bg-background/60 rounded-lg px-2.5 py-2'>
      <div className='text-muted-foreground flex items-center gap-1 text-[11px] leading-none font-medium'>
        <Icon className='size-3 shrink-0' aria-hidden='true' />
        <span className='truncate'>{props.label}</span>
      </div>
      <div
        title={props.title}
        className={cn(
          'mt-1.5 truncate text-xs font-semibold tabular-nums',
          props.tone === 'profit' && 'text-success',
          props.tone === 'pending' && 'text-warning'
        )}
      >
        {props.value}
      </div>
    </div>
  )
}

function getProfitMetric(
  period: FinancialAccountingPeriodSummary | undefined,
  pendingLabel: string,
  missingRecordsLabel: string
): { value: string; title?: string; tone: 'neutral' | 'profit' | 'pending' } {
  if (!period) {
    return { value: '—', tone: 'neutral' }
  }
  if (period.accounting_complete && period.profit_quota !== null) {
    return { value: formatQuota(period.profit_quota), tone: 'profit' }
  }
  return {
    value: pendingLabel,
    title: missingRecordsLabel,
    tone: 'pending',
  }
}

export function FinancialAccountingMetrics(props: {
  overview?: FinancialAccountingOverview | null
}) {
  const { t } = useTranslation()
  const today = props.overview?.today
  const yesterday = props.overview?.yesterday
  const todayProfit = getProfitMetric(
    today,
    t('Pending accounting'),
    t('{{count}} cost records are missing.', {
      count: today?.missing_cost_records ?? 0,
    })
  )
  const yesterdayProfit = getProfitMetric(
    yesterday,
    t('Pending accounting'),
    t('{{count}} cost records are missing.', {
      count: yesterday?.missing_cost_records ?? 0,
    })
  )

  return (
    <div className='grid grid-cols-2 gap-2'>
      <AccountingMetric
        icon={CircleDollarSign}
        label={t("Today's turnover")}
        value={today ? formatQuota(today.revenue_quota) : '—'}
        tone='turnover'
      />
      <AccountingMetric
        icon={TrendingUp}
        label={t("Today's profit")}
        value={todayProfit.value}
        title={todayProfit.title}
        tone={todayProfit.tone}
      />
      <AccountingMetric
        icon={CircleDollarSign}
        label={t("Yesterday's turnover")}
        value={yesterday ? formatQuota(yesterday.revenue_quota) : '—'}
        tone='turnover'
      />
      <AccountingMetric
        icon={TrendingUp}
        label={t("Yesterday's profit")}
        value={yesterdayProfit.value}
        title={yesterdayProfit.title}
        tone={yesterdayProfit.tone}
      />
    </div>
  )
}
