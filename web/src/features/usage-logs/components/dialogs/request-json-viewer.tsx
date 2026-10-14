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

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'

import { MAX_REQUEST_SNAPSHOT_PREVIEW_CHARACTERS } from './request-snapshot-limits'

export default function RequestJsonViewer({
  filename = 'request-body.json',
  json,
  downloadUrl,
}: {
  filename?: string
  json: string
  downloadUrl?: string
}) {
  const { t } = useTranslation()
  if (json.length > MAX_REQUEST_SNAPSHOT_PREVIEW_CHARACTERS) {
    return (
      <Alert>
        <AlertTitle>{t('Large request snapshot')}</AlertTitle>
        <AlertDescription>
          {t(
            'This snapshot is too large to preview. Download the complete JSON file or copy its content.'
          )}
          <div className='mt-2 flex flex-wrap gap-2'>
            <CopyButton
              value={json}
              size='sm'
              variant='outline'
              aria-label={t('Copy complete JSON')}
            >
              {t('Copy complete JSON')}
            </CopyButton>
            {!downloadUrl && (
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => {
                  const blob = new Blob([json], {
                    type: 'application/json;charset=utf-8',
                  })
                  const url = URL.createObjectURL(blob)
                  const link = document.createElement('a')
                  link.href = url
                  link.download = filename
                  link.click()
                  URL.revokeObjectURL(url)
                }}
              >
                {t('Download complete JSON')}
              </Button>
            )}
          </div>
        </AlertDescription>
      </Alert>
    )
  }
  return (
    <CodeBlock
      code={json}
      enableCollapse={false}
      filename={filename}
      language='json'
      maxExpandedLines={28}
      showLineNumbers
      title={t('JSON')}
    >
      <CodeBlockCopyButton />
    </CodeBlock>
  )
}
