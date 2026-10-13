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
import { Cloud, CloudAlert, CloudUpload, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import {
  useVideoStudioStore,
  type SaveState,
} from '@/stores/video-studio-store'

import { ProjectSwitcher } from './project-switcher'

const SAVE_LABELS: Record<SaveState, string> = {
  saved: 'Saved',
  dirty: 'Unsaved changes',
  saving: 'Saving...',
  conflict: 'Edited elsewhere',
  error: 'Save failed',
}

const SAVE_ICONS: Record<SaveState, typeof Cloud> = {
  saved: Cloud,
  dirty: CloudUpload,
  saving: CloudUpload,
  conflict: CloudAlert,
  error: CloudAlert,
}

type StudioToolbarProps = {
  onAddNode: () => void
  onReload: () => void
}

export function StudioToolbar(props: StudioToolbarProps) {
  const { t } = useTranslation()
  const projectId = useVideoStudioStore((state) => state.projectId)
  const name = useVideoStudioStore((state) => state.name)
  const saveState = useVideoStudioStore((state) => state.saveState)
  const setName = useVideoStudioStore((state) => state.setName)
  const failed = saveState === 'conflict' || saveState === 'error'
  const SaveIcon = SAVE_ICONS[saveState]

  return (
    <header className='bg-background/70 flex h-11 shrink-0 items-center gap-1 border-b px-2 backdrop-blur-xl'>
      <ProjectSwitcher projectId={projectId} />
      <Input
        aria-label={t('Project name')}
        className='hover:bg-muted/60 focus-visible:bg-muted/60 h-7 w-52 border-transparent bg-transparent px-2 text-sm font-medium shadow-none'
        value={name}
        maxLength={128}
        onChange={(event) => setName(event.target.value)}
      />
      <span
        role='status'
        aria-live='polite'
        className={cn(
          'bg-muted/50 text-muted-foreground ml-1 flex items-center gap-1 rounded-full px-2 py-0.5 font-mono text-[10px]',
          failed && 'bg-destructive/15 text-destructive'
        )}
      >
        <SaveIcon className='size-3.5' />
        {t(SAVE_LABELS[saveState])}
      </span>
      {saveState === 'conflict' && (
        <Button size='xs' variant='outline' onClick={props.onReload}>
          {t('Reload latest version')}
        </Button>
      )}
      <div className='flex-1' />
      <Button size='sm' onClick={props.onAddNode}>
        <Plus />
        {t('Add generation node')}
      </Button>
    </header>
  )
}
