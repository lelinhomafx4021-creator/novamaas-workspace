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
    <aside
      className='col-start-1 row-span-2 hidden min-w-0 flex-col gap-5 border-r border-(--studio-subtle-border) bg-(--studio-surface) md:flex min-[75rem]:row-auto'
      aria-label={t('Workspace')}
    >
      <div className='grid gap-5 px-4.5 pt-7'>
        <p className='text-muted-foreground text-[10px] font-medium tracking-[0.16em] uppercase'>
          {t('Workspace')}
        </p>
        <Button
          size='lg'
          className='h-10 w-full'
          onClick={props.onNewCreation}
          disabled={props.disabled}
        >
          <HugeiconsIcon icon={Add01Icon} aria-hidden='true' />
          {t('New creation')}
        </Button>
      </div>
      <nav
        className='studio-navigation flex flex-col gap-1.5 px-3'
        aria-label={t('Studio navigation')}
      >
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
          <span
            className='bg-primary ml-auto size-[5px] shrink-0 rounded-full'
            aria-hidden='true'
          />
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
      <div className='mt-auto grid gap-5 px-4.5 pt-6 pb-5'>
        <div className='studio-note grid gap-2.5 rounded-[10px] border px-3.5 py-4'>
          <HugeiconsIcon
            icon={SparklesIcon}
            className='text-primary size-5'
            aria-hidden='true'
          />
          <p className='text-xs leading-[1.6] font-medium'>
            {t('A space for your next idea.')}
          </p>
          <span className='text-muted-foreground text-[11px] leading-[1.7]'>
            {t('Your API key. Your models. Your imagination.')}
          </span>
        </div>
        <Separator />
        <div className='flex min-w-0 items-center gap-2.5'>
          <span
            className='bg-muted text-primary grid size-8 shrink-0 place-items-center rounded-full border text-xs'
            aria-hidden='true'
          >
            {username?.slice(0, 1).toUpperCase() || 'N'}
          </span>
          <div className='min-w-0'>
            <p className='truncate text-xs font-medium'>
              {username || t('Guest')}
            </p>
            <span className='text-muted-foreground text-[10px]'>
              {t('Personal workspace')}
            </span>
          </div>
        </div>
      </div>
    </aside>
  )
}
