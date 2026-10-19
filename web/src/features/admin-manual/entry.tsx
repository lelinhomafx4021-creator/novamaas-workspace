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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { useAdminManualAccess } from './use-admin-manual-access'

export function DocumentationSwitcher(props: { kind: 'api' | 'admin' }) {
  const { t } = useTranslation()
  const access = useAdminManualAccess()
  if (!access.available) return null

  return (
    <nav
      aria-label={t('Docs')}
      className='bg-muted/60 mb-5 grid grid-cols-2 gap-1 rounded-lg border p-1'
    >
      <Link
        to='/docs'
        aria-current={props.kind === 'api' ? 'page' : undefined}
        className={cn(
          'rounded-md px-2 py-2 text-center text-xs font-medium transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring',
          props.kind === 'api'
            ? 'bg-background text-foreground shadow-xs'
            : 'text-muted-foreground'
        )}
      >
        {t('API documentation')}
      </Link>
      <Link
        to='/admin-manual'
        aria-current={props.kind === 'admin' ? 'page' : undefined}
        className={cn(
          'rounded-md px-2 py-2 text-center text-xs font-medium transition-colors hover:text-foreground focus-visible:outline-2 focus-visible:outline-ring',
          props.kind === 'admin'
            ? 'bg-background text-foreground shadow-xs'
            : 'text-muted-foreground'
        )}
      >
        {t('Platform integration guide')}
      </Link>
    </nav>
  )
}
