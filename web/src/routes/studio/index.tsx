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
import { createFileRoute, redirect } from '@tanstack/react-router'
import z from 'zod'

import { VideoStudio } from '@/features/video-studio'
import { getStatus } from '@/lib/api'
import { parseHeaderNavModulesFromStatus } from '@/lib/nav-modules'
import { useAuthStore } from '@/stores/auth-store'

const studioSearchSchema = z.object({
  project: z.string().max(64).optional().catch(undefined),
})

export const Route = createFileRoute('/studio/')({
  validateSearch: studioSearchSchema,
  beforeLoad: async ({ location }) => {
    const { auth } = useAuthStore.getState()
    if (!auth.user || !auth.accessToken) {
      throw redirect({ to: '/sign-in', search: { redirect: location.href } })
    }
    const status = await getStatus().catch(() => null)
    if (!parseHeaderNavModulesFromStatus(status).studio) {
      throw redirect({ to: '/dashboard' })
    }
  },
  component: StudioPage,
})

function StudioPage() {
  const { project } = Route.useSearch()
  return <VideoStudio projectId={project} />
}
