import Taro from '@tarojs/taro'

import { apiRequest, getAuthorizedMiniSession, getConfiguredApiBaseUrl } from './request'
import { buildApiUrl } from './url'

export interface PlaygroundMedia {
  image_input_models: string[]
  voice_input_models: string[]
  voice_output_models: string[]
  transcription_model: string
  speech_model: string
  speech_voice: string
}

export function getPlaygroundMedia(group: string) {
  return apiRequest<PlaygroundMedia>(`/api/user/playground/media?group=${encodeURIComponent(group)}`)
}

export async function transcribePlaygroundAudio(filePath: string, group: string, chatModel: string, transcriptionModel: string) {
  const session = await getAuthorizedMiniSession()
  const response = await Taro.uploadFile({
    url: buildApiUrl(getConfiguredApiBaseUrl(), '/pg/audio/transcriptions'),
    filePath,
    name: 'file',
    formData: { group, chat_model: chatModel, model: transcriptionModel },
    header: { Authorization: `Bearer ${session.accessToken}` },
  })
  if (response.statusCode < 200 || response.statusCode >= 300) throw new Error('TRANSCRIPTION_FAILED')
  const result = JSON.parse(response.data) as { text?: string }
  if (!result.text?.trim()) throw new Error('TRANSCRIPTION_EMPTY')
  return result.text.trim()
}

export async function synthesizePlaygroundSpeech(input: string, group: string, chatModel: string, speechModel: string, voice: string) {
  const session = await getAuthorizedMiniSession()
  const response = await Taro.request<ArrayBuffer>({
    url: buildApiUrl(getConfiguredApiBaseUrl(), '/pg/audio/speech'),
    method: 'POST',
    responseType: 'arraybuffer',
    header: { Authorization: `Bearer ${session.accessToken}`, 'Content-Type': 'application/json' },
    data: { model: speechModel, group, chat_model: chatModel, input, voice, response_format: 'mp3' },
  })
  if (response.statusCode < 200 || response.statusCode >= 300 || !(response.data instanceof ArrayBuffer)) throw new Error('SPEECH_FAILED')
  const path = `${Taro.env.USER_DATA_PATH}/miniapp-speech-${Date.now()}.mp3`
  const fs = Taro.getFileSystemManager()
  await new Promise<void>((resolve, reject) => fs.writeFile({ filePath: path, data: response.data, success: () => resolve(), fail: reject }))
  return path
}

export async function imageDataUrl(filePath: string) {
  const fs = Taro.getFileSystemManager()
  const encoded = await new Promise<string>((resolve, reject) => fs.readFile({ filePath, encoding: 'base64', success: ({ data }) => resolve(String(data)), fail: reject }))
  const mime = encoded.startsWith('/9j/') ? 'image/jpeg' : encoded.startsWith('iVBOR') ? 'image/png' : encoded.startsWith('UklGR') ? 'image/webp' : ''
  if (!mime || encoded.length > 1_398_104) throw new Error('IMAGE_INVALID')
  return `data:${mime};base64,${encoded}`
}
