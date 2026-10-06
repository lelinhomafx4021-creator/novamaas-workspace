import type { ReactNode } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import type { MiniBindingRequiredData } from '@/auth/client'

import { WeChatConnectionConfirmation } from './wechat-connection'

const nativeEvents = vi.hoisted(() => ({
  nicknameInput: undefined as Record<string, unknown> | undefined,
  confirmButton: undefined as Record<string, unknown> | undefined,
}))

vi.mock('@tarojs/components', async () => {
  const { createElement } = await import('react')
  const nativeElement = (tag: string) => (props: Record<string, unknown>) => {
    if (tag === 'input' && props.type === 'nickname') {
      nativeEvents.nicknameInput = props
    }
    if (tag === 'button' && props.className === 'profile-button') {
      nativeEvents.confirmButton = props
    }
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
vi.mock('@tarojs/taro', () => ({ default: { navigateTo: vi.fn() } }))
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

const binding: MiniBindingRequiredData = {
  binding_required: true,
  confirmation_available: true,
  matched_account: {
    username: 'matched-user',
    display_name: 'Matched Account',
    phone_hint: '+86 138****1234',
  },
  flow_token: 'matched-flow',
  flow_expires_at: 2000,
  password_login_enabled: true,
  user_agreement_enabled: false,
  privacy_policy_enabled: false,
  email_verification_enabled: false,
  registration_enabled: false,
}

describe('WeChat phone connection confirmation', () => {
  it('guides native WeChat avatar and nickname selection with explicit actions', () => {
    const markup = renderToStaticMarkup(
      <WeChatConnectionConfirmation
        binding={binding}
        busy={false}
        errorKey=''
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    expect(markup).toContain('Matched Account')
    expect(markup).toContain('+86 138****1234')
    expect(markup).toContain(
      'data-open-type="chooseAvatar">Choose avatar</button>'
    )
    expect(markup).toContain('Avatar and nickname (optional)')
    expect(markup).toContain('>Use WeChat nickname</button>')
    expect(markup).toContain('type="nickname"')
    expect(markup).toContain('aria-label="WeChat nickname (optional)"')
    expect(markup).toContain('placeholder="Select your WeChat nickname"')
    expect(markup).toContain('above the WeChat keyboard')
    expect(markup).toContain('Confirm and sign in')
    expect(markup).not.toContain('disabled=""')
  })

  it.each(['onBlur', 'onConfirm'])(
    'submits native nickname selection received only through %s',
    (eventName) => {
      const onConfirm = vi.fn()
      renderToStaticMarkup(
        <WeChatConnectionConfirmation
          binding={binding}
          busy={false}
          errorKey=''
          onConfirm={onConfirm}
          onCancel={vi.fn()}
        />
      )
      const receiveNickname = nativeEvents.nicknameInput?.[
        eventName
      ] as (event: { detail: { value: string } }) => void
      const confirm = nativeEvents.confirmButton?.onClick as () => void
      receiveNickname({ detail: { value: '  微信昵称  ' } })
      confirm()
      expect(onConfirm).toHaveBeenCalledWith({
        acceptTerms: false,
        avatarPath: undefined,
        nickname: '微信昵称',
      })
    }
  )

  it('requires no credentials even when older flow metadata requests MFA', () => {
    const markup = renderToStaticMarkup(
      <WeChatConnectionConfirmation
        binding={{
          ...binding,
          confirmation_available: false,
          two_factor_required: true,
          password_login_enabled: false,
        }}
        busy={false}
        errorKey=''
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    expect(markup).not.toContain('type="password"')
    expect(markup).not.toContain('Two-factor or backup code')
    expect(markup).not.toContain('disabled=""')
    expect(markup).toContain('Confirm and sign in')
  })

  it('does not offer confirmation without a matched platform account', () => {
    const markup = renderToStaticMarkup(
      <WeChatConnectionConfirmation
        binding={{ ...binding, matched_account: undefined }}
        busy={false}
        errorKey=''
        onConfirm={vi.fn()}
        onCancel={vi.fn()}
      />
    )
    expect(markup).toBe('')
  })
})
