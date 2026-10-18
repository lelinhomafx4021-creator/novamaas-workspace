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
import { useState, type Dispatch, type SetStateAction } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { MediaAssetList } from '../../assets/types'
import {
  previewVideoMaterial,
  queryVideoMaterials,
  uploadVideoMaterial,
} from '../api'
import type { VideoForm, VideoMaterial } from '../types'

export function VideoMaterialPanel(props: {
  video: VideoForm
  busy: boolean
  baseUrl: string
  apiKey: string
  onChange: Dispatch<SetStateAction<VideoForm>>
}) {
  const { t } = useTranslation()
  const [url, setUrl] = useState('')
  const [search, setSearch] = useState('')
  const [page, setPage] = useState(1)
  const [library, setLibrary] = useState<MediaAssetList | null>(null)
  const [preview, setPreview] = useState<{ name: string; url: string } | null>(
    null
  )
  const [loading, setLoading] = useState(false)
  const [uploadToLibrary, setUploadToLibrary] = useState(false)
  const materials = props.video.materials
  const scenario =
    props.video.scenario || (materials.length ? 'reference' : 'text')
  let nextRole = 'reference_image'
  if (scenario === 'first_frame' || scenario === 'first_last_frame') {
    nextRole = materials.length ? 'last_frame' : 'first_frame'
  }
  const add = (next: VideoMaterial[], preparationIds: string[] = []) =>
    props.onChange((current) => ({
      ...current,
      scenario:
        current.scenario === 'text' || !current.scenario
          ? 'reference'
          : current.scenario,
      materials: [...current.materials, ...next],
      preparationRunIds: [
        ...(current.preparationRunIds || []),
        ...preparationIds,
      ],
    }))
  const query = async (nextPage: number) => {
    setLoading(true)
    try {
      setLibrary(
        await queryVideoMaterials(props.baseUrl, props.apiKey, search, nextPage)
      )
      setPage(nextPage)
    } catch {
      toast.error(t('Unable to query materials'))
    } finally {
      setLoading(false)
    }
  }
  const readFiles = async (files: FileList | null) => {
    const selectedFiles = [...(files || [])]
    if (!selectedFiles.length) return
    setLoading(true)
    const next: VideoMaterial[] = []
    const prepared: string[] = []
    try {
      for (const file of selectedFiles) {
        if (!file.type.startsWith('image/') || file.size > 30 * 1024 * 1024) {
          throw new Error('Invalid image')
        }
        let role = 'reference_image'
        if (scenario === 'first_frame' || scenario === 'first_last_frame') {
          role =
            materials.length + next.length === 0 ? 'first_frame' : 'last_frame'
        }
        if (uploadToLibrary) {
          const form = new FormData()
          form.set('file', file)
          form.set('base_url', props.baseUrl)
          form.set('api_key', props.apiKey)
          const result = await uploadVideoMaterial(form)
          next.push({
            id: crypto.randomUUID(),
            source: 'asset',
            role,
            url: `asset://${result.asset.id}`,
            name: result.asset.name,
          })
          prepared.push(result.preparation_run_id)
        } else {
          const data = await new Promise<string>((resolve, reject) => {
            const reader = new FileReader()
            reader.addEventListener('load', () =>
              typeof reader.result === 'string'
                ? resolve(reader.result)
                : reject(new Error('Invalid image'))
            )
            reader.addEventListener('error', reject)
            reader.readAsDataURL(file)
          })
          next.push({
            id: crypto.randomUUID(),
            source: 'base64',
            role,
            url: data,
            name: file.name,
          })
        }
      }
    } catch {
      toast.error(t('Unable to load images; use image files up to 30 MB'))
    } finally {
      if (next.length) add(next, prepared)
      setLoading(false)
    }
  }
  return (
    <Card className='space-y-3 p-3'>
      <Label htmlFor='video-scenario'>{t('Generation scenario')}</Label>
      <Select
        value={scenario}
        disabled={props.busy || loading}
        onValueChange={(value) =>
          props.onChange((current) => ({
            ...current,
            scenario: value as VideoForm['scenario'],
          }))
        }
      >
        <SelectTrigger id='video-scenario'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value='text'>{t('Text-to-Video')}</SelectItem>
          <SelectItem value='first_frame'>{t('First frame')}</SelectItem>
          <SelectItem value='first_last_frame'>
            {t('First and last frames')}
          </SelectItem>
          <SelectItem value='reference'>
            {t('Multiple reference images')}
          </SelectItem>
        </SelectContent>
      </Select>
      <div className='flex items-center gap-2'>
        <Checkbox
          id='video-platform-mode'
          checked={Boolean(props.video.platformMode)}
          disabled={props.busy}
          onCheckedChange={(checked) =>
            props.onChange((current) => ({
              ...current,
              platformMode: Boolean(checked),
            }))
          }
        />
        <Label htmlFor='video-platform-mode'>
          {t('Platform integration test')}
        </Label>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Platform materials are queried with the video generation token. Channel replica readiness is checked during generation.'
        )}
      </p>
      <div className='flex gap-2'>
        <Input
          aria-label={t('Material URL or asset ID')}
          placeholder='https://… / asset://…'
          value={url}
          disabled={props.busy}
          onChange={(event) => setUrl(event.target.value)}
        />
        <Button
          variant='outline'
          disabled={props.busy || !url.trim()}
          onClick={() => {
            add([
              {
                id: crypto.randomUUID(),
                source: url.trim().startsWith('asset://') ? 'asset' : 'url',
                url: url.trim(),
                role: nextRole,
              },
            ])
            setUrl('')
          }}
        >
          {t('Add material')}
        </Button>
      </div>
      {props.video.platformMode && (
        <div className='space-y-2'>
          <div className='flex items-center gap-2'>
            <Checkbox
              id='upload-video-material-library'
              checked={uploadToLibrary}
              onCheckedChange={(checked) =>
                setUploadToLibrary(Boolean(checked))
              }
              disabled={props.busy || loading}
            />
            <Label htmlFor='upload-video-material-library'>
              {t('Upload new images to material library')}
            </Label>
          </div>
          <p className='text-muted-foreground text-xs'>
            {t('New uploads use the default material group automatically.')}
          </p>
          <div className='flex gap-2'>
            <Input
              aria-label={t('Search materials')}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              disabled={props.busy || loading}
            />
            <Button
              variant='outline'
              disabled={
                props.busy || loading || !props.baseUrl || !props.apiKey
              }
              onClick={() => void query(1)}
            >
              {t('Search materials')}
            </Button>
          </div>
          {library && (
            <div className='space-y-2 rounded border p-2'>
              {!library.items.length && <p>{t('No input materials')}</p>}
              {library.items
                .filter((asset) => asset.type === 'image')
                .map((asset) => (
                  <div
                    key={asset.id}
                    className='flex items-center justify-between gap-2 text-xs'
                  >
                    <span className='min-w-0 truncate'>
                      {asset.name} · {asset.status}
                    </span>
                    <Button
                      size='sm'
                      variant='ghost'
                      disabled={props.busy || loading}
                      onClick={async () => {
                        setLoading(true)
                        try {
                          const result = await previewVideoMaterial(
                            props.baseUrl,
                            props.apiKey,
                            asset.id
                          )
                          setPreview({ name: asset.name, url: result.url })
                        } catch {
                          toast.error(t('Preview unavailable'))
                        } finally {
                          setLoading(false)
                        }
                      }}
                    >
                      {t('Preview')}
                    </Button>
                    <Button
                      size='sm'
                      variant='outline'
                      disabled={
                        props.busy ||
                        !['ready', 'processing'].includes(asset.status) ||
                        materials.some(
                          (item) => item.url === `asset://${asset.id}`
                        )
                      }
                      onClick={() =>
                        add([
                          {
                            id: crypto.randomUUID(),
                            source: 'asset',
                            url: `asset://${asset.id}`,
                            role: nextRole,
                            name: asset.name,
                          },
                        ])
                      }
                    >
                      {t('Add material')}
                    </Button>
                  </div>
                ))}
              <div className='flex justify-between'>
                <Button
                  variant='ghost'
                  size='sm'
                  disabled={loading || page <= 1}
                  onClick={() => void query(page - 1)}
                >
                  {t('Previous')}
                </Button>
                <span>{page}</span>
                <Button
                  variant='ghost'
                  size='sm'
                  disabled={
                    loading || page * library.page_size >= library.total
                  }
                  onClick={() => void query(page + 1)}
                >
                  {t('Next')}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}
      <Label htmlFor='video-material-files'>
        {loading ? t('Loading…') : t('Select images')}
      </Label>
      <Input
        id='video-material-files'
        type='file'
        multiple
        accept='image/*'
        disabled={props.busy || loading}
        onChange={(event) => {
          void readFiles(event.target.files)
          event.target.value = ''
        }}
      />
      {materials.map((material, index) => (
        <div key={material.id} className='space-y-2 rounded border p-2'>
          <div className='flex items-center gap-2'>
            {material.source !== 'asset' && (
              <img
                src={material.url}
                alt={t('Input material {{index}}', { index: index + 1 })}
                className='size-10 rounded object-contain'
                loading='lazy'
              />
            )}
            <span className='min-w-0 flex-1 truncate text-xs'>
              {index + 1}.{' '}
              {material.name ||
                (material.source === 'base64'
                  ? t('Local File (Base64)')
                  : material.url)}
            </span>
          </div>
          <div className='flex flex-wrap gap-2'>
            <Select
              value={material.role}
              disabled={props.busy}
              onValueChange={(role) =>
                props.onChange((current) => ({
                  ...current,
                  materials: materials.map((item, position) =>
                    position === index
                      ? { ...item, role: role || 'reference_image' }
                      : item
                  ),
                }))
              }
            >
              <SelectTrigger className='h-7 w-40'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[
                  ['first_frame', 'First frame'],
                  ['last_frame', 'End Frame'],
                  ['reference_image', 'Reference image'],
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
                variant='outline'
                size='sm'
                disabled={
                  props.busy ||
                  index + direction < 0 ||
                  index + direction >= materials.length
                }
                aria-label={t(
                  direction < 0 ? 'Move material up' : 'Move material down'
                )}
                onClick={() => {
                  const next = [...materials]
                  ;[next[index], next[index + direction]] = [
                    next[index + direction],
                    next[index],
                  ]
                  props.onChange((current) => ({ ...current, materials: next }))
                }}
              >
                {direction < 0 ? '↑' : '↓'}
              </Button>
            ))}
            <Button
              size='sm'
              variant='outline'
              disabled={props.busy}
              aria-label={t('Remove material {{index}}', { index: index + 1 })}
              onClick={() =>
                props.onChange((current) => ({
                  ...current,
                  materials: materials.filter(
                    (_, position) => index !== position
                  ),
                }))
              }
            >
              {t('Remove')}
            </Button>
          </div>
        </div>
      ))}
      <Dialog
        open={Boolean(preview)}
        onOpenChange={(open) => {
          if (!open) setPreview(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{preview?.name || t('Preview')}</DialogTitle>
          </DialogHeader>
          {preview && (
            <img
              src={preview.url}
              alt={preview.name}
              className='max-h-[70vh] w-full object-contain'
            />
          )}
        </DialogContent>
      </Dialog>
    </Card>
  )
}
