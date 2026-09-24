import { describe, expect, it } from 'vitest'

import { isAccountDeletionConfirmed, parseAccountLanguage } from './preferences'

describe('account preferences', () => {
  it('accepts only supported server language preferences', () => {
    expect(parseAccountLanguage('{"language":"zh-TW"}')).toBe('zh-TW')
    expect(parseAccountLanguage('{"language":"de"}')).toBeNull()
    expect(parseAccountLanguage('invalid')).toBeNull()
  })

  it('requires an exact non-empty username before account deletion', () => {
    expect(isAccountDeletionConfirmed('alice', 'alice')).toBe(true)
    expect(isAccountDeletionConfirmed('Alice', 'alice')).toBe(false)
    expect(isAccountDeletionConfirmed('', '')).toBe(false)
  })
})
