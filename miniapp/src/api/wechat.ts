/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import Taro from '@tarojs/taro'

import {
  ApiRequestError,
  apiRequest,
  getAuthorizedMiniSession,
  getConfiguredApiBaseUrl,
} from './request'
import { buildApiUrl } from './url'

export interface WeChatBinding {
  app_id: string
  nickname: string
  has_avatar: boolean
  bound_at: string
  last_login_at?: string
  updated_at: string
}

export function getWeChatBindings() {
  return apiRequest<WeChatBinding[]>('/api/user/self/wechat-miniapp')
}

export async function getWeChatAvatar(appId: string) {
  const session = await getAuthorizedMiniSession()
  const result = await Taro.downloadFile({
    url: buildApiUrl(
      getConfiguredApiBaseUrl(),
      `/api/user/self/wechat-miniapp/avatar?app_id=${encodeURIComponent(appId)}`
    ),
    header: { Authorization: `Bearer ${session.accessToken}` },
  })
  if (result.statusCode !== 200) throw new Error('Avatar unavailable')
  return result.tempFilePath
}

export async function readWeChatAvatar(avatarPath: string) {
  const info = await Taro.getFileInfo({ filePath: avatarPath })
  if (!('size' in info) || info.size > 2 * 1024 * 1024) {
    throw new ApiRequestError(
      'Avatar too large',
      400,
      'MINI_AUTH_PROFILE_INVALID'
    )
  }
  return new Promise<string>((resolve, reject) => {
    Taro.getFileSystemManager().readFile({
      filePath: avatarPath,
      encoding: 'base64',
      success: (result) => resolve(result.data as string),
      fail: reject,
    })
  })
}

export async function updateWeChatProfile(
  appId: string,
  nickname: string,
  avatarPath?: string
) {
  const avatarBase64 = avatarPath
    ? await readWeChatAvatar(avatarPath)
    : undefined
  return apiRequest('/api/user/self/wechat-miniapp/profile', {
    method: 'POST',
    data: { app_id: appId, nickname, avatar_base64: avatarBase64 },
  })
}

export function unlinkWeChat(appId: string, proof: string) {
  return apiRequest('/api/user/self/wechat-miniapp/unbind', {
    retryAuth: false,
    method: 'POST',
    data: { app_id: appId },
    header: { 'X-Security-Proof': proof },
  })
}

export function sendWeChatSecuritySMS() {
  return apiRequest<{ challenge_token: string }>(
    '/api/user/self/phone/security/code',
    { method: 'POST' }
  )
}

export async function verifyWeChatSecurity(input: {
  password?: string
  two_factor_code?: string
  challenge_token?: string
  code?: string
}) {
  const result = await apiRequest<{ proof_token: string }>(
    '/api/user/self/phone/security/verify',
    { method: 'POST', data: { ...input, scope: 'wechat.manage' } }
  )
  return result.proof_token
}
