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
import { Link, useNavigate } from '@tanstack/react-router'
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import { createProject, deleteProject, listProjects } from '../api'
import { studioErrorMessage } from '../lib/errors'

const PROJECTS_QUERY_KEY = ['video-studio', 'projects'] as const

export function ProjectList() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const projects = useQuery({
    queryKey: PROJECTS_QUERY_KEY,
    queryFn: async () => {
      const response = await listProjects(1, 100)
      if (!response.success) throw new Error(response.message)
      return response.data.items
    },
  })

  const create = useMutation({
    mutationFn: async () => {
      const response = await createProject(t('Untitled project'))
      if (!response.success) throw new Error(response.message)
      return response.data
    },
    onSuccess: async (project) => {
      await queryClient.invalidateQueries({ queryKey: PROJECTS_QUERY_KEY })
      await navigate({
        to: '/video-studio/$projectId',
        params: { projectId: project.id },
      })
    },
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  const remove = useMutation({
    mutationFn: async (id: string) => {
      const response = await deleteProject(id)
      if (!response.success) throw new Error(response.message)
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: PROJECTS_QUERY_KEY }),
    onError: (error) => toast.error(studioErrorMessage(error)),
  })

  return (
    <div className='mx-auto flex w-full max-w-5xl flex-col gap-4 p-6'>
      <div className='flex items-center justify-between gap-4'>
        <div>
          <h1 className='text-xl font-semibold'>{t('Video Studio')}</h1>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Arrange prompts and reference images on a canvas and generate videos.'
            )}
          </p>
        </div>
        <Button disabled={create.isPending} onClick={() => create.mutate()}>
          <Plus />
          {t('New project')}
        </Button>
      </div>
      {projects.data?.length === 0 && (
        <div className='text-muted-foreground rounded-lg border border-dashed p-10 text-center text-sm'>
          {t('No projects yet. Create one to start.')}
        </div>
      )}
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3'>
        {projects.data?.map((project) => (
          <Card key={project.id} className='relative'>
            <CardHeader>
              <CardTitle className='truncate pr-8'>
                <Link
                  to='/video-studio/$projectId'
                  params={{ projectId: project.id }}
                  className='after:absolute after:inset-0 focus-visible:underline focus-visible:outline-none'
                >
                  {project.name}
                </Link>
              </CardTitle>
              <CardDescription>
                {t('Updated {{time}}', {
                  time: new Date(project.updated_at * 1000).toLocaleString(),
                })}
              </CardDescription>
            </CardHeader>
            <Button
              size='icon-sm'
              variant='ghost'
              className='absolute top-3 right-3 z-10'
              aria-label={t('Delete project')}
              disabled={remove.isPending}
              onClick={() => {
                if (
                  window.confirm(
                    t(
                      'Delete this project? Generated videos stay in Task Logs.'
                    )
                  )
                ) {
                  remove.mutate(project.id)
                }
              }}
            >
              <Trash2 />
            </Button>
          </Card>
        ))}
      </div>
    </div>
  )
}
