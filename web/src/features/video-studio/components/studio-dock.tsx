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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet'

/** Below this width the side panels no longer fit next to a usable canvas. */
export const STUDIO_DOCK_QUERY = '(min-width: 1024px)'

type StudioDockProps = {
  docked: boolean
  side: 'left' | 'right'
  title: string
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
}

/**
 * Renders a side panel docked next to the canvas on wide screens and inside
 * a slide-over sheet on narrow ones, so the panels themselves stay unaware
 * of the breakpoint.
 */
export function StudioDock(props: StudioDockProps) {
  const { t } = useTranslation()
  if (props.docked) return props.children
  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side={props.side}
        className='studio-surface w-[min(22rem,92vw)] gap-0 p-0'
        aria-label={t(props.title)}
      >
        <SheetTitle className='sr-only'>{t(props.title)}</SheetTitle>
        <SheetDescription className='sr-only'>
          {t(props.title)}
        </SheetDescription>
        {/* Leaves room for the sheet's own close button. */}
        <div className='flex min-h-0 flex-1 pt-10 [&>aside]:w-full [&>aside]:border-0'>
          {props.children}
        </div>
      </SheetContent>
    </Sheet>
  )
}
