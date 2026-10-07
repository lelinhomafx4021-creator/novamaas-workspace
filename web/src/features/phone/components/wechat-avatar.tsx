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
import { useEffect, useState } from 'react'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { api } from '@/lib/api'

export function WeChatAvatar(props: {
  binding: {
    app_id: string
    nickname: string
    has_avatar: boolean
    updated_at?: string
  }
  base: string
}) {
  const [url, setUrl] = useState('')
  useEffect(() => {
    setUrl('')
    if (!props.binding.has_avatar) return
    let cancelled = false
    let objectUrl = ''
    void api
      .get<Blob>(
        `${props.base}/avatar?app_id=${encodeURIComponent(props.binding.app_id)}`,
        { responseType: 'blob' }
      )
      .then((response) => {
        if (!cancelled) {
          objectUrl = URL.createObjectURL(response.data)
          setUrl(objectUrl)
        }
      })
      .catch(() => {
        if (!cancelled) setUrl('')
      })
    return () => {
      cancelled = true
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [
    props.binding.app_id,
    props.binding.has_avatar,
    props.binding.updated_at,
    props.base,
  ])
  return (
    <Avatar>
      <AvatarImage src={url} alt={props.binding.nickname} />
      <AvatarFallback>
        {props.binding.nickname.slice(0, 2) || 'WX'}
      </AvatarFallback>
    </Avatar>
  )
}
