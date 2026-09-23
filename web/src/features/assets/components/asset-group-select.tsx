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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { AssetGroup } from '../types'

type AssetGroupSelectProps = {
  groups: AssetGroup[]
  value: string
  isAdmin: boolean
  onValueChange: (value: string) => void
}

type AssetGroupOptionLabelProps = {
  group: AssetGroup
  isAdmin: boolean
  selected?: boolean
}

function AssetGroupOptionLabel(props: AssetGroupOptionLabelProps) {
  const { t } = useTranslation()
  const creator = props.group.owner_name || `#${props.group.owner_user_id}`
  const metadata = props.isAdmin
    ? `${t('ID')}: ${props.group.id} · ${t('Creator')}: ${creator}`
    : `${t('ID')}: ${props.group.id}`

  return (
    <span className='flex w-full min-w-0 flex-1 flex-col items-start text-left leading-tight'>
      <span
        className='block w-full truncate font-medium'
        data-testid={props.selected ? 'selected-asset-group-name' : undefined}
      >
        {props.group.name}
      </span>
      <span
        aria-label={metadata}
        className='text-muted-foreground mt-0.5 flex w-full min-w-0 items-baseline gap-1 overflow-hidden text-[11px] leading-4 whitespace-nowrap'
        data-testid={
          props.selected ? 'selected-asset-group-metadata' : undefined
        }
        title={metadata}
      >
        <span className='shrink-0'>{t('ID')}: </span>
        <span
          className='min-w-0 truncate font-mono'
          data-testid={props.selected ? 'selected-asset-group-id' : undefined}
        >
          {props.group.id}
        </span>
        {props.isAdmin && (
          <>
            <span className='shrink-0'> · </span>
            <span className='shrink-0'>{t('Creator')}: </span>
            <span className='max-w-[35%] shrink-0 truncate'>{creator}</span>
          </>
        )}
      </span>
    </span>
  )
}

export function AssetGroupSelect(props: AssetGroupSelectProps) {
  const { t } = useTranslation()
  const items = useMemo(
    () => [
      { value: null, label: t('All groups') },
      ...props.groups.map((group) => ({
        value: group.id,
        label: (
          <AssetGroupOptionLabel
            group={group}
            isAdmin={props.isAdmin}
            selected
          />
        ),
      })),
    ],
    [props.groups, props.isAdmin, t]
  )

  return (
    <Select
      items={items}
      value={props.value || null}
      onValueChange={(value) => props.onValueChange(value ?? '')}
    >
      <SelectTrigger
        aria-label={t('Asset group')}
        className='min-h-12 w-full data-[size=default]:h-auto *:data-[slot=select-value]:line-clamp-none *:data-[slot=select-value]:min-w-0'
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent
        align='start'
        alignItemWithTrigger={false}
        className='sm:min-w-96'
      >
        <SelectGroup>
          <SelectItem value={null} className='py-2'>
            {t('All groups')}
          </SelectItem>
          {props.groups.map((group) => (
            <SelectItem
              key={group.id}
              value={group.id}
              className='items-start py-2'
            >
              <AssetGroupOptionLabel group={group} isAdmin={props.isAdmin} />
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
