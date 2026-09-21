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
  Clock,
  Download,
  ExternalLink,
  Film,
  Image as ImageIcon,
  Layers,
  Network,
  Upload,
  Video,
} from 'lucide-react'
import {
  useRef,
  useState,
  type ChangeEvent,
  type Dispatch,
  type SetStateAction,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import {
  DEFAULT_VIDEO_PROMPT,
  ENDPOINT_PATH_PRESETS,
  VIDEO_RATIOS,
  VIDEO_RESOLUTIONS,
  VIDEO_ROLES,
} from '../constants'
import type { CheckResult, VideoForm, VideoMetrics } from '../types'
import { CheckTable } from './check-table'
import { RawJsonDialog } from './raw-json-dialog'

export function VideoPanel(props: {
  video: VideoForm
  busy: boolean
  videoChecks: CheckResult[]
  videoMetrics: VideoMetrics | null
  onVideoChange: Dispatch<SetStateAction<VideoForm>>
  onExportPdf?: () => void
}) {
  const { t } = useTranslation()
  const firstFileInputRef = useRef<HTMLInputElement>(null)
  const lastFileInputRef = useRef<HTMLInputElement>(null)
  const [showBase64Textarea, setShowBase64Textarea] = useState(false)

  const handleFirstFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error(t('Please select an image file'))
      return
    }
    const reader = new FileReader()
    reader.addEventListener('load', (event) => {
      const result = event.target?.result
      if (typeof result === 'string') {
        props.onVideoChange((current) => ({
          ...current,
          base64Data: result,
        }))
        toast.success(
          t('Image loaded as Base64 ({{size}} KB)', {
            size: Math.round(file.size / 1024),
          })
        )
      }
    })
    reader.addEventListener('error', () => {
      toast.error(t('Failed to read image file'))
    })
    reader.readAsDataURL(file)
  }

  const handleLastFileChange = (e: ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) {
      toast.error(t('Please select an image file'))
      return
    }
    const reader = new FileReader()
    reader.addEventListener('load', (event) => {
      const result = event.target?.result
      if (typeof result === 'string') {
        props.onVideoChange((current) => ({
          ...current,
          lastFrameBase64: result,
        }))
        toast.success(
          t('End frame image loaded ({{size}} KB)', {
            size: Math.round(file.size / 1024),
          })
        )
      }
    })
    reader.addEventListener('error', () => {
      toast.error(t('Failed to read image file'))
    })
    reader.readAsDataURL(file)
  }

  const statusVariant = (status?: string) => {
    const s = (status || '').toLowerCase()
    if (s === 'succeeded' || s === 'success') {
      return 'default'
    }
    if (s === 'failed' || s === 'failure') {
      return 'destructive'
    }
    if (s === 'running' || s === 'queued' || s === 'processing') {
      return 'secondary'
    }
    return 'outline'
  }

  return (
    <TabsContent value='video' className='space-y-5'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Doubao Seedance video generation test. All non-prompt fields are strictly optional: unchecked fields will NOT be sent upstream.'
          )}
        </p>
        <div className='flex items-center gap-2'>
          <RawJsonDialog videoMetrics={props.videoMetrics} />
        </div>
      </div>

      {/* Prompt Configuration */}
      <div className='space-y-2'>
        <div className='flex items-center justify-between'>
          <Label htmlFor='video-prompt' className='font-medium'>
            {t('Prompt (required)')}
          </Label>
          <Button
            variant='ghost'
            size='xs'
            disabled={props.busy}
            onClick={() =>
              props.onVideoChange((cur) => ({
                ...cur,
                prompt: DEFAULT_VIDEO_PROMPT,
              }))
            }
            className='h-6 text-xs'
          >
            {t('Use default prompt')}
          </Button>
        </div>
        <Textarea
          id='video-prompt'
          rows={3}
          value={props.video.prompt}
          disabled={props.busy}
          placeholder={t(
            'Describe the video scene, camera motion, light, style...'
          )}
          onChange={(event) =>
            props.onVideoChange((current) => ({
              ...current,
              prompt: event.target.value,
            }))
          }
        />
      </div>

      {/* Endpoint Path & Route Compatibility */}
      <Card>
        <CardHeader className='pb-3'>
          <div className='flex items-center justify-between'>
            <div className='flex items-center gap-1.5 text-sm font-medium'>
              <Network className='size-4' />
              <span>{t('Endpoint Path & Route Compatibility')}</span>
            </div>
            {props.video.customPath.trim() ? (
              <Badge variant='secondary' className='font-mono text-xs'>
                {props.video.customPath}
              </Badge>
            ) : (
              <Badge variant='outline' className='text-xs'>
                {t('Auto Detect / Standard')}
              </Badge>
            )}
          </div>
        </CardHeader>
        <CardContent className='space-y-3 pt-1'>
          <div className='grid gap-3 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('Quick Path Preset')}</Label>
              <Select
                value={props.video.customPath}
                disabled={props.busy}
                onValueChange={(val) => {
                  props.onVideoChange((c) => ({ ...c, customPath: val || '' }))
                }}
              >
                <SelectTrigger className='w-full text-xs'>
                  <SelectValue placeholder={t('Select path preset')} />
                </SelectTrigger>
                <SelectContent>
                  {ENDPOINT_PATH_PRESETS.map((preset) => (
                    <SelectItem
                      key={preset.value || 'auto'}
                      value={preset.value}
                    >
                      {preset.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('Custom Path String')}</Label>
              <Input
                placeholder='/api/v3/contents/generations/tasks'
                value={props.video.customPath}
                disabled={props.busy}
                onChange={(e) =>
                  props.onVideoChange((c) => ({
                    ...c,
                    customPath: e.target.value,
                  }))
                }
                className='font-mono text-xs'
              />
            </div>
          </div>
          <p className='text-muted-foreground text-xs leading-relaxed'>
            {t(
              'Supports custom vendor path isolation. You can specify a path override here, or directly fill in a full URL (with /contents/generations/tasks) in Base URL above. Both are automatically normalized and supported.'
            )}
          </p>
        </CardContent>
      </Card>

      {/* Multimodal Input: First Frame / Reference Image */}
      <Card>
        <CardHeader className='pb-3'>
          <div className='flex items-center justify-between'>
            <div className='flex items-center space-x-2'>
              <Checkbox
                id='toggle-has-image'
                checked={props.video.hasImage}
                disabled={props.busy}
                onCheckedChange={(checked) =>
                  props.onVideoChange((cur) => ({
                    ...cur,
                    hasImage: Boolean(checked),
                  }))
                }
              />
              <Label
                htmlFor='toggle-has-image'
                className='flex cursor-pointer items-center gap-1.5 font-medium'
              >
                <ImageIcon className='size-4' />
                {t('Enable Input Image (Image-to-Video)')}
              </Label>
            </div>
            {!props.video.hasImage && (
              <Badge variant='outline' className='text-xs'>
                {t('Text-to-Video mode')}
              </Badge>
            )}
          </div>
        </CardHeader>
        {props.video.hasImage && (
          <CardContent className='space-y-4 pt-1'>
            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-1.5'>
                <Label className='text-xs'>{t('Image Role')}</Label>
                <Select
                  value={props.video.role}
                  disabled={props.busy}
                  onValueChange={(val) => {
                    if (val) {
                      props.onVideoChange((cur) => ({ ...cur, role: val }))
                    }
                  }}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_ROLES.map((role) => (
                      <SelectItem key={role.value} value={role.value}>
                        {role.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-1.5'>
                <Label className='text-xs'>{t('Upload Mode')}</Label>
                <Tabs
                  value={props.video.uploadMode}
                  onValueChange={(val) =>
                    props.onVideoChange((cur) => ({
                      ...cur,
                      uploadMode: val as 'url' | 'base64',
                    }))
                  }
                  className='w-full'
                >
                  <TabsList className='grid w-full grid-cols-2'>
                    <TabsTrigger value='url' className='text-xs'>
                      {t('Public URL')}
                    </TabsTrigger>
                    <TabsTrigger value='base64' className='text-xs'>
                      {t('Local File (Base64)')}
                    </TabsTrigger>
                  </TabsList>
                </Tabs>
              </div>
            </div>

            {props.video.uploadMode === 'url' ? (
              <div className='space-y-2'>
                <Label htmlFor='video-image-url' className='text-xs'>
                  {t('Image Public URL')}
                </Label>
                <Input
                  id='video-image-url'
                  placeholder='https://example.com/image.jpg'
                  value={props.video.imageUrl}
                  disabled={props.busy}
                  onChange={(e) =>
                    props.onVideoChange((cur) => ({
                      ...cur,
                      imageUrl: e.target.value,
                    }))
                  }
                />
                {props.video.imageUrl.trim() && (
                  <div className='mt-2 flex items-center gap-3 rounded-lg border p-2'>
                    <img
                      src={props.video.imageUrl}
                      alt='preview'
                      className='h-20 w-20 rounded border object-cover'
                      onError={(e) => {
                        ;(e.target as HTMLElement).style.display = 'none'
                      }}
                    />
                    <div className='text-muted-foreground text-xs'>
                      <p className='text-foreground font-medium'>
                        {t('Image Preview')}
                      </p>
                      <p className='max-w-sm truncate'>
                        {props.video.imageUrl}
                      </p>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className='space-y-3'>
                <div className='flex flex-wrap items-center gap-3'>
                  <input
                    ref={firstFileInputRef}
                    type='file'
                    accept='image/png,image/jpeg,image/webp,image/jpg'
                    className='hidden'
                    onChange={handleFirstFileChange}
                    disabled={props.busy}
                  />
                  <Button
                    type='button'
                    variant='secondary'
                    size='sm'
                    disabled={props.busy}
                    onClick={() => firstFileInputRef.current?.click()}
                    className='gap-1.5'
                  >
                    <Upload className='size-4' />
                    {t('Choose Image File')}
                  </Button>
                  <Button
                    type='button'
                    variant='ghost'
                    size='xs'
                    onClick={() => setShowBase64Textarea(!showBase64Textarea)}
                    className='text-xs'
                  >
                    {showBase64Textarea
                      ? t('Hide Data URI')
                      : t('Paste Data URI directly')}
                  </Button>
                </div>

                {showBase64Textarea && (
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('Manual Base64 Data URI')}
                    </Label>
                    <Textarea
                      rows={2}
                      placeholder='data:image/png;base64,...'
                      value={props.video.base64Data}
                      disabled={props.busy}
                      onChange={(e) =>
                        props.onVideoChange((cur) => ({
                          ...cur,
                          base64Data: e.target.value,
                        }))
                      }
                      className='font-mono text-xs'
                    />
                  </div>
                )}

                {props.video.base64Data ? (
                  <div className='bg-muted/20 flex items-center gap-3 rounded-lg border p-2'>
                    <img
                      src={props.video.base64Data}
                      alt='base64 preview'
                      className='h-20 w-20 rounded border object-cover'
                    />
                    <div className='text-muted-foreground space-y-1 text-xs'>
                      <p className='text-foreground font-medium'>
                        {t('Base64 Image Loaded')}
                      </p>
                      <p>
                        {t('Length: {{length}} chars', {
                          length: props.video.base64Data.length,
                        })}
                      </p>
                      <Button
                        variant='ghost'
                        size='xs'
                        className='text-destructive hover:text-destructive h-5 px-1.5 text-xs'
                        onClick={() =>
                          props.onVideoChange((cur) => ({
                            ...cur,
                            base64Data: '',
                          }))
                        }
                      >
                        {t('Clear')}
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            )}
          </CardContent>
        )}
      </Card>

      {/* Optional End Frame (First + Last Frame Mode) */}
      <Card>
        <CardHeader className='pb-3'>
          <div className='flex items-center space-x-2'>
            <Checkbox
              id='toggle-has-last-frame'
              checked={props.video.hasLastFrame}
              disabled={props.busy}
              onCheckedChange={(checked) =>
                props.onVideoChange((cur) => ({
                  ...cur,
                  hasLastFrame: Boolean(checked),
                }))
              }
            />
            <Label
              htmlFor='toggle-has-last-frame'
              className='flex cursor-pointer items-center gap-1.5 font-medium'
            >
              <Film className='size-4' />
              {t('Enable End Frame (Start-to-End Frame Mode)')}
            </Label>
          </div>
        </CardHeader>
        {props.video.hasLastFrame && (
          <CardContent className='space-y-4 pt-1'>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('End Frame Upload Mode')}</Label>
              <Tabs
                value={props.video.lastFrameMode}
                onValueChange={(val) =>
                  props.onVideoChange((cur) => ({
                    ...cur,
                    lastFrameMode: val as 'url' | 'base64',
                  }))
                }
                className='w-full'
              >
                <TabsList className='grid w-full grid-cols-2'>
                  <TabsTrigger value='url' className='text-xs'>
                    {t('Public URL')}
                  </TabsTrigger>
                  <TabsTrigger value='base64' className='text-xs'>
                    {t('Local File (Base64)')}
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            </div>

            {props.video.lastFrameMode === 'url' ? (
              <div className='space-y-2'>
                <Label htmlFor='video-last-frame-url' className='text-xs'>
                  {t('End Frame Public URL')}
                </Label>
                <Input
                  id='video-last-frame-url'
                  placeholder='https://example.com/end-frame.jpg'
                  value={props.video.lastFrameUrl}
                  disabled={props.busy}
                  onChange={(e) =>
                    props.onVideoChange((cur) => ({
                      ...cur,
                      lastFrameUrl: e.target.value,
                    }))
                  }
                />
                {props.video.lastFrameUrl.trim() && (
                  <div className='mt-2 flex items-center gap-3 rounded-lg border p-2'>
                    <img
                      src={props.video.lastFrameUrl}
                      alt='last frame preview'
                      className='h-20 w-20 rounded border object-cover'
                      onError={(e) => {
                        ;(e.target as HTMLElement).style.display = 'none'
                      }}
                    />
                    <div className='text-muted-foreground text-xs'>
                      <p className='text-foreground font-medium'>
                        {t('End Frame Preview')}
                      </p>
                      <p className='max-w-sm truncate'>
                        {props.video.lastFrameUrl}
                      </p>
                    </div>
                  </div>
                )}
              </div>
            ) : (
              <div className='space-y-3'>
                <input
                  ref={lastFileInputRef}
                  type='file'
                  accept='image/png,image/jpeg,image/webp,image/jpg'
                  className='hidden'
                  onChange={handleLastFileChange}
                  disabled={props.busy}
                />
                <Button
                  type='button'
                  variant='secondary'
                  size='sm'
                  disabled={props.busy}
                  onClick={() => lastFileInputRef.current?.click()}
                  className='gap-1.5'
                >
                  <Upload className='size-4' />
                  {t('Choose End Frame File')}
                </Button>
                {props.video.lastFrameBase64 ? (
                  <div className='bg-muted/20 flex items-center gap-3 rounded-lg border p-2'>
                    <img
                      src={props.video.lastFrameBase64}
                      alt='end frame preview'
                      className='h-20 w-20 rounded border object-cover'
                    />
                    <div className='text-muted-foreground space-y-1 text-xs'>
                      <p className='text-foreground font-medium'>
                        {t('End Frame Loaded')}
                      </p>
                      <Button
                        variant='ghost'
                        size='xs'
                        className='text-destructive hover:text-destructive h-5 px-1.5 text-xs'
                        onClick={() =>
                          props.onVideoChange((cur) => ({
                            ...cur,
                            lastFrameBase64: '',
                          }))
                        }
                      >
                        {t('Clear')}
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            )}
          </CardContent>
        )}
      </Card>

      {/* Optional Top-Level Parameters */}
      <Card>
        <CardHeader className='pb-2'>
          <CardTitle className='flex items-center gap-1.5 text-sm font-medium'>
            <Layers className='size-4' />
            {t('Optional Parameters (Included ONLY if checked)')}
          </CardTitle>
        </CardHeader>
        <CardContent className='space-y-4 pt-1'>
          <div className='grid gap-4 sm:grid-cols-2 md:grid-cols-3'>
            {/* Resolution */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-res'
                  checked={props.video.hasResolution}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasResolution: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-res'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Resolution (resolution)')}
                </Label>
              </div>
              {props.video.hasResolution && (
                <Select
                  value={props.video.resolution}
                  disabled={props.busy}
                  onValueChange={(val) => {
                    if (val) {
                      props.onVideoChange((c) => ({ ...c, resolution: val }))
                    }
                  }}
                >
                  <SelectTrigger className='h-8 text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_RESOLUTIONS.map((r) => (
                      <SelectItem key={r} value={r} className='text-xs'>
                        {r}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            {/* Ratio */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-ratio'
                  checked={props.video.hasRatio}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasRatio: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-ratio'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Aspect Ratio (ratio)')}
                </Label>
              </div>
              {props.video.hasRatio && (
                <Select
                  value={props.video.ratio}
                  disabled={props.busy}
                  onValueChange={(val) => {
                    if (val) props.onVideoChange((c) => ({ ...c, ratio: val }))
                  }}
                >
                  <SelectTrigger className='h-8 text-xs'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_RATIOS.map((r) => (
                      <SelectItem key={r} value={r} className='text-xs'>
                        {r}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
            </div>

            {/* Duration */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-duration'
                  checked={props.video.hasDuration}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasDuration: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-duration'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Duration (duration, sec)')}
                </Label>
              </div>
              {props.video.hasDuration && (
                <div className='flex items-center gap-2'>
                  <Input
                    type='number'
                    min={1}
                    max={60}
                    value={props.video.duration}
                    disabled={props.busy}
                    onChange={(e) =>
                      props.onVideoChange((c) => ({
                        ...c,
                        duration: Number.parseInt(e.target.value, 10) || 5,
                      }))
                    }
                    className='h-8 text-xs'
                  />
                  <span className='text-muted-foreground text-xs'>
                    {t('sec')}
                  </span>
                </div>
              )}
            </div>

            {/* Watermark */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-watermark'
                  checked={props.video.hasWatermark}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasWatermark: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-watermark'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Watermark (watermark)')}
                </Label>
              </div>
              {props.video.hasWatermark && (
                <div className='flex items-center justify-between pt-1'>
                  <span className='text-muted-foreground text-xs'>
                    {props.video.watermark ? t('Enabled') : t('Disabled')}
                  </span>
                  <Switch
                    checked={props.video.watermark}
                    disabled={props.busy}
                    onCheckedChange={(checked) =>
                      props.onVideoChange((c) => ({
                        ...c,
                        watermark: Boolean(checked),
                      }))
                    }
                  />
                </div>
              )}
            </div>

            {/* Seed */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-seed'
                  checked={props.video.hasSeed}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasSeed: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-seed'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Seed (seed)')}
                </Label>
              </div>
              {props.video.hasSeed && (
                <Input
                  type='number'
                  placeholder='e.g. 123456'
                  value={props.video.seed}
                  disabled={props.busy}
                  onChange={(e) =>
                    props.onVideoChange((c) => ({ ...c, seed: e.target.value }))
                  }
                  className='h-8 text-xs'
                />
              )}
            </div>

            {/* Generate Audio */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-audio'
                  checked={props.video.hasGenerateAudio}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasGenerateAudio: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-audio'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Generate Audio (generate_audio)')}
                </Label>
              </div>
              {props.video.hasGenerateAudio && (
                <div className='flex items-center justify-between pt-1'>
                  <span className='text-muted-foreground text-xs'>
                    {props.video.generateAudio ? t('Enabled') : t('Disabled')}
                  </span>
                  <Switch
                    checked={props.video.generateAudio}
                    disabled={props.busy}
                    onCheckedChange={(checked) =>
                      props.onVideoChange((c) => ({
                        ...c,
                        generateAudio: Boolean(checked),
                      }))
                    }
                  />
                </div>
              )}
            </div>

            {/* Return Last Frame */}
            <div className='space-y-2 rounded-lg border p-3'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-ret-last'
                  checked={props.video.hasReturnLastFrame}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasReturnLastFrame: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-ret-last'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Return Last Frame (return_last_frame)')}
                </Label>
              </div>
              {props.video.hasReturnLastFrame && (
                <div className='flex items-center justify-between pt-1'>
                  <span className='text-muted-foreground text-xs'>
                    {props.video.returnLastFrame ? t('Enabled') : t('Disabled')}
                  </span>
                  <Switch
                    checked={props.video.returnLastFrame}
                    disabled={props.busy}
                    onCheckedChange={(checked) =>
                      props.onVideoChange((c) => ({
                        ...c,
                        returnLastFrame: Boolean(checked),
                      }))
                    }
                  />
                </div>
              )}
            </div>

            {/* Custom Extra JSON */}
            <div className='space-y-2 rounded-lg border p-3 sm:col-span-2 md:col-span-2'>
              <div className='flex items-center space-x-2'>
                <Checkbox
                  id='opt-custom-json'
                  checked={props.video.hasCustomJson}
                  disabled={props.busy}
                  onCheckedChange={(checked) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      hasCustomJson: Boolean(checked),
                    }))
                  }
                />
                <Label
                  htmlFor='opt-custom-json'
                  className='cursor-pointer text-xs font-medium'
                >
                  {t('Custom Extra Parameters (Merged into top-level JSON)')}
                </Label>
              </div>
              {props.video.hasCustomJson && (
                <Textarea
                  rows={2}
                  placeholder='{"draft": false, "camera_fixed": true}'
                  value={props.video.customJson}
                  disabled={props.busy}
                  onChange={(e) =>
                    props.onVideoChange((c) => ({
                      ...c,
                      customJson: e.target.value,
                    }))
                  }
                  className='font-mono text-xs'
                />
              )}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Execution Results / Video Player */}
      {props.videoMetrics && (
        <Card className='border-primary/20 bg-primary/5'>
          <CardHeader className='pb-3'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div className='flex items-center gap-2'>
                <Video className='text-primary size-5' />
                <CardTitle className='text-sm font-semibold'>
                  {t('Execution Status & Output')}
                </CardTitle>
                <Badge variant={statusVariant(props.videoMetrics.status)}>
                  {props.videoMetrics.status || t('Unknown')}
                </Badge>
              </div>
              <div className='flex items-center gap-2'>
                {props.videoMetrics.elapsed_ms > 0 && (
                  <Badge variant='outline' className='gap-1 text-xs'>
                    <Clock className='size-3' />
                    {(props.videoMetrics.elapsed_ms / 1000).toFixed(1)}s
                  </Badge>
                )}
                {props.onExportPdf && (
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-7 gap-1 text-xs'
                    onClick={props.onExportPdf}
                  >
                    <Download className='size-3' />
                    {t('Export Video PDF')}
                  </Button>
                )}
                <RawJsonDialog videoMetrics={props.videoMetrics} />
              </div>
            </div>
          </CardHeader>
          <CardContent className='space-y-4'>
            {props.videoMetrics.endpoint_url && (
              <p className='text-muted-foreground font-mono text-xs break-all'>
                {t('Endpoint URL')}:{' '}
                <span className='text-foreground font-semibold'>POST</span>{' '}
                {props.videoMetrics.endpoint_url}
              </p>
            )}
            {props.videoMetrics.task_id && (
              <p className='text-muted-foreground font-mono text-xs'>
                {t('Task ID')}: {props.videoMetrics.task_id}
              </p>
            )}

            {props.videoMetrics.fail_reason && (
              <Alert variant='destructive'>
                <AlertTitle>{t('Generation Failed')}</AlertTitle>
                <AlertDescription className='text-xs break-all'>
                  {props.videoMetrics.fail_reason}
                </AlertDescription>
              </Alert>
            )}

            {props.videoMetrics.video_url && (
              <div className='bg-background/80 space-y-3 rounded-xl border p-4 shadow-sm'>
                <div className='flex items-center justify-between'>
                  <p className='text-foreground text-sm font-medium'>
                    {t('Generated Video Output')}
                  </p>
                  <a
                    href={props.videoMetrics.video_url}
                    target='_blank'
                    rel='noreferrer'
                    className='text-primary flex items-center gap-1 text-xs hover:underline'
                  >
                    {t('Open URL in new tab')}
                    <ExternalLink className='size-3' />
                  </a>
                </div>
                <div className='flex justify-center overflow-hidden rounded-lg bg-black'>
                  <video
                    src={props.videoMetrics.video_url}
                    controls
                    autoPlay
                    playsInline
                    className='max-h-96 w-auto max-w-full'
                  />
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Step Checks Table */}
      <div className='space-y-2'>
        <p className='text-sm font-medium'>{t('Task Execution Pipeline')}</p>
        <CheckTable
          checks={props.videoChecks}
          busy={props.busy}
          onRun={() => {}}
        />
      </div>
    </TabsContent>
  )
}
