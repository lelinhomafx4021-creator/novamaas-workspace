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
import { Check, Code2, Copy } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import type { VideoMetrics } from '../types'

function formatJSON(raw?: string): string {
  if (!raw || !raw.trim()) return ''
  try {
    const parsed = JSON.parse(raw)
    return JSON.stringify(parsed, null, 2)
  } catch {
    return raw
  }
}

function JsonCodeViewer({ code, label }: { code: string; label: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    if (!code) return
    try {
      await navigator.clipboard.writeText(code)
      setCopied(true)
      toast.success(t('JSON copied to clipboard'))
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t('Failed to copy'))
    }
  }

  if (!code) {
    return (
      <div className='bg-muted/40 text-muted-foreground flex h-60 items-center justify-center rounded-lg border border-dashed text-sm'>
        {t('No data available for {{label}} yet', { label })}
      </div>
    )
  }

  return (
    <div className='relative'>
      <div className='absolute top-2 right-2 z-10'>
        <Button
          variant='secondary'
          size='xs'
          className='h-7 gap-1 px-2 text-xs'
          onClick={handleCopy}
        >
          {copied ? <Check className='size-3.5' /> : <Copy className='size-3.5' />}
          {copied ? t('Copied') : t('Copy JSON')}
        </Button>
      </div>
      <pre className='bg-muted/60 max-h-96 overflow-auto rounded-lg border p-3.5 pt-8 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap'>
        {code}
      </pre>
    </div>
  )
}

export function RawJsonDialog(props: {
  videoMetrics?: VideoMetrics | null
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'request' | 'submit' | 'poll'>('request')

  const requestJSON = formatJSON(props.videoMetrics?.raw_request_json)
  const submitJSON = formatJSON(props.videoMetrics?.raw_submit_response_json)
  const pollJSON = formatJSON(props.videoMetrics?.raw_poll_response_json)

  const hasAnyData = Boolean(requestJSON || submitJSON || pollJSON)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger
        render={
          <Button
            variant='outline'
            size='sm'
            disabled={props.disabled || !hasAnyData}
            className='gap-1.5'
          >
            <Code2 className='size-4' />
            {t('View Raw Request & Response JSON')}
          </Button>
        }
      />
      <DialogContent className='max-w-2xl sm:max-w-3xl'>
        <DialogHeader>
          <DialogTitle>{t('Raw Request & Response JSON')}</DialogTitle>
          <DialogDescription>
            {t(
              'Inspect exact JSON payloads sent to and received from Volcano Ark API.'
            )}
          </DialogDescription>
        </DialogHeader>

        <Tabs
          value={activeTab}
          onValueChange={(val) => setActiveTab(val as 'request' | 'submit' | 'poll')}
          className='w-full'
        >
          <TabsList className='grid w-full grid-cols-3'>
            <TabsTrigger value='request' className='text-xs'>
              {t('1. Submit Request Body')}
            </TabsTrigger>
            <TabsTrigger value='submit' className='text-xs'>
              {t('2. Submit Response Body')}
            </TabsTrigger>
            <TabsTrigger value='poll' className='text-xs'>
              {t('3. Final Poll Response')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value='request' className='mt-3'>
            <JsonCodeViewer
              code={requestJSON}
              label={t('Submit Request Body')}
            />
          </TabsContent>

          <TabsContent value='submit' className='mt-3'>
            <JsonCodeViewer
              code={submitJSON}
              label={t('Submit Response Body')}
            />
          </TabsContent>

          <TabsContent value='poll' className='mt-3'>
            <JsonCodeViewer
              code={pollJSON}
              label={t('Final Poll Response Body')}
            />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
