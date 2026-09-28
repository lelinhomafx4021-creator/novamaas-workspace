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
import {
  AiNetworkIcon,
  ArrowRight01Icon,
  CloudServerIcon,
  DatabaseSync01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

export function TokenNetworkVisual() {
  const { t } = useTranslation()

  return (
    <div
      className='maas-workbench w-full rounded-[1.75rem] p-5 sm:p-7'
      role='group'
      aria-label={t('Gateway workbench')}
    >
      <div className='maas-workbench-header flex items-center justify-between gap-3 border-b pb-5'>
        <span className='maas-workbench-eyebrow flex items-center gap-2 text-sm font-semibold'>
          <span
            aria-hidden
            className='maas-workbench-mark size-2 rounded-full'
          />
          {t('Gateway workbench')}
        </span>
        <span className='maas-workbench-status shrink-0 rounded-full px-2.5 py-1 text-[10px] font-medium sm:text-xs'>
          {t('Available now')}
        </span>
      </div>

      <div className='mt-7'>
        <p className='maas-workbench-label text-xs font-medium'>
          {t('One entry point')}
        </p>
        <div className='maas-workbench-entry mt-3 flex min-w-0 items-center justify-between gap-2 rounded-xl px-3 py-3.5 sm:px-4'>
          <span className='maas-workbench-endpoint min-w-0 font-mono text-xs font-medium sm:text-sm'>
            POST /v1/chat/completions
          </span>
          <HugeiconsIcon
            icon={ArrowRight01Icon}
            className='maas-workbench-arrow size-4 shrink-0'
            aria-hidden='true'
          />
        </div>
      </div>

      <div className='maas-workbench-stage mt-4 flex items-center gap-3 rounded-2xl px-4 py-4'>
        <span className='maas-workbench-gateway-icon flex size-10 shrink-0 items-center justify-center rounded-xl'>
          <HugeiconsIcon
            icon={AiNetworkIcon}
            className='size-5'
            aria-hidden='true'
          />
        </span>
        <span className='min-w-0'>
          <span className='block text-sm font-semibold'>
            {t('Unified gateway access')}
          </span>
          <span className='maas-workbench-muted mt-0.5 block text-xs'>
            {t('Policy · Routing · Billing')}
          </span>
        </span>
      </div>

      <div className='mt-7'>
        <p className='maas-workbench-label text-xs font-medium'>
          {t('Connected supply')}
        </p>
        <div className='mt-3 grid gap-2 sm:grid-cols-2'>
          {[
            {
              icon: CloudServerIcon,
              label: t('Model providers'),
              meta: t('Public API supply'),
            },
            {
              icon: DatabaseSync01Icon,
              label: t('Private token pools'),
              meta: t('Managed inventory'),
            },
          ].map((source) => (
            <div
              key={source.label}
              className='maas-workbench-source flex min-w-0 items-center gap-2.5 rounded-xl px-3 py-3'
            >
              <HugeiconsIcon
                icon={source.icon}
                className='maas-workbench-source-icon size-4 shrink-0'
                aria-hidden='true'
              />
              <span className='min-w-0'>
                <span className='block truncate text-xs font-medium'>
                  {source.label}
                </span>
                <span className='maas-workbench-muted mt-0.5 block truncate text-[11px]'>
                  {source.meta}
                </span>
              </span>
            </div>
          ))}
        </div>
      </div>

      <div className='maas-workbench-next mt-7 flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t pt-5'>
        <span className='text-sm font-medium'>
          {t('Evaluated token supply')}
        </span>
        <Badge
          variant='outline'
          className='maas-workbench-roadmap h-5 rounded-full px-2 text-[10px]'
        >
          {t('Roadmap')}
        </Badge>
      </div>
    </div>
  )
}
