import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { SecureVerificationDialog } from '../components/secure-verification-dialog'

function showDialog(options: { has2FA: boolean; code?: string }) {
  const verify = vi.fn()
  const cancel = vi.fn()
  render(
    <SecureVerificationDialog
      open
      onOpenChange={vi.fn()}
      methods={{
        has2FA: options.has2FA,
        hasPasskey: true,
        passkeySupported: false,
      }}
      state={{
        method: options.has2FA ? '2fa' : null,
        loading: false,
        code: options.code ?? '',
      }}
      onVerify={verify}
      onCancel={cancel}
      onCodeChange={vi.fn()}
      onMethodChange={vi.fn()}
    />
  )
  return { verify, cancel }
}

test('when no supported security method is available verification stays disabled', () => {
  showDialog({ has2FA: false })
  expect(screen.getByRole('button', { name: 'Verify' })).toBeDisabled()
  expect(screen.queryByRole('tab', { name: 'Passkey' })).not.toBeInTheDocument()
})

test('an alphanumeric backup code can be submitted with Enter in the shared window', () => {
  const { verify } = showDialog({ has2FA: true, code: 'ABCD1234' })
  const code = screen.getByRole('textbox', { name: 'Authenticator code' })
  expect(code).toHaveAttribute('inputmode', 'text')
  expect(screen.queryByRole('tab', { name: 'Passkey' })).not.toBeInTheDocument()
  fireEvent.keyDown(code, { key: 'Enter' })
  expect(verify).toHaveBeenCalledWith('2fa', 'ABCD1234')
})

test('canceling a shared security check calls cancellation without verification', async () => {
  const { verify, cancel } = showDialog({ has2FA: true })
  await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(cancel).toHaveBeenCalledOnce()
  expect(verify).not.toHaveBeenCalled()
})
