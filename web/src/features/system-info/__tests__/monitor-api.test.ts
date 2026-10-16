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
import { CanceledError } from 'axios'
import { toast } from 'sonner'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { getResourceMonitor } from '../api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
  useAuthStore.getState().auth.reset('idle')
})

test('canceling a previous monitor observer does not cancel the replacement request or show a global error', async () => {
  useAuthStore.getState().auth.reset('complete')
  const errorToast = vi
    .spyOn(toast, 'error')
    .mockImplementation(() => 'test-toast')
  const firstController = new AbortController()
  const replacementController = new AbortController()
  let acknowledgeFirstRequest: () => void = () => undefined
  const firstStarted = new Promise<void>((resolve) => {
    acknowledgeFirstRequest = resolve
  })
  const payload = { meta: { node_name: 'replacement-node' } }
  api.defaults.adapter = (config) => {
    if (config.signal === firstController.signal) {
      return new Promise((_, reject) => {
        config.signal?.addEventListener?.('abort', () =>
          reject(new CanceledError('canceled', config))
        )
        acknowledgeFirstRequest()
      })
    }
    return Promise.resolve({
      config,
      status: 200,
      statusText: 'OK',
      headers: {},
      data: { success: true, data: payload },
    })
  }

  const first = getResourceMonitor(60, firstController.signal).then(
    (value) => ({ kind: 'success', value }),
    (error: unknown) => ({ kind: 'error', error })
  )
  await firstStarted
  const replacement = getResourceMonitor(60, replacementController.signal).then(
    (value) => ({ kind: 'success', value }),
    (error: unknown) => ({ kind: 'error', error })
  )
  firstController.abort()

  expect(await first).toMatchObject({ kind: 'error' })
  expect(await replacement).toEqual({ kind: 'success', value: payload })
  expect(replacementController.signal.aborted).toBe(false)
  expect(errorToast).not.toHaveBeenCalled()
})
