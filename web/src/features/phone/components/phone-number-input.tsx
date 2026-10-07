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
import type { ComponentProps } from 'react'
import { useTranslation } from 'react-i18next'

import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  InputGroupText,
} from '@/components/ui/input-group'

export function PhoneNumberInput(
  props: ComponentProps<'input'> & { groupClassName?: string }
) {
  const { t } = useTranslation()
  const { groupClassName, ...inputProps } = props
  return (
    <InputGroup className={groupClassName}>
      <InputGroupAddon
        className='border-r pr-3'
        aria-label={t('Country calling code')}
      >
        <InputGroupText>+86</InputGroupText>
      </InputGroupAddon>
      <InputGroupInput
        {...inputProps}
        type='tel'
        inputMode='numeric'
        autoComplete='tel-national'
      />
    </InputGroup>
  )
}
