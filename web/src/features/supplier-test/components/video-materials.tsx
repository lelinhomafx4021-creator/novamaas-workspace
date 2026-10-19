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
import {
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type SetStateAction,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { api } from '@/lib/api'

import type { VideoForm, VideoExchange } from '../types'
import { buildVideoRequestPayload } from '../video-json'

type ImageInput = {
  type: 'image_url'
  image_url: { url: string }
  role?: string
  [key: string]: unknown
}
type LibraryAsset = { id: string; name: string; status: string; type: string }

export function VideoMaterials(props: {
  video: VideoForm
  busy: boolean
  onChange: Dispatch<SetStateAction<VideoForm>>
  onUsePlatform: () => void
  onBusyChange: (value: boolean) => void
  stopVersion: number
}) {
  const { t } = useTranslation()
  const [url, setUrl] = useState(''),
    [assets, setAssets] = useState<LibraryAsset[] | null>(null),
    [toLibrary, setToLibrary] = useState(false),
    [preview, setPreview] = useState('')
  const imageKeys = useRef(new WeakMap<ImageInput, string>())
  const abort = useRef<AbortController | null>(null)
  useEffect(() => () => abort.current?.abort(), [])
  useEffect(() => {
    abort.current?.abort()
  }, [props.stopVersion])
  const images = (props.video.materials ||
    (
      (buildVideoRequestPayload('', props.video)?.content || []) as Record<
        string,
        unknown
      >[]
    ).filter((item) => item.type === 'image_url')) as ImageInput[]
  const change = (next: ImageInput[]) =>
    props.onChange((current) => ({
      ...current,
      materials: next,
      hasImage: false,
      hasLastFrame: false,
      rawPayload: undefined,
    }))
  // Use the existing authenticated material-library routes. Preserve the raw
  // response alongside its parsed UI data; no parallel material API is introduced.
  const call = async (
    method: 'GET' | 'POST',
    path: string,
    data?: unknown,
    requestBody?: string,
    replacePrevious = false
  ) => {
    const started = performance.now()
    const response = await api.request<string>({
      method,
      url: path,
      data,
      responseType: 'text',
      transformResponse: [(value) => value],
      validateStatus: () => true,
      skipBusinessError: true,
      skipErrorHandler: true,
      signal: abort.current?.signal,
    })
    const headers = Object.entries(response.config.headers || {})
      .map(
        ([key, value]) =>
          `${key}: ${/authorization|cookie|token|key/i.test(key) ? '[REDACTED]' : String(value)}`
      )
      .join('\n')
    let request = requestBody || ''
    if (!request && data && !(data instanceof FormData)) {
      request = JSON.stringify(data, null, 2)
    }
    const exchange: VideoExchange = {
      method,
      url: path,
      headers,
      request,
      response: String(response.data),
      status: response.status,
      request_id: String(response.headers['x-request-id'] || ''),
      elapsed_ms: Math.round(performance.now() - started),
    }
    props.onChange((current) => ({
      ...current,
      materialEvidence: [
        ...(current.materialEvidence || []).filter(
          (item) =>
            !replacePrevious || item.method !== method || item.url !== path
        ),
        exchange,
      ],
    }))
    const body = JSON.parse(exchange.response)
    if (!body.success) throw new Error('Material operation failed')
    return body.data
  }
  const query = async () => {
    abort.current = new AbortController()
    props.onBusyChange(true)
    props.onUsePlatform()
    try {
      setAssets(
        (await call('GET', '/api/asset-library/assets?p=1&page_size=40')).items
      )
    } catch {
      toast.error(t('Material operation failed'))
    } finally {
      props.onBusyChange(false)
      abort.current = null
    }
  }
  const files = async (selected: File[]) => {
    if (!selected.length) return
    abort.current = new AbortController()
    props.onBusyChange(true)
    const added: ImageInput[] = []
    try {
      let groupId = ''
      if (toLibrary) {
        props.onUsePlatform()
        const groups = (await call(
          'GET',
          '/api/asset-library/groups'
        )) as Array<{ id: string; name: string }>
        groupId =
          groups.find((group) =>
            ['default', '默认分组', 'default group'].includes(
              group.name.toLowerCase()
            )
          )?.id || ''
        if (!groupId) {
          groupId = (
            await call('POST', '/api/asset-library/groups', {
              name: 'default',
              description: '',
            })
          ).id
        }
      }
      for (const file of selected) {
        if (!file.type.startsWith('image/') || file.size > 30 * 1024 * 1024) {
          throw new Error('Use image files up to 30 MB')
        }
        let source = ''
        if (toLibrary) {
          const form = new FormData()
          form.set('group_id', groupId)
          form.set('name', file.name)
          form.set('type', 'image')
          form.set('file', file)
          const digest = await crypto.subtle.digest(
            'SHA-256',
            await file.arrayBuffer()
          )
          let asset = (await call(
            'POST',
            '/api/asset-library/assets',
            form,
            JSON.stringify(
              {
                group_id: groupId,
                name: file.name,
                type: 'image',
                file: {
                  name: file.name,
                  bytes: file.size,
                  sha256: Array.from(new Uint8Array(digest), (value) =>
                    value.toString(16).padStart(2, '0')
                  ).join(''),
                },
                encoding: 'multipart/form-data',
              },
              null,
              2
            )
          )) as LibraryAsset
          const deadline = Date.now() + 5 * 60 * 1000
          while (
            asset.status === 'processing' &&
            Date.now() < deadline &&
            !abort.current?.signal.aborted
          ) {
            await new Promise((resolve) => setTimeout(resolve, 2000))
            const result = await call(
              'GET',
              `/api/asset-library/assets?p=1&page_size=40&search=${encodeURIComponent(asset.id)}`,
              undefined,
              undefined,
              true
            )
            asset =
              result.items.find((item: LibraryAsset) => item.id === asset.id) ||
              asset
          }
          if (asset.status !== 'ready') throw new Error('Material is not ready')
          source = `asset://${asset.id}`
        } else {
          source = await new Promise<string>((resolve, reject) => {
            const reader = new FileReader()
            reader.addEventListener(
              'load',
              () => {
                if (typeof reader.result === 'string') resolve(reader.result)
                else reject(new Error('Invalid image'))
              },
              { once: true }
            )
            reader.addEventListener('error', reject, { once: true })
            reader.readAsDataURL(file)
          })
        }
        if (abort.current?.signal.aborted) break
        added.push({
          type: 'image_url',
          image_url: { url: source },
          role: 'reference_image',
        })
      }
    } catch (error) {
      toast.error(
        t(error instanceof Error ? error.message : 'Material operation failed')
      )
    } finally {
      if (added.length) change([...images, ...added])
      props.onBusyChange(false)
      abort.current = null
    }
  }
  return (
    <Card className='space-y-2 p-3'>
      <Label>{t('Input materials')}</Label>
      <div className='flex gap-2'>
        <Input
          aria-label={t('Material URL or asset ID')}
          value={url}
          disabled={props.busy}
          placeholder='https://… / asset://…'
          onChange={(event) => setUrl(event.target.value)}
        />
        <Button
          variant='outline'
          disabled={props.busy || !url.trim()}
          onClick={() => {
            if (url.trim().startsWith('asset://')) props.onUsePlatform()
            change([
              ...images,
              {
                type: 'image_url',
                image_url: { url: url.trim() },
                role: 'reference_image',
              },
            ])
            setUrl('')
          }}
        >
          {t('Add material')}
        </Button>
      </div>
      <Label>
        <Checkbox
          checked={toLibrary}
          disabled={props.busy}
          onCheckedChange={(value) => setToLibrary(Boolean(value))}
        />
        {t('Upload new images to material library')}
      </Label>
      <Input
        type='file'
        multiple
        accept='image/*'
        aria-label={t('Select images')}
        disabled={props.busy}
        onChange={(event) => {
          const selected = [...(event.target.files || [])]
          event.target.value = ''
          void files(selected)
        }}
      />
      <Button
        variant='outline'
        disabled={props.busy}
        onClick={() => void query()}
      >
        {t('Browse platform materials')}
      </Button>
      {assets
        ?.filter((asset) => asset.type === 'image')
        .map((asset) => (
          <div key={asset.id} className='flex items-center gap-2 text-xs'>
            <span className='flex-1'>
              {asset.name} · {asset.status}
            </span>
            <Button
              size='sm'
              variant='ghost'
              disabled={props.busy}
              onClick={async () => {
                abort.current = new AbortController()
                props.onBusyChange(true)
                try {
                  const result = await call(
                    'GET',
                    `/api/asset-library/assets/${encodeURIComponent(asset.id)}/preview?variant=original`
                  )
                  setPreview(result.url)
                } catch {
                  toast.error(t('Material operation failed'))
                } finally {
                  props.onBusyChange(false)
                  abort.current = null
                }
              }}
            >
              {t('Preview')}
            </Button>
            <Button
              size='sm'
              variant='outline'
              disabled={props.busy || asset.status !== 'ready'}
              onClick={() =>
                change([
                  ...images,
                  {
                    type: 'image_url',
                    image_url: { url: `asset://${asset.id}` },
                    role: 'reference_image',
                  },
                ])
              }
            >
              {t('Add material')}
            </Button>
          </div>
        ))}
      {preview && (
        <img
          src={preview}
          alt={t('Preview')}
          className='max-h-60 max-w-full object-contain'
        />
      )}
      {images.map((image, index) => {
        if (!imageKeys.current.has(image)) {
          imageKeys.current.set(image, crypto.randomUUID())
        }
        return (
          <div
            key={imageKeys.current.get(image)}
            className='flex flex-wrap items-center gap-2'
          >
            <span className='min-w-0 flex-1 truncate text-xs'>
              {index + 1}.{' '}
              {image.image_url.url.startsWith('data:')
                ? t('Local File (Base64)')
                : image.image_url.url}
            </span>
            <Select
              value={image.role || 'reference_image'}
              disabled={props.busy}
              onValueChange={(role) =>
                change(
                  images.map((item, position) =>
                    position === index
                      ? { ...item, role: role || 'reference_image' }
                      : item
                  )
                )
              }
            >
              <SelectTrigger className='w-36'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[
                  ['first_frame', 'First Frame'],
                  ['last_frame', 'End Frame'],
                  ['reference_image', 'Reference Image'],
                ].map(([value, label]) => (
                  <SelectItem key={value} value={value}>
                    {t(label)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {[-1, 1].map((direction) => (
              <Button
                key={direction}
                size='sm'
                variant='ghost'
                aria-label={t(
                  direction < 0 ? 'Move material up' : 'Move material down'
                )}
                disabled={
                  props.busy ||
                  index + direction < 0 ||
                  index + direction >= images.length
                }
                onClick={() => {
                  const next = [...images]
                  ;[next[index], next[index + direction]] = [
                    next[index + direction],
                    next[index],
                  ]
                  change(next)
                }}
              >
                {direction < 0 ? '↑' : '↓'}
              </Button>
            ))}
            <Button
              size='sm'
              variant='ghost'
              disabled={props.busy}
              aria-label={t('Remove material {{index}}', { index: index + 1 })}
              onClick={() =>
                change(images.filter((_, position) => position !== index))
              }
            >
              {t('Remove')}
            </Button>
          </div>
        )
      })}
    </Card>
  )
}
