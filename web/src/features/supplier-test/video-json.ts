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
import type { VideoForm } from './types'

export function buildVideoRequestPayload(
  model: string,
  video: VideoForm
): Record<string, unknown> | null {
  const content: Array<Record<string, unknown>> = []
  const prompt = video.prompt.trim()
  if (prompt) {
    content.push({
      ...video.preservedContent?.find((item) => item.type === 'text'),
      type: 'text',
      text: prompt,
    })
  }

  if (video.materials) {
    for (const material of video.materials) {
      if (!material.url.trim()) continue
      const original = material.payload || {}
      const image =
        original.image_url && typeof original.image_url === 'object'
          ? (original.image_url as Record<string, unknown>)
          : {}
      content.push({
        ...original,
        type: 'image_url',
        image_url: { ...image, url: material.url.trim() },
        role: material.role,
      })
    }
    for (const item of video.preservedContent || []) {
      if (
        item.type !== 'text' ||
        item !== video.preservedContent?.find((entry) => entry.type === 'text')
      ) {
        content.push(item)
      }
    }
  }
  if (!video.materials && video.hasImage) {
    const role = video.role || 'reference_image'
    if (video.uploadMode === 'url' && video.imageUrl.trim()) {
      content.push({
        type: 'image_url',
        image_url: { url: video.imageUrl.trim() },
        role,
      })
    } else if (video.uploadMode === 'base64' && video.base64Data.trim()) {
      content.push({
        type: 'image_url',
        image_url: { url: video.base64Data.trim() },
        role,
      })
    }
  }

  if (!video.materials && video.hasLastFrame) {
    if (video.lastFrameMode === 'url' && video.lastFrameUrl.trim()) {
      content.push({
        type: 'image_url',
        image_url: { url: video.lastFrameUrl.trim() },
        role: 'last_frame',
      })
    } else if (
      video.lastFrameMode === 'base64' &&
      video.lastFrameBase64.trim()
    ) {
      content.push({
        type: 'image_url',
        image_url: { url: video.lastFrameBase64.trim() },
        role: 'last_frame',
      })
    }
  }

  if (video.contentOrder && video.materials) {
    const images = content.filter((item) => item.type === 'image_url')
    const ordered: Array<Record<string, unknown>> = []
    let imageIndex = 0
    let textSeen = false
    for (const entry of video.contentOrder) {
      if ('material' in entry) {
        const image = images[imageIndex++]
        if (image) ordered.push(image)
      } else if (entry.value.type === 'text' && !textSeen) {
        textSeen = true
        if (prompt) ordered.push({ ...entry.value, text: prompt })
      } else ordered.push(entry.value)
    }
    if (!textSeen && prompt) ordered.unshift({ type: 'text', text: prompt })
    ordered.push(...images.slice(imageIndex))
    content.splice(0, content.length, ...ordered)
  }

  const payload: Record<string, unknown> = {}
  if (content.length > 0) payload.content = content

  if (video.hasResolution && video.resolution) {
    payload.resolution = video.resolution
  }
  if (video.hasRatio && video.ratio) {
    payload.ratio = video.ratio
  }
  if (video.hasDuration && Number.isFinite(video.duration)) {
    payload.duration = video.duration
  }
  if (video.hasWatermark) {
    payload.watermark = video.watermark
  }
  if (video.hasSeed && video.seed.trim() !== '') {
    const parsed = Number.parseInt(video.seed.trim(), 10)
    if (!Number.isNaN(parsed)) {
      payload.seed = parsed
    }
  }
  if (video.hasGenerateAudio) {
    payload.generate_audio = video.generateAudio
  }
  if (video.hasReturnLastFrame) {
    payload.return_last_frame = video.returnLastFrame
  }

  if (video.hasCustomJson && video.customJson.trim()) {
    try {
      const extra = JSON.parse(video.customJson.trim())
      if (extra && typeof extra === 'object' && !Array.isArray(extra)) {
        Object.assign(payload, extra)
      }
    } catch {
      // Ignore invalid extra JSON in build preview
    }
  }

  if (Object.keys(payload).length === 0) return null
  const trimmedModel = model.trim()
  if (trimmedModel) payload.model = trimmedModel
  return payload
}

export type ParseVideoPayloadResult = {
  success: boolean
  error?: string
  videoPatch?: Partial<VideoForm>
  model?: string
}

