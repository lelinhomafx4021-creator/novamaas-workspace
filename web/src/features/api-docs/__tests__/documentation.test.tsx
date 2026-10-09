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
import { readFileSync, readdirSync } from 'node:fs'
import { URL as NodeURL } from 'node:url'

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRouteWithContext,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { Route as DocsRoute } from '@/routes/docs'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

const requestBody = `{
  "model": "YOUR_VIDEO_MODEL_ID",
  "content": [
    {
      "type": "image_url",
      "image_url": {
        "url": "https://files.example.com/assets/sample.jpg"
      }
    }
  ],
  "duration": 5
}`

const mockDocuments = [
  {
    id: 'quickstart',
    title: '快速开始',
    content: `## 首次调用\n\n使用 YOUR_API_KEY。\n\n\`\`\`json\n${requestBody}\n\`\`\``,
  },
  {
    id: 'text',
    title: '文本模型',
    content: '## 流式响应\n\n参数 stream 使用 SSE 流式传输。',
  },
]

const initialAdapter = api.defaults.adapter
const initialConfig = useSystemConfigStore.getState()
let client: QueryClient
let requireAuth: boolean
let enabled: boolean
let failDocs: boolean
let apiDocuments: typeof mockDocuments

beforeEach(() => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  localStorage.clear()
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState(initialConfig)
  useSystemConfigStore.getState().setLoading(false)
  requireAuth = false
  enabled = true
  failDocs = false
  apiDocuments = mockDocuments
  api.defaults.adapter = async (config) => {
    let data: unknown = ''
    if (config.url === '/api/status') {
      data = {
        HeaderNavModules: JSON.stringify({ docs: { enabled, requireAuth } }),
      }
    }
    if (config.url === '/api/docs') {
      if (failDocs) throw new Error('Documentation request failed')
      data = apiDocuments
    }
    return {
      data: { success: true, data },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
})

afterEach(async () => {
  cleanup()
  await api.get('/api/status')
  client?.clear()
  api.defaults.adapter = initialAdapter
  useAuthStore.getState().auth.reset()
  useSystemConfigStore.setState(initialConfig)
  localStorage.clear()
})

async function renderDocumentation(path = '/docs') {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  const root = createRootRouteWithContext<{ queryClient: QueryClient }>()({
    component: Outlet,
  })
  const docs = DocsRoute.update({
    id: '/docs/',
    path: '/docs/',
    getParentRoute: () => root,
  } as Parameters<typeof DocsRoute.update>[0])
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => <h1>Home</h1>,
  })
  const login = createRoute({
    getParentRoute: () => root,
    path: '/sign-in',
    component: () => <h1>Login</h1>,
  })
  const router = createRouter({
    routeTree: root.addChildren([docs, home, login]),
    context: { queryClient: client },
    history: createMemoryHistory({ initialEntries: [path] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return router
}

test('search finds matching topics, navigates to results and resets', async () => {
  const user = userEvent.setup()
  const router = await renderDocumentation()
  await screen.findByRole('heading', { level: 1, name: '快速开始' })

  const menu = screen.getByRole('button', { name: 'Documentation menu' })
  await user.click(menu)
  const input = screen.getByRole('searchbox', { name: 'Search documentation' })
  await user.type(input, 'stream')

  const nav = screen.getByRole('navigation', { name: 'API documentation' })
  const result = within(nav).getByRole('link', { name: /文本模型/ })
  expect(result).toHaveTextContent('stream')
  await user.click(result)

  await screen.findByRole('heading', { level: 1, name: '文本模型' })
  expect(router.state.location.href).toBe('/docs?article=text')
})

test('quickstart presents and copies gateway address while other articles omit it', async () => {
  const user = userEvent.setup()
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText)

  await renderDocumentation('/docs?article=quickstart')
  await screen.findByRole('heading', { level: 1, name: '快速开始' })
  const addresses = screen.getByRole('region', { name: 'Gateway URL' })
  expect(within(addresses).getByText('https://gateway.ai.shilijia.xyz')).toBeVisible()

  await user.click(within(addresses).getByRole('button', { name: 'Copy Gateway URL' }))
  expect(writeText).toHaveBeenCalledWith('https://gateway.ai.shilijia.xyz')

  cleanup()
  await renderDocumentation('/docs?article=text')
  await screen.findByRole('heading', { level: 1, name: '文本模型' })
  expect(screen.queryByRole('region', { name: 'Gateway URL' })).not.toBeInTheDocument()
})

test('code blocks render and support plain-text clipboard copying', async () => {
  const user = userEvent.setup()
  const writeText = vi.fn().mockResolvedValue(undefined)
  vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText)

  await renderDocumentation('/docs?article=quickstart')
  await screen.findByRole('heading', { level: 1, name: '快速开始' })
  await user.click(screen.getByRole('button', { name: 'Copy to clipboard' }))
  expect(writeText).toHaveBeenCalledWith(requestBody)
  expect(await screen.findByRole('button', { name: 'Copied' })).toBeVisible()
})

test('deep link scrolls to anchor heading and supports article traversal', async () => {
  const user = userEvent.setup()
  const scroll = vi.spyOn(HTMLElement.prototype, 'scrollIntoView')
  const router = await renderDocumentation('/docs?article=quickstart#首次调用')

  const heading = await screen.findByRole('heading', { level: 2, name: '首次调用' })
  await waitFor(() => expect(scroll.mock.contexts).toContain(heading))

  const next = screen.getByRole('link', { name: /Next\s*文本模型/ })
  await user.click(next)
  await screen.findByRole('heading', { level: 1, name: '文本模型' })
  expect(router.state.location.search.article).toBe('text')
})

test('enforces access control with authentication redirects and error recovery', async () => {
  const user = userEvent.setup()
  enabled = true
  requireAuth = true
  await renderDocumentation()
  expect(await screen.findByRole('heading', { level: 1, name: 'Login' })).toBeVisible()

  cleanup()
  enabled = false
  await renderDocumentation()
  expect(await screen.findByRole('heading', { level: 1, name: 'Home' })).toBeVisible()

  cleanup()
  enabled = true
  requireAuth = false
  failDocs = true
  await renderDocumentation()
  expect(await screen.findByRole('heading', { level: 1, name: 'Documentation unavailable' })).toBeVisible()

  failDocs = false
  await user.click(screen.getByRole('button', { name: 'Retry' }))
  expect(await screen.findByRole('heading', { level: 1, name: '快速开始' })).toBeVisible()
})

test('renders full platform documentation suite with correct navigation', async () => {
  const user = userEvent.setup()
  const docDir = new NodeURL('../../../../../docs/platform-api', import.meta.url)
  const files = readdirSync(docDir)
    .filter((file) => file.endsWith('.md'))
    .sort()

  apiDocuments = files.map((file) => {
    const raw = readFileSync(new NodeURL(file, `${docDir}/`), 'utf-8')
    const [title, ...body] = raw.split('\n')
    const [, id] = file.replace(/\.md$/, '').split('-')
    return {
      id,
      title: title.replace(/^#\s*/, '').trim(),
      content: body.join('\n').trim(),
    }
  })

  await renderDocumentation()
  await screen.findByRole('heading', { level: 1, name: '快速接入' })

  const nav = screen.getByRole('navigation', { name: 'API documentation' })
  const links = within(nav).getAllByRole('link')
  expect(links).toHaveLength(7)

  const titles = ['快速接入', '文本模型', '图像生成', '视频生成', '素材库 API', '媒体任务 Webhook', '素材库 Webhook']
  titles.forEach((expectedTitle, idx) => {
    expect(links[idx]).toHaveTextContent(expectedTitle)
  })

  await user.click(within(nav).getByRole('link', { name: /视频生成/ }))
  expect(await screen.findByRole('heading', { level: 1, name: '视频生成' })).toBeVisible()
  expect(screen.getAllByText('/api/v3/contents/generations/tasks')[0]).toBeVisible()
})
