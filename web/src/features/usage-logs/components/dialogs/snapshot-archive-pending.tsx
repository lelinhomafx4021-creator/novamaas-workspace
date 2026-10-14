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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

export function SnapshotArchivePending(props: {
  refreshing: boolean
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  return (
    <Alert>
      <AlertTitle>{t('Snapshot archive in progress')}</AlertTitle>
      <AlertDescription>
        {t(
          'Snapshots are being uploaded. Ready snapshots remain available while the archive completes.'
        )}
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={props.refreshing}
          onClick={props.onRefresh}
        >
          {t('Reload snapshots')}
        </Button>
      </AlertDescription>
    </Alert>
  )
}
