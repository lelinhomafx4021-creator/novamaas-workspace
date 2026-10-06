import { Text, View } from '@tarojs/components'
import type { PropsWithChildren, ReactNode } from 'react'

import './page-shell.scss'

interface PageShellProps extends PropsWithChildren {
  compact?: boolean
  description: string
  eyebrow?: string
  title: string
  trailing?: ReactNode
}

export function PageShell(props: PageShellProps) {
  return (
    <View
      className={
        props.compact ? 'page-shell page-shell--compact' : 'page-shell'
      }
    >
      <View className='page-shell__glow page-shell__glow--one' />
      <View className='page-shell__glow page-shell__glow--two' />
      <View className='page-shell__header'>
        {props.eyebrow ? (
          <Text className='page-shell__eyebrow'>{props.eyebrow}</Text>
        ) : null}
        <View className='page-shell__title-row'>
          <Text className='page-shell__title'>{props.title}</Text>
          {props.trailing}
        </View>
        <Text className='page-shell__description'>{props.description}</Text>
      </View>
      <View className='page-shell__content'>{props.children}</View>
    </View>
  )
}
