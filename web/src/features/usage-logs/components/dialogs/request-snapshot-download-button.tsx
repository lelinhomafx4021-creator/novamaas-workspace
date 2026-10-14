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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import { downloadRequestSnapshot } from '../../request-snapshot-download'

export function RequestSnapshotDownloadButton(props: { url: string }) {
  const { t } = useTranslation()
  const [downloading, setDownloading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleDownload = async () => {
    if (downloading) return
    setDownloading(true)
    setError(null)
    try {
      await downloadRequestSnapshot(props.url)
    } catch (downloadError) {
      setError(
        downloadError instanceof Error
          ? downloadError.message
          : t('Failed to download snapshot')
      )
    } finally {
      setDownloading(false)
    }
  }

  return (
    <div className='flex min-w-0 flex-col gap-2'>
      <Button
        type='button'
        variant='outline'
        size='sm'
        disabled={downloading}
        onClick={() => void handleDownload()}
      >
        {downloading && <Spinner data-icon='inline-start' aria-hidden='true' />}
        {downloading ? t('Downloading...') : t('Download archived JSON')}
      </Button>
      {error && (
        <Alert variant='destructive'>
          <AlertTitle>{t('Failed to download snapshot')}</AlertTitle>
          <AlertDescription className='break-all'>{error}</AlertDescription>
        </Alert>
      )}
    </div>
  )
}
