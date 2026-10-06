import type { ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { WeChatBindingEntry } from './wechat-binding'
vi.mock('@tarojs/components', async () => {
  const { createElement } = await import('react')
  const nativeElement = (tag: string) => (props: Record<string, unknown>) => {
    return createElement(
      tag,
      {
        className: props.className,
        disabled: props.disabled,
        placeholder: props.placeholder,
        type: props.type,
        defaultValue: props.value,
        'aria-label': props.ariaLabel,
        'data-open-type': props.openType,
      },
      props.children as ReactNode
    )
  }
  return {
    Button: nativeElement('button'),
    Checkbox: nativeElement('input'),
    Image: nativeElement('img'),
    Input: nativeElement('input'),
    Text: nativeElement('span'),
    View: nativeElement('div'),
  }
})
vi.mock('@tarojs/taro', () => ({
  default: { navigateTo: vi.fn() },
  useDidShow: vi.fn(),
}))
vi.mock('react-i18next', async () => {
  const { resources } = await import('../i18n/resources')
  return {
    useTranslation: () => ({
      t: (key: string) =>
        resources.en.translation[
          key as keyof typeof resources.en.translation
        ] ?? key,
    }),
  }
})

const binding = {
  app_id: 'wx-test',
  nickname: 'Selected WeChat name',
  has_avatar: false,
  bound_at: '2026-10-06T00:00:00Z',
  updated_at: '2026-10-06T00:00:00Z',
}
describe('WeChat binding profile overview', () => {
  it('shows the bound identity with edit and unlink actions without an expanded form', () => {
    const markup = renderToStaticMarkup(
      <WeChatBindingEntry binding={binding} editable onReload={vi.fn()} />
    )
    expect(markup).toContain('Selected WeChat name')
    expect(markup).toContain('WeChat connected')
    expect(markup).toContain('Edit profile')
    expect(markup).toContain('Disconnect WeChat')
    expect(markup).not.toContain('<input')
    expect(markup).not.toContain('Save profile')
    expect(markup).not.toContain('Use WeChat nickname')
  })
  it('keeps the non-editable binding summary read-only and shows a profile fallback', () => {
    const markup = renderToStaticMarkup(
      <WeChatBindingEntry
        binding={{ ...binding, nickname: '' }}
        onReload={vi.fn()}
      />
    )
    expect(markup).toContain('Nickname and avatar not provided')
    expect(markup).not.toContain('<button')
    expect(markup).not.toContain('<input')
  })
})
