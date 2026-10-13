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
import axios from 'axios'
import { useEffect, useRef } from 'react'

import { useVideoStudioStore } from '@/stores/video-studio-store'

import { saveProject } from '../api'

const AUTOSAVE_DELAY_MS = 800

/**
 * Saves the project shortly after edits stop. A 409 stops autosave and keeps
 * local edits so the user can reload or copy them; nothing is overwritten.
 */
export function useProjectAutosave() {
  const saveState = useVideoStudioStore((state) => state.saveState)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (saveState !== 'dirty') return
    timer.current = setTimeout(async () => {
      const store = useVideoStudioStore.getState()
      store.markSaving()
      try {
        const response = await saveProject(store.projectId, {
          revision: store.revision,
          name: store.name,
          draft: JSON.stringify(store.draft),
          canvas: JSON.stringify(store.canvas),
        })
        if (!response.success) throw new Error(response.message)
        useVideoStudioStore.getState().markSaved(response.data.revision)
      } catch (error) {
        const conflict =
          axios.isAxiosError(error) && error.response?.status === 409
        useVideoStudioStore.getState().markFailed(conflict)
      }
    }, AUTOSAVE_DELAY_MS)
    return () => {
      if (timer.current) clearTimeout(timer.current)
    }
  }, [saveState])
}
