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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'

import '../studio.css'
import { createProject } from '../api'
import { studioErrorMessage } from '../lib/errors'
import { fetchProjects, PROJECTS_QUERY_KEY } from '../lib/projects-query'
import { StudioEditor } from './studio-editor'

/**
 * Full-screen studio under the top navigation. Without a project in the URL
 * it opens the most recently edited one, creating the first project on the
 * fly, so users land on a canvas instead of an empty project list.
 */
export function VideoStudio(props: { projectId?: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const creating = useRef(false)
  const projects = useQuery({
    queryKey: PROJECTS_QUERY_KEY,
    queryFn: fetchProjects,
    enabled: !props.projectId,
  })

  useEffect(() => {
    if (props.projectId || !projects.data) return
    const open = (project: string) =>
      navigate({ to: '/studio', search: { project }, replace: true })
    const latest = projects.data[0]
    if (latest) {
      void open(latest.id)
      return
    }
    // Guard against a second create from a re-run effect.
    if (creating.current) return
    creating.current = true
    void createProject(t('Untitled project')).then((response) => {
      if (response.success) void open(response.data.id)
    })
  }, [props.projectId, projects.data, navigate, t])

  return (
    <PublicLayout showMainContainer={false}>
      <div className='studio-surface fixed inset-x-0 top-16 bottom-0'>
        {props.projectId ? (
          <StudioEditor key={props.projectId} projectId={props.projectId} />
        ) : (
          <div className='text-muted-foreground flex h-full items-center justify-center font-mono text-xs tracking-widest uppercase'>
            {projects.error
              ? studioErrorMessage(projects.error)
              : t('Loading...')}
          </div>
        )}
      </div>
    </PublicLayout>
  )
}
