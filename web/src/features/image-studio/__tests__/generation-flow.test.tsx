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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import { ImageStudioPage } from '@/routes/image-studio'
import { useAuthStore } from '@/stores/auth-store'

import * as api from '../api'
import { ImageStudio } from '../image-studio'
import type { StudioReference, StudioRun } from '../types'

vi.mock('../api')
vi.mock('@/features/keys/api')

const completed: StudioRun = {
  id: 'run-1',
  owner_user_id: 7,
  status: 'completed',
  prompt: 'A mountain',
  created_at: 1,
  model: 'dall-e-3',
  quota: 0,
  results: [
    {
      id: 'result-1',
      index: 0,
      preview_url: 'https://example.test/original.png',
      thumbnail_url: 'https://example.test/thumbnail.png',
    },
  ],
}
const key = {
  id: 1,
  name: 'Image key',
  key: '***',
  status: 1,
  group: '',
} as ApiKey

let client: QueryClient
beforeEach(() => {
  client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  client.setQueryData(['status'], { ImageStudioEnabled: true })
  useAuthStore.getState().auth.setBundle({
    access_token: 'studio-test-session',
    token_type: 'Bearer',
    access_expires_at: 2000000000,
    user: { id: 7, username: 'artist', role: 1, group: 'default' },
    session: {
      sid: 'studio-session',
      current: true,
      login_method: 'password',
      ip: '127.0.0.1',
      user_agent: 'vitest',
      created_at: 1,
      last_active_at: 1,
      expires_at: 2000000000,
    },
  })
  vi.mocked(api.getStudioSettings).mockResolvedValue({
    enabled: true,
    max_images: 4,
    max_prompt_characters: 1000,
    max_reference_images: 8,
    max_reference_bytes: 20 * 1024 * 1024,
  })
  vi.mocked(getApiKeys).mockResolvedValue({
    success: true,
    data: { items: [key], total: 1, page: 1, page_size: 100 },
  })
  vi.mocked(api.listStudioModels).mockImplementation(async (_id, kind) =>
    kind === 'text' ? ['text-model'] : ['dall-e-3']
  )
  vi.mocked(api.listStudioRuns).mockResolvedValue({ items: [], total: 0 })
})
afterEach(() => {
  client.clear()
  useAuthStore.getState().auth.reset()
  vi.resetAllMocks()
})

