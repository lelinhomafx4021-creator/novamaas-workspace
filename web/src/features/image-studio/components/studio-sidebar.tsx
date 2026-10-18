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
  Add01Icon,
  ArrowUpRight01Icon,
  Clock01Icon,
  Image01Icon,
  Key01Icon,
  SparklesIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Separator } from '@/components/ui/separator'
import { useAuthStore } from '@/stores/auth-store'

export function StudioSidebar(props: {
  authenticated: boolean
  disabled: boolean
  onNewCreation: () => void
  onHistory: () => void
}) {
  const { t } = useTranslation()
  const username = useAuthStore((state) => state.auth.user?.username)

  return (
    <aside className='studio-sidebar' aria-label={t('Workspace')}>
      <div className='studio-sidebar-top'>
        <p className='studio-eyebrow'>{t('Workspace')}</p>
        <Button
          size='lg'
          className='w-full'
          onClick={props.onNewCreation}
          disabled={props.disabled}
        >
          <HugeiconsIcon icon={Add01Icon} aria-hidden='true' />
          {t('New creation')}
        </Button>
      </div>
      <nav className='studio-navigation' aria-label={t('Studio navigation')}>
        <Button
          variant='secondary'
          aria-current='page'
          onClick={() =>
            document
              .querySelector<HTMLTextAreaElement>('#studio-prompt')
              ?.focus()
          }
        >
          <HugeiconsIcon icon={Image01Icon} aria-hidden='true' />
          {t('Image creation')}
          <span className='studio-nav-dot' aria-hidden='true' />
        </Button>
        <Button
          variant='ghost'
          onClick={props.onHistory}
          aria-controls='studio-history'
        >
          <HugeiconsIcon icon={Clock01Icon} aria-hidden='true' />
          {t('Generation records')}
        </Button>
        {props.authenticated && (
          <Button
            variant='ghost'
            nativeButton={false}
            role='link'
            render={<Link to='/keys' />}
          >
            <HugeiconsIcon icon={Key01Icon} aria-hidden='true' />
            {t('API Keys')}
            <HugeiconsIcon
              icon={ArrowUpRight01Icon}
              className='ml-auto size-3.5'
              aria-hidden='true'
            />
          </Button>
        )}
      </nav>
      <div className='studio-sidebar-bottom'>
        <div className='studio-note'>
          <HugeiconsIcon
            icon={SparklesIcon}
            className='size-5'
            aria-hidden='true'
          />
          <p>{t('A space for your next idea.')}</p>
          <span>{t('Your API key. Your models. Your imagination.')}</span>
        </div>
        <Separator />
        <div className='studio-profile'>
          <span className='studio-avatar' aria-hidden='true'>
            {username?.slice(0, 1).toUpperCase() || 'N'}
          </span>
          <div className='min-w-0'>
            <p className='truncate'>{username || t('Guest')}</p>
            <span>{t('Personal workspace')}</span>
          </div>
        </div>
      </div>
    </aside>
  )
}
