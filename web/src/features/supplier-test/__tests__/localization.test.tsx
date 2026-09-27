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
import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { expect, test } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'

import { CheckTable } from '../components/check-table'
import {
  buildVideoHtmlReport,
  buildVideoMarkdownReport,
  type VideoReportInput,
} from '../report'
import type { CheckResult } from '../types'

const submittedCheck: CheckResult = {
  id: 'video_submit',
  title: 'Task submit',
  status: 'pass',
  message: 'Task submitted. Task ID: {{taskId}}',
  messageArgs: { taskId: 'video-42' },
}

test('video check title and interpolated detail follow a language change', async () => {
  const i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en, fr },
  })

  render(
    <I18nextProvider i18n={i18n}>
      <CheckTable checks={[submittedCheck]} busy={false} />
    </I18nextProvider>
  )

  expect(screen.getByText('Task submitted. Task ID: video-42')).toBeVisible()
  await act(async () => {
    await i18n.changeLanguage('fr')
  })
  expect(screen.getByText('Envoi de la tâche')).toBeVisible()
  expect(screen.getByText('Tâche envoyée. ID : video-42')).toBeVisible()
})

test('video Markdown and HTML reports translate check details in the selected language', async () => {
  const i18n = createInstance()
  await i18n.init({ lng: 'fr', resources: { fr } })
  const input: VideoReportInput = {
    language: 'fr',
    baseUrl: 'https://supplier.example',
    model: 'video-model',
    elapsedMs: 1000,
    videoConfig: {
      prompt: 'A sunset',
      hasImage: false,
      uploadMode: 'url',
      hasLastFrame: false,
    },
    videoChecks: [submittedCheck],
    t: (key, options) => i18n.t(key, options),
  }

  const markdown = buildVideoMarkdownReport(input)
  const html = buildVideoHtmlReport(input)

  expect(markdown).toContain('Tâche envoyée. ID : video-42')
  expect(markdown).toContain('Envoi de la tâche: Réussi')
  expect(html).toContain('Tâche envoyée. ID : video-42')
  expect(html).toContain('Réussi')
  expect(html).toContain('<html lang="fr">')
})