async function openStudio(guest = false) {
  const root = createRootRoute({ component: Outlet })
  const studio = createRoute({
    getParentRoute: () => root,
    path: '/image-studio',
    component: guest ? ImageStudioPage : ImageStudio,
  })
  const dashboard = createRoute({
    getParentRoute: () => root,
    path: '/dashboard',
    component: () => <main>Dashboard page</main>,
  })
  const signIn = createRoute({
    getParentRoute: () => root,
    path: '/sign-in',
    validateSearch: (search: Record<string, unknown>) => ({
      redirect: String(search.redirect || ''),
    }),
    component: () => <main>Sign in page</main>,
  })
  const router = createRouter({
    routeTree: root.addChildren([studio, dashboard, signIn]),
    history: createMemoryHistory({ initialEntries: ['/image-studio'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  if (guest) await screen.findByRole('heading', { name: 'Image creation' })
  else await screen.findByText('dall-e-3', { exact: true })
  const user = userEvent.setup()
  return user
}

test('guests can browse the editor and examples without fetching keys, models or private records', async () => {
  useAuthStore.getState().auth.reset()
  const user = await openStudio(true)
  expect(screen.getByRole('main').closest('.h-svh')).toHaveClass(
    'w-full',
    'overflow-hidden'
  )
  const retention = screen.getByText(
    'Generated images are kept for 30 days, then automatically deleted. Please download them in time.'
  )
  expect(retention).toBeVisible()
  expect(
    screen
      .getByRole('form', { name: 'Image prompt editor' })
      .compareDocumentPosition(retention) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  await user.click(
    screen.getByRole('button', { name: 'Cinematic photography' })
  )
  expect(
    screen.getByRole('textbox', { name: 'Describe your image' })
  ).not.toHaveValue('')
  expect(screen.getByRole('button', { name: 'Optimize prompt' })).toBeDisabled()
  expect(getApiKeys).not.toHaveBeenCalled()
  expect(api.listStudioModels).not.toHaveBeenCalled()
  expect(api.listStudioRuns).not.toHaveBeenCalled()
  expect(api.createStudioRun).not.toHaveBeenCalled()
  expect(screen.getByText('Your creative journey starts here')).toBeVisible()
  expect(screen.queryByText('No generation records')).not.toBeInTheDocument()
  expect(
    screen.getByRole('link', {
      name: 'Sign in to view your generation records',
    })
  ).toHaveAttribute('href', '/sign-in?redirect=%2Fimage-studio')
  const signIn = screen.getByRole('link', { name: 'Sign in to create images' })
  expect(signIn).toHaveAttribute('href', '/sign-in?redirect=%2Fimage-studio')
  await user.click(signIn)
  expect(await screen.findByText('Sign in page')).toBeVisible()
})

test('signing out clears the prompt, private generation records and model selection from the editor', async () => {
  vi.mocked(api.listStudioRuns).mockResolvedValue({
    items: [completed],
    total: 1,
  })
  const user = await openStudio()
  await user.type(
    screen.getByRole('textbox', { name: 'Describe your image' }),
    'Private draft'
  )
  expect(screen.getByRole('button', { name: 'Delete image' })).toBeVisible()
  await act(() => useAuthStore.getState().auth.reset())
  expect(
    await screen.findByRole('link', { name: 'Sign in to create images' })
  ).toBeVisible()
  expect(
    screen.queryByRole('button', { name: 'Delete image' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByText('dall-e-3', { exact: true })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('textbox', { name: 'Describe your image' })
  ).toHaveValue('')
})

test('the canvas and editor share the creation area while records remain separate', async () => {
  await openStudio()
  const editor = screen.getByRole('form', { name: 'Image prompt editor' })
  const canvas = screen.getByRole('region', { name: 'Generation results' })
  const records = screen.getByRole('region', { name: 'Generation records' })
  const workspace = screen.getByRole('region', { name: 'Creative canvas' })
  expect(workspace).toContainElement(canvas)
  expect(workspace).toContainElement(editor)
  expect(workspace).not.toContainElement(records)
  expect(
    canvas.compareDocumentPosition(editor) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  expect(editor).not.toContainElement(records)
  expect(
    await within(records).findByText('No generation records')
  ).toBeVisible()
  expect(
    within(records).queryByRole('link', {
      name: 'Sign in to view your generation records',
    })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('complementary', { name: 'Workspace' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('textbox', { name: 'Describe your image' })
  ).toHaveAccessibleDescription(/Text model/)
})

test.each([
  ['keep', 'Delete generated image only'],
  ['delete', 'Also delete asset library image'],
] as const)(
  'deleting a saved image explains cleanup and applies the %s library choice',
  async (choice, label) => {
    const saved = {
      ...completed,
      results: [{ ...completed.results[0], asset_id: 'asset-1' }],
    }
    vi.mocked(api.listStudioRuns).mockResolvedValueOnce({
      items: [saved],
      total: 1,
    })
    vi.mocked(api.deleteStudioResult).mockResolvedValue(undefined)
    const user = await openStudio()
    await user.click(screen.getByRole('button', { name: 'Delete image' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Delete image' })
    expect(
      within(dialog).getByText(
        'Deleted images cannot be restored. Files are removed in the background.'
      )
    ).toBeVisible()
    await user.click(within(dialog).getByRole('button', { name: label }))
    await waitFor(() =>
      expect(api.deleteStudioResult).toHaveBeenCalledWith('result-1', choice)
    )
    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(api.deleteStudioRun).not.toHaveBeenCalled()
  }
)

test('deleting a failed record confirms irreversible removal and deletes the record instead of an image', async () => {
  vi.mocked(api.listStudioRuns).mockResolvedValueOnce({
    items: [{ ...completed, status: 'failed', results: [] }],
    total: 1,
  })
  vi.mocked(api.deleteStudioRun).mockResolvedValue(undefined)
  const user = await openStudio()
  await user.click(screen.getByRole('button', { name: 'Delete record' }))
  expect(
    screen.getByText('Delete this generation record? This cannot be undone.')
  ).toBeVisible()
  await user.click(
    within(screen.getByRole('alertdialog')).getByRole('button', {
      name: 'Delete',
    })
  )
  await waitFor(() =>
    expect(api.deleteStudioRun).toHaveBeenCalledWith('run-1', undefined)
  )
  expect(api.deleteStudioResult).not.toHaveBeenCalled()
})

async function generate() {
  const user = await openStudio()
  await user.type(
    screen.getByRole('textbox', { name: 'Describe your image' }),
    'A mountain'
  )
  await user.click(screen.getByRole('button', { name: 'Create images' }))
  return user
}

test('completed images fit a landscape canvas with the prompt only below it and still open the full preview', async () => {
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: completed,
    replay: false,
  })
  const user = await generate()
  const preview = await screen.findByRole('button', { name: 'Preview' })
  expect(preview).toHaveClass(
    'studio-result-frame',
    'aspect-[4/3]',
    'flex-none'
  )
  expect(within(preview).getByRole('img', { name: 'A mountain' })).toHaveClass(
    'object-contain'
  )
  const gallery = within(
    screen.getByRole('region', { name: 'Generation results' })
  )
  expect(gallery.getAllByText('A mountain', { exact: true })).toHaveLength(1)
  expect(
    preview.compareDocumentPosition(
      gallery.getByText('A mountain', { exact: true })
    ) & Node.DOCUMENT_POSITION_FOLLOWING
  ).toBeTruthy()
  await user.click(preview)
  expect(
    within(screen.getByRole('dialog', { name: 'Generated image' })).getByRole(
      'img',
      { name: 'A mountain' }
    )
  ).toHaveAttribute('src', completed.results[0].preview_url)
})

test('asset saving explains automatic history storage and is optional for using images in video generation', async () => {
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: completed,
    replay: false,
  })
  const user = await generate()
  const explanation =
    'Images are saved in the generation records on the right for 30 days. Download them before expiry. Save to the asset library only for direct use in video generation; library copies are kept separately.'
  const save = await screen.findByRole('button', {
    name: 'Save to asset library (for video generation)',
  })
  expect(save).toHaveAccessibleDescription(explanation)
  expect(screen.getByText(explanation)).toBeVisible()
  expect(api.saveStudioResult).not.toHaveBeenCalled()
  await user.click(save)
  const dialog = screen.getByRole('dialog', {
    name: 'Save to asset library (for video generation)',
  })
  expect(dialog).toHaveAccessibleDescription(explanation)
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  expect(api.saveStudioResult).not.toHaveBeenCalled()
})

test('prompt optimization identifies and calls the chosen text model separately from image generation', async () => {
  vi.mocked(api.listStudioModels).mockImplementation(async (_id, kind) =>
    kind === 'text' ? ['text-model', 'chosen-text-model'] : ['dall-e-3']
  )
  vi.mocked(api.assistStudioPrompt).mockResolvedValue({
    suggested_prompt: 'A mountain in soft light',
  })
  const user = await openStudio()
  await user.click(screen.getByRole('button', { name: 'Model settings' }))
  const textModels = within(screen.getByRole('group', { name: 'Text model' }))
  await user.selectOptions(
    textModels.getByRole('combobox', { name: 'Model' }),
    'chosen-text-model'
  )
  await user.click(screen.getByRole('button', { name: 'Done' }))
  expect(
    await screen.findByText('Text model: chosen-text-model')
  ).toBeInTheDocument()
  const prompt = screen.getByRole('textbox', { name: 'Describe your image' })
  await user.type(prompt, 'A mountain')
  await user.click(screen.getByRole('button', { name: 'Optimize prompt' }))
  await waitFor(() => expect(prompt).toHaveValue('A mountain in soft light'))
  expect(api.assistStudioPrompt).toHaveBeenCalledWith({
    token_id: 1,
    model: 'chosen-text-model',
    prompt: 'A mountain',
  })
  expect(screen.getByText('dall-e-3', { exact: true })).toBeInTheDocument()
  expect(api.createStudioRun).not.toHaveBeenCalled()
})

test('the dashboard link leaves the studio', async () => {
  const user = await openStudio()
  await user.click(screen.getByRole('link', { name: 'Back to Dashboard' }))
  expect(await screen.findByText('Dashboard page')).toBeVisible()
})

test('collapsing and expanding the editor preserves the prompt and exposes its current state', async () => {
  const user = await openStudio()
  await user.type(
    screen.getByRole('textbox', { name: 'Describe your image' }),
    'Keep this draft'
  )
  await user.click(
    screen.getByRole('button', { name: 'Collapse prompt editor' })
  )
  expect(
    screen.queryByRole('textbox', { name: 'Describe your image' })
  ).not.toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Show prompt editor' })
  ).toHaveAttribute('aria-expanded', 'false')
  expect(screen.getByText('Keep this draft')).toBeVisible()
  await user.click(screen.getByRole('button', { name: 'Show prompt editor' }))
  expect(
    screen.getByRole('textbox', { name: 'Describe your image' })
  ).toHaveValue('Keep this draft')
  await user.click(screen.getByRole('button', { name: 'Expand prompt editor' }))
  expect(
    screen.getByRole('button', { name: 'Restore prompt editor' })
  ).toHaveAttribute('aria-pressed', 'true')
  await user.click(
    screen.getByRole('button', { name: 'Restore prompt editor' })
  )
  expect(
    screen.getByRole('textbox', { name: 'Describe your image' })
  ).toHaveValue('Keep this draft')
})

test('file uploads and dropped images become removable references, block generation during upload and send only retained object IDs', async () => {
  let finishUpload!: (reference: StudioReference) => void
  vi.mocked(api.uploadStudioReference)
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finishUpload = resolve
        })
    )
    .mockResolvedValueOnce({
      id: 'ref-2',
      url: 'https://example.test/second.png',
      expires_at: 2000000000,
    })
    .mockResolvedValueOnce({
      id: 'ref-3',
      url: 'https://example.test/third.png',
      expires_at: 2000000000,
    })
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: completed,
    replay: false,
  })
  const user = await openStudio()
  const file = new File(['image'], 'reference.png', { type: 'image/png' })
  await user.type(
    screen.getByRole('textbox', { name: 'Describe your image' }),
    'Use the reference'
  )
  await user.upload(screen.getByLabelText('Upload reference images'), file)
  expect(await screen.findByText('Uploading...')).toBeVisible()
  expect(screen.getByRole('button', { name: 'Create images' })).toBeDisabled()
  await act(() =>
    finishUpload({
      id: 'ref-1',
      url: 'https://example.test/first.png',
      expires_at: 2000000000,
    })
  )
  expect(await screen.findByAltText('Reference image 1')).toBeVisible()
  const editor = screen.getByRole('form', { name: 'Image prompt editor' })
  fireEvent.drop(editor, { dataTransfer: { files: [file], getData: () => '' } })
  expect(await screen.findByAltText('Reference image 2')).toBeVisible()
  fireEvent.drop(editor, {
    dataTransfer: {
      files: [],
      getData: () => 'https://example.test/source.png',
    },
  })
  expect(await screen.findByAltText('Reference image 3')).toBeVisible()
  expect(api.uploadStudioReference).toHaveBeenNthCalledWith(1, file)
  expect(api.uploadStudioReference).toHaveBeenNthCalledWith(2, file)
  expect(api.uploadStudioReference).toHaveBeenNthCalledWith(
    3,
    'https://example.test/source.png'
  )
  await user.click(
    screen.getByRole('button', { name: 'Remove reference image 3' })
  )
  await user.click(
    screen.getByRole('button', { name: 'Remove reference image 1' })
  )
  expect(api.createStudioRun).not.toHaveBeenCalled()
  await user.click(screen.getByRole('button', { name: 'Create images' }))
  await waitFor(() =>
    expect(api.createStudioRun).toHaveBeenCalledWith({
      token_id: 1,
      idempotency_key: expect.any(String),
      request: {
        model: 'dall-e-3',
        prompt: 'Use the reference',
        reference_ids: ['ref-2'],
      },
    })
  )
  await screen.findByRole('button', { name: 'Preview' })
  await user.click(screen.getByRole('button', { name: 'New creation' }))
  expect(screen.queryByAltText('Reference image 1')).not.toBeInTheDocument()
})

test('invalid image files and batches beyond the reference limit do not upload', async () => {
  const user = userEvent.setup({ applyAccept: false })
  await openStudio()
  const input = screen.getByLabelText('Upload reference images')
  await user.upload(
    input,
    new File(['not an image'], 'file.pdf', { type: 'application/pdf' })
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Reference images must be PNG, JPEG or WebP'
  )
  await user.upload(
    input,
    Array.from(
      { length: 9 },
      (_, index) => new File(['image'], `${index}.png`, { type: 'image/png' })
    )
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'You can add up to 8 reference images.'
  )
  expect(api.uploadStudioReference).not.toHaveBeenCalled()
})

test('a failed submission retries with the same idempotency key and nothing is saved by default', async () => {
  vi.mocked(api.createStudioRun)
    .mockRejectedValueOnce(new Error('network'))
    .mockResolvedValueOnce({ run: completed, replay: false })
  const user = await generate()
  await screen.findByText('network')
  await user.click(screen.getByRole('button', { name: 'Create images' }))
  await screen.findByRole('img', { name: 'A mountain' })
  const calls = vi.mocked(api.createStudioRun).mock.calls
  expect(calls).toHaveLength(2)
  expect(calls[0][0]).toEqual({
    token_id: 1,
    idempotency_key: expect.any(String),
    request: { model: 'dall-e-3', prompt: 'A mountain' },
  })
  expect(calls[1][0].idempotency_key).toBe(calls[0][0].idempotency_key)
  expect(api.saveStudioResult).not.toHaveBeenCalled()
  expect(
    screen.getByRole('button', {
      name: 'Save to asset library (for video generation)',
    })
  ).toBeEnabled()
})

test('a replayed run that is still generating polls until it completes', async () => {
  const running: StudioRun = { ...completed, status: 'running', results: [] }
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: running,
    replay: true,
  })
  vi.mocked(api.getStudioRun).mockResolvedValue(running)
  await generate()
  await screen.findByText('This generation is still in progress')
  vi.mocked(api.getStudioRun).mockResolvedValue(completed)
  await screen.findByRole('img', { name: 'A mountain' }, { timeout: 3000 })
  await waitFor(() =>
    expect(
      screen.queryByText('This generation is still in progress')
    ).not.toBeInTheDocument()
  )
})

test('starting a new creation clears the canvas and draft while keeping existing records', async () => {
  vi.mocked(api.createStudioRun).mockResolvedValue({
    run: completed,
    replay: false,
  })
  vi.mocked(api.listStudioRuns).mockResolvedValue({
    items: [completed],
    total: 1,
  })
  const user = await generate()
  await screen.findByRole('button', { name: 'Preview' })
  await user.click(screen.getByRole('button', { name: 'New creation' }))
  const prompt = screen.getByRole('textbox', { name: 'Describe your image' })
  expect(prompt).toHaveValue('')
  expect(prompt).toHaveFocus()
  await waitFor(() => expect(prompt).toHaveAttribute('aria-invalid', 'false'))
  await user.type(prompt, 'Next composition')
  expect(prompt).toHaveAttribute('aria-invalid', 'false')
  expect(
    screen.queryByRole('button', { name: 'Preview' })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('heading', { name: /Imagine it/ })).toBeVisible()
  expect(screen.getByRole('button', { name: 'Delete image' })).toBeVisible()
  expect(api.deleteStudioRun).not.toHaveBeenCalled()
  expect(api.deleteStudioResult).not.toHaveBeenCalled()
})

test('the records shortcut scrolls to the history panel and moves keyboard focus there', async () => {
  const user = await openStudio()
  await user.click(screen.getByRole('button', { name: 'Generation records' }))
  const panel = document.querySelector('#studio-history')
  expect(panel).toHaveFocus()
  expect(panel).toContainElement(
    screen.getByRole('region', { name: 'Generation records' })
  )
})

test('selecting a record opens its image and returns keyboard focus to the canvas', async () => {
  vi.mocked(api.listStudioRuns).mockResolvedValue({
    items: [completed],
    total: 1,
  })
  vi.mocked(api.getStudioRun).mockResolvedValue(completed)
  const user = await openStudio()
  const history = screen.getByRole('region', { name: 'Generation records' })
  await user.click(within(history).getByRole('button', { name: /A mountain/ }))
  expect(await screen.findByRole('button', { name: 'Preview' })).toBeVisible()
  expect(
    screen.getByRole('group', { name: 'Generation results' })
  ).toHaveFocus()
  expect(
    within(history).getByRole('button', { name: /A mountain/ })
  ).toHaveAttribute('aria-pressed', 'true')
})

test('dragging a historical thumbnail uses its full-resolution original as the reference', async () => {
  vi.mocked(api.listStudioRuns).mockResolvedValue({
    items: [completed],
    total: 1,
  })
  await openStudio()
  const history = screen.getByRole('region', { name: 'Generation records' })
  const setData = vi.fn()
  fireEvent.dragStart(
    within(history).getByRole('img', { name: 'Generated image' }),
    { dataTransfer: { setData } }
  )
  expect(setData).toHaveBeenCalledWith(
    'text/uri-list',
    'https://example.test/original.png'
  )
})

test('Ctrl Enter generates from the editor and prevents another submission while pending', async () => {
  let finish: (value: { run: StudioRun; replay: boolean }) => void = () =>
    undefined
  vi.mocked(api.createStudioRun).mockReturnValue(
    new Promise((resolve) => {
      finish = resolve
    })
  )
  const user = await openStudio()
  const prompt = screen.getByRole('textbox', { name: 'Describe your image' })
  await user.type(prompt, 'A mountain')
  await user.keyboard('{Control>}{Enter}{/Control}')
  await waitFor(() => expect(api.createStudioRun).toHaveBeenCalledTimes(1))
  expect(screen.getByRole('button', { name: 'New creation' })).toBeDisabled()
  expect(screen.getByRole('button', { name: /Generating/ })).toBeDisabled()
  expect(prompt).toBeDisabled()
  await act(async () => finish({ run: completed, replay: false }))
  expect(await screen.findByRole('button', { name: 'Preview' })).toBeVisible()
  expect(screen.getByRole('button', { name: 'Create images' })).toBeEnabled()
})
