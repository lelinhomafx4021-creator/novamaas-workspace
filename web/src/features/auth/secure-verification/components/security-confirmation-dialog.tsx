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
import { ShieldCheck } from 'lucide-react'
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'

/** Shared window for scoped security checks across sensitive account actions. */
export function SecurityConfirmationDialog(
  props: ComponentProps<typeof Dialog>
) {
  const { t } = useTranslation()
  return (
    <Dialog
      {...props}
      title={
        <>
          <ShieldCheck className='text-primary h-5 w-5' />
          {props.title}
        </>
      }
      description={
        props.description ??
        t('Confirm your identity before accessing this sensitive action.')
      }
      contentClassName='top-[8vh] max-w-[calc(100%-1.5rem)] translate-y-0 overflow-hidden border-none shadow-xl sm:top-1/2 sm:max-w-md sm:translate-y-[-50%] sm:rounded-xl'
      headerClassName='border-b pb-4 text-left'
      titleClassName='flex items-center gap-2 text-lg font-semibold'
      descriptionClassName='text-left'
      contentHeight='auto'
      bodyClassName='px-1 py-1'
      footerClassName='bg-muted/30 border-t px-6 py-4 sm:flex-row sm:justify-end'
    />
  )
}