export function parseVideoPayloadToForm(raw: string): ParseVideoPayloadResult {
  const trimmed = raw.trim()
  if (!trimmed) {
    return { success: false, error: 'Empty JSON string' }
  }

  let obj: Record<string, unknown>
  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { success: false, error: 'Root payload must be a JSON object' }
    }
    obj = parsed as Record<string, unknown>
  } catch (err) {
    return {
      success: false,
      error: err instanceof Error ? err.message : 'Invalid JSON syntax',
    }
  }

  const patch: Partial<VideoForm> = {
    prompt: '',
    materials: [],
    contentOrder: [],
    preservedContent: [],
    hasImage: false,
    hasLastFrame: false,
    hasResolution: false,
    hasRatio: false,
    hasDuration: false,
    hasWatermark: false,
    hasSeed: false,
    hasGenerateAudio: false,
    hasReturnLastFrame: false,
    hasCustomJson: false,
    customJson: '',
  }
  let extractedModel: string | undefined

  if (typeof obj.model === 'string' && obj.model.trim()) {
    extractedModel = obj.model.trim()
  }

  const handledKeys = new Set([
    'model',
    'content',
    'resolution',
    'ratio',
    'duration',
    'watermark',
    'seed',
    'generate_audio',
    'return_last_frame',
  ])

  if (Array.isArray(obj.content)) {
    for (const [contentIndex, item] of obj.content.entries()) {
      if (!item || typeof item !== 'object') continue
      const itemType = (item as Record<string, unknown>).type
      if (itemType === 'image_url') patch.contentOrder?.push({ material: true })
      else patch.contentOrder?.push({ value: item as Record<string, unknown> })
      if (
        itemType === 'text' &&
        typeof (item as Record<string, unknown>).text === 'string'
      ) {
        if (!patch.preservedContent?.some((entry) => entry.type === 'text')) {
          patch.prompt = String((item as Record<string, unknown>).text)
        }
        patch.preservedContent?.push(item as Record<string, unknown>)
      } else if (itemType === 'image_url') {
        const imgObj = (item as Record<string, unknown>).image_url as
          | Record<string, unknown>
          | undefined
        const urlStr = typeof imgObj?.url === 'string' ? imgObj.url : ''
        const roleStr =
          typeof (item as Record<string, unknown>).role === 'string'
            ? String((item as Record<string, unknown>).role)
            : ''

        let source: 'base64' | 'asset' | 'url' = 'url'
        if (urlStr.startsWith('data:')) source = 'base64'
        else if (urlStr.startsWith('asset://')) source = 'asset'
        patch.materials?.push({
          id: `native-${contentIndex}`,
          source,
          url: urlStr,
          role: roleStr || 'reference_image',
          payload: item as Record<string, unknown>,
        })
        if (roleStr === 'last_frame') {
          patch.hasLastFrame = true
          if (urlStr.startsWith('data:image/')) {
            patch.lastFrameMode = 'base64'
            patch.lastFrameBase64 = urlStr
          } else {
            patch.lastFrameMode = 'url'
            patch.lastFrameUrl = urlStr
          }
        } else {
          patch.hasImage = true
          patch.role = roleStr || 'reference_image'
          if (urlStr.startsWith('data:image/')) {
            patch.uploadMode = 'base64'
            patch.base64Data = urlStr
          } else {
            patch.uploadMode = 'url'
            patch.imageUrl = urlStr
          }
        }
      } else {
        patch.preservedContent?.push(item as Record<string, unknown>)
      }
    }
  }

  if (typeof obj.resolution === 'string' && obj.resolution.trim()) {
    patch.hasResolution = true
    patch.resolution = obj.resolution.trim()
  }

  if (typeof obj.ratio === 'string' && obj.ratio.trim()) {
    patch.hasRatio = true
    patch.ratio = obj.ratio.trim()
  }

  if (typeof obj.duration === 'number' && Number.isFinite(obj.duration)) {
    patch.hasDuration = true
    patch.duration = obj.duration
  }

  if (typeof obj.watermark === 'boolean') {
    patch.hasWatermark = true
    patch.watermark = obj.watermark
  }

  if (obj.seed !== undefined && obj.seed !== null) {
    patch.hasSeed = true
    patch.seed = String(obj.seed)
  }

  if (typeof obj.generate_audio === 'boolean') {
    patch.hasGenerateAudio = true
    patch.generateAudio = obj.generate_audio
  }

  if (typeof obj.return_last_frame === 'boolean') {
    patch.hasReturnLastFrame = true
    patch.returnLastFrame = obj.return_last_frame
  }

  const extra: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(obj)) {
    if (!handledKeys.has(k)) {
      extra[k] = v
    }
  }

  if (Object.keys(extra).length > 0) {
    patch.hasCustomJson = true
    patch.customJson = JSON.stringify(extra, null, 2)
  }

  return {
    success: true,
    videoPatch: patch,
    model: extractedModel,
  }
}
