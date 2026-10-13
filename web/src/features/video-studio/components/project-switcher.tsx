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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Check, ChevronsUpDown, Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import { createProject, deleteProject } from '../api'
import { studioErrorMessage } from '../lib/errors'
import { fetchProjects, PROJECTS_QUERY_KEY } from '../lib/projects-query'

/** Top-bar project menu: switch, create, or delete projects. */
export function ProjectSwitcher(props: { projectId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const projects = useQuery({
    queryKey: PROJECTS_QUERY_KEY,
    queryFn: fetchProjects,
  })

  const open = (project: string) =>
    navigate({ to: '/studio', search: { project } })

  const create = useMutation({
    mutationFn: async () => {
      const response = await createProject(t('Untitled project'))
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    onSuccess: async (project) => {
      await queryClient.invalidateQueries({ queryKey: PROJECTS_QUERY_KEY })
      await open(project.id)
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  const remove = useMutation({
    mutationFn: async () => {
      const response = await deleteProject(props.projectId)
      if (!response.success) throw new Error(response.message)
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: PROJECTS_QUERY_KEY })
      // Without a project the studio reopens the most recent remaining one.
      await navigate({ to: '/studio', search: {} })
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant='ghost'
            size='sm'
            className='px-2'
            aria-label={t('Switch project')}
          />
        }
      >
        <ChevronsUpDown className='text-muted-foreground' />
        <span className='sr-only'>{t('Switch project')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='start' className='w-64'>
        <DropdownMenuGroup>
          <DropdownMenuLabel>{t('Projects')}</DropdownMenuLabel>
          <div className='max-h-72 overflow-y-auto'>
            {projects.data?.map((project) => (
              <DropdownMenuItem
                key={project.id}
                onClick={() => open(project.id)}
              >
                <span className='min-w-0 flex-1 truncate'>{project.name}</span>
                {project.id === props.projectId && <Check />}
              </DropdownMenuItem>
            ))}
          </div>
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <DropdownMenuItem
          disabled={create.isPending}
          onClick={() => create.mutate()}
        >
          <Plus />
          {t('New project')}
        </DropdownMenuItem>
        <DropdownMenuItem
          variant='destructive'
          disabled={remove.isPending}
          onClick={() => {
            if (
              window.confirm(
                t('Delete this project? Generated videos stay in Task Logs.')
              )
            ) {
              remove.mutate()
            }
          }}
        >
          <Trash2 />
          {t('Delete project')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
