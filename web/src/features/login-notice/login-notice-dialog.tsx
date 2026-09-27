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
import { useMutation, useQuery } from '@tanstack/react-query'
import {
  AlertTriangle,
  ArrowUpRight,
  Bell,
  Images,
  ShieldAlert,
  Smartphone,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { RichContent } from '@/components/rich-content'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { useAuthStore } from '@/stores/auth-store'

import { acknowledgeLoginNotice, getLoginNotice } from './api'
import { createDeviceFingerprint } from './device-fingerprint'

const FEATURED_RELEASE_KEY = '2026.09-asset-library-miniapp-preview'
const RELEASE_STORAGE_PREFIX = 'release-notices:v1:'

function getSeenReleaseKeys(userID: number): string[] {
  try {
    const stored = localStorage.getItem(`${RELEASE_STORAGE_PREFIX}${userID}`)
    const parsed: unknown = stored ? JSON.parse(stored) : []
    return Array.isArray(parsed)
      ? parsed.filter((key): key is string => typeof key === 'string')
      : []
  } catch {
    return []
  }
}

export function LoginNoticeDialog() {
  const { t } = useTranslation()
  const featuredReleaseImageUrl =
    import.meta.env.VITE_FEATURED_RELEASE_IMAGE_URL?.trim()
  const featuredReleaseImage = featuredReleaseImageUrl?.startsWith('https://')
    ? featuredReleaseImageUrl
    : undefined
  const sessionID = useAuthStore((state) => state.auth.session?.sid)
  const userID = useAuthStore((state) => state.auth.user?.id)
  const [dismissedSessionID, setDismissedSessionID] = useState<string>()
  const [dismissedReleases, setDismissedReleases] = useState<{
    userID: number
    keys: string[]
  }>()
  const noticeQuery = useQuery({
    queryKey: ['login-notice', sessionID],
    queryFn: getLoginNotice,
    enabled: Boolean(sessionID),
    staleTime: Infinity,
    retry: 1,
  })
  const acknowledgement = useMutation({
    mutationFn: async () => {
      const fingerprint = await createDeviceFingerprint()
      await acknowledgeLoginNotice(fingerprint)
    },
    onSuccess: () => finishNotice(),
    onError: () => toast.error(t('Failed to record acknowledgement')),
  })

  const notice = noticeQuery.data
  const storedReleaseKeys = userID ? getSeenReleaseKeys(userID) : []
  const seenReleases = new Set([
    ...storedReleaseKeys,
    ...(dismissedReleases && dismissedReleases.userID === userID
      ? dismissedReleases.keys
      : []),
  ])
  const featuredReleaseDue = !seenReleases.has(
    `featured:${FEATURED_RELEASE_KEY}`
  )
  const releaseAnnouncements =
    notice?.announcements.filter(
      (announcement) =>
        announcement.kind === 'release' && announcement.releaseKey
    ) ?? []
  const pendingReleases = releaseAnnouncements.filter(
    (announcement) => !seenReleases.has(`admin:${announcement.releaseKey}`)
  )
  const systemAnnouncements =
    notice?.announcements.filter(
      (announcement) => announcement.kind !== 'release'
    ) ?? []
  const complianceDue = Boolean(notice?.requires_acknowledgement)
  const releaseDue = featuredReleaseDue || pendingReleases.length > 0

  function finishNotice() {
    if (userID && releaseDue) {
      const nextKeys = [
        ...seenReleases,
        ...(featuredReleaseDue ? [`featured:${FEATURED_RELEASE_KEY}`] : []),
        ...pendingReleases.map(
          (announcement) => `admin:${announcement.releaseKey}`
        ),
      ]
      setDismissedReleases({ userID, keys: nextKeys })
      try {
        localStorage.setItem(
          `${RELEASE_STORAGE_PREFIX}${userID}`,
          JSON.stringify(nextKeys)
        )
      } catch {
        // Keep the current session dismissed when storage is unavailable.
      }
    }
    setDismissedSessionID(sessionID)
  }

  const open = Boolean(
    sessionID &&
    userID &&
    notice &&
    (complianceDue || releaseDue) &&
    dismissedSessionID !== sessionID
  )
  if (!notice) return null

  const periods = [
    { label: t('Today'), stats: notice.statistics.today },
    { label: t('Last 7 days'), stats: notice.statistics.seven_days },
    { label: t('Last 30 days'), stats: notice.statistics.thirty_days },
  ]

  return (
    <AlertDialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && complianceDue) return
        if (!nextOpen) finishNotice()
      }}
    >
      <AlertDialogContent className='max-h-[min(94svh,900px)] w-[calc(100%-2rem)] data-[size=default]:max-w-3xl data-[size=default]:sm:max-w-3xl'>
        <AlertDialogHeader className='place-items-start text-left'>
          <AlertDialogTitle>
            {releaseDue ? t('Product updates') : t('Compliance reminder')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {releaseDue
              ? t(
                  'New capabilities for your business, with the latest account notices in one place.'
                )
              : t('Review your recent video activity and account notices.')}
          </AlertDialogDescription>
        </AlertDialogHeader>

        <ScrollArea className='max-h-[min(72svh,690px)] pr-3'>
          <div className='space-y-4'>
            {featuredReleaseDue ? (
              <section
                aria-labelledby='featured-release-title'
                className='overflow-hidden rounded-xl border'
              >
                <div className='space-y-2 p-4 sm:p-5'>
                  <Badge variant='outline'>
                    {t('Product release preview')}
                  </Badge>
                  <h3
                    id='featured-release-title'
                    className='text-xl font-semibold tracking-tight text-balance sm:text-2xl'
                  >
                    {t('A faster path from assets to growth')}
                  </h3>
                </div>
                {featuredReleaseImage ? (
                  <img
                    src={featuredReleaseImage}
                    alt={t(
                      'Two colleagues organize media assets and check usage on a phone'
                    )}
                    referrerPolicy='no-referrer'
                    className='aspect-[2.5/1] w-full object-cover object-center'
                  />
                ) : null}
                <div className='grid gap-4 p-4 sm:grid-cols-2 sm:p-5'>
                  <div className='space-y-2'>
                    <div className='flex items-center gap-2 text-sm font-semibold'>
                      <Images className='size-4 text-blue-600' />
                      {t('Asset Library is live')}
                      <Badge variant='outline'>{t('Live now')}</Badge>
                    </div>
                    <p className='text-muted-foreground text-sm leading-relaxed'>
                      {t(
                        'Use Volcengine Ark-style Action requests with platform AK/SK to reduce migration work. Manage assets, channel copies, and video generation from one place.'
                      )}
                    </p>
                    <a
                      href='/assets/'
                      className='text-primary inline-flex items-center gap-1 text-sm font-medium underline-offset-4 hover:underline'
                    >
                      {t('Explore Asset Library')}{' '}
                      <ArrowUpRight className='size-4' />
                    </a>
                  </div>
                  <div className='space-y-2'>
                    <div className='flex items-center gap-2 text-sm font-semibold'>
                      <Smartphone className='size-4 text-[#c77b4a]' />
                      {t('WeChat mini program is coming')}
                      <Badge variant='secondary'>{t('Coming soon')}</Badge>
                    </div>
                    <p className='text-muted-foreground text-sm leading-relaxed'>
                      {t(
                        'Check token usage, tasks, and account activity on your phone. Show prospects your model catalog and service entry during customer conversations.'
                      )}
                    </p>
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'Mini program availability will be announced separately.'
                      )}
                    </p>
                  </div>
                </div>
              </section>
            ) : null}

            {pendingReleases.map((announcement) => (
              <section
                key={announcement.releaseKey}
                aria-label={announcement.title || t('Release update')}
                className='overflow-hidden rounded-xl border'
              >
                {announcement.imageUrl ? (
                  <img
                    src={announcement.imageUrl}
                    alt={announcement.title || t('Release update')}
                    referrerPolicy='no-referrer'
                    className='aspect-[2.5/1] w-full object-cover'
                  />
                ) : null}
                <div className='space-y-2 p-4 sm:p-5'>
                  <Badge variant='outline'>{t('Release update')}</Badge>
                  <h3 className='text-lg font-semibold'>
                    {announcement.title}
                  </h3>
                  <RichContent breaks content={announcement.content} />
                  {announcement.extra ? (
                    <RichContent
                      breaks
                      content={announcement.extra}
                      className='text-muted-foreground'
                    />
                  ) : null}
                </div>
              </section>
            ))}

            {complianceDue && systemAnnouncements.length > 0 ? (
              <section aria-labelledby='login-notice-announcements'>
                <h3
                  id='login-notice-announcements'
                  className='mb-2 flex items-center gap-2 text-sm font-medium'
                >
                  <Bell className='size-4' />
                  {t('System announcements')}
                </h3>
                <div className='space-y-2'>
                  {systemAnnouncements.map((announcement, index) => (
                    <Card
                      key={announcement.id ?? `announcement-${index}`}
                      size='sm'
                    >
                      <CardHeader>
                        <CardTitle className='flex items-center justify-between gap-2'>
                          <span>{t('Announcement')}</span>
                          {announcement.publishDate ? (
                            <Badge variant='outline'>
                              {announcement.publishDate}
                            </Badge>
                          ) : null}
                        </CardTitle>
                      </CardHeader>
                      <CardContent className='space-y-2'>
                        <RichContent breaks content={announcement.content} />
                        {announcement.extra ? (
                          <RichContent
                            breaks
                            content={announcement.extra}
                            className='text-muted-foreground'
                          />
                        ) : null}
                      </CardContent>
                    </Card>
                  ))}
                </div>
              </section>
            ) : null}

            {complianceDue ? (
              <section aria-labelledby='login-notice-activity'>
                <h3
                  id='login-notice-activity'
                  className='mb-2 flex items-center gap-2 text-sm font-medium'
                >
                  <ShieldAlert className='size-4' />
                  {t('Video generation and violation statistics')}
                </h3>
                <div className='overflow-hidden rounded-lg border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Period')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Video generations')}
                        </TableHead>
                        <TableHead className='text-right'>
                          {t('Violations')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {periods.map((period) => (
                        <TableRow key={period.label}>
                          <TableCell>{period.label}</TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {period.stats.generated}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            <span
                              className={
                                period.stats.violations > 0
                                  ? 'text-destructive font-medium'
                                  : undefined
                              }
                            >
                              {period.stats.violations}
                            </span>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </section>
            ) : null}

            {complianceDue ? (
              <Alert variant='destructive'>
                <AlertTriangle />
                <AlertTitle>{t('Acknowledgement required')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'A violation was recorded in the last 7 days. You must acknowledge this notice before closing it.'
                  )}{' '}
                  {t(
                    'Confirmation records a privacy-preserving device fingerprint for audit purposes.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
          </div>
        </ScrollArea>

        <AlertDialogFooter>
          <AlertDialogAction
            type='button'
            disabled={acknowledgement.isPending}
            onClick={() =>
              complianceDue ? acknowledgement.mutate() : finishNotice()
            }
          >
            {acknowledgement.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : null}
            {complianceDue ? t('I acknowledge') : t('Got it')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
