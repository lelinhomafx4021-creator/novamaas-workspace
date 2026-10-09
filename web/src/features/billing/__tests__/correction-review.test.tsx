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
import { fireEvent, render, screen, within } from '@testing-library/react'
import i18next, { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next, setI18n } from 'react-i18next'
import { afterEach, beforeEach, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { CorrectionReview } from '../components/correction-review'
import type { CorrectionBatch, CorrectionRow } from '../correction-api'

const batch: CorrectionBatch = {
  id: 'review-preview',
  user_id: 4,
  created_by: 1,
  created_at: 100,
  start_at: 100,
  end_at: 200,
  reason: 'Review current model pricing',
  audit_logged_at: 0,
  reversal_audit_logged_at: 0,
  models: '["doubao-seedance-2-5"]',
  mode: 'model_pricing',
  target_group: '',
  target_rate: '',
  status: 'preview',
  can_apply: true,
  net_delta: 500000,
  charge_delta: 500000,
  refund_delta: 0,
  sha256: 'review-digest',
  expires_at: 9999999999,
  rows: [],
}

const originalCurrency = useSystemConfigStore.getState().config.currency
beforeEach(() => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      ...DEFAULT_CURRENCY_CONFIG,
      quotaDisplayType: 'CNY',
      usdExchangeRate: 7,
    },
  })
})
afterEach(() => {
  useSystemConfigStore.getState().setConfig({ currency: originalCurrency })
  setI18n(i18next)
})

test.each([
  { net: 500000, title: 'Additional wallet charge', amount: '¥7' },
  { net: -500000, title: 'Wallet refund', amount: '¥7' },
  { net: 0, title: 'Wallet balance unchanged', amount: '' },
])(
  'net $net clearly identifies $title without requiring sign interpretation',
  (example) => {
    render(<CorrectionReview batch={{ ...batch, net_delta: example.net }} />)
    const notice = screen.getByRole('status', { name: 'Wallet balance change' })
    expect(within(notice).getByText(example.title)).toBeVisible()
    if (example.amount) expect(notice).toHaveTextContent(example.amount)
    expect(notice).not.toHaveTextContent('-¥')
    expect(notice).toHaveTextContent('after confirmation')
  }
)

test('a reversed refund identifies the completed opposite wallet charge', () => {
  render(
    <CorrectionReview
      batch={{ ...batch, status: 'reversed', net_delta: -500000 }}
    />
  )
  const notice = screen.getByRole('status', { name: 'Wallet balance change' })
  expect(notice).toHaveTextContent('Additional wallet charge')
  expect(notice).toHaveTextContent('¥7 was deducted from the wallet.')
  expect(notice).not.toHaveTextContent('after confirmation')
})

function pricedRow(
  id: number,
  resolution: string,
  hasVideo: boolean,
  videoRatio: number
): CorrectionRow {
  return {
    source_entry_id: id,
    model_name: 'doubao-seedance-2-5',
    posted_at: 100 + id,
    original_group: 'default',
    original_rate: '1',
    original_quota: 100000,
    effective_quota: 100000,
    corrected_quota: 110000,
    delta: 10000,
    blocked: '',
    target_pricing: JSON.stringify({
      pricing_mode: 'tokens',
      per_call_billing: false,
      model_price: -1,
      model_ratio: 5,
      total_tokens: 12345,
      resolution,
      has_video: hasVideo,
      other_ratios: { video_input: videoRatio },
    }),
  }
}

test.each([
  {
    language: 'zhCN',
    tokens: '12,345',
    model: '5.5x',
    video: '1.1x',
    other: '1.25x',
  },
  {
    language: 'zhTW',
    tokens: '12,345',
    model: '5.5x',
    video: '1.1x',
    other: '1.25x',
  },
  {
    language: 'fr',
    tokens: '12 345',
    model: '5,5x',
    video: '1,1x',
    other: '1,25x',
  },
])(
  '$language previews render token usage and all pricing multipliers without a locale error',
  async (example) => {
    const localized = createInstance()
    await localized.use(initReactI18next).init({
      lng: example.language,
      fallbackLng: 'en',
      resources: {
        [example.language]: { translation: { Model: 'Model' } },
        en: { translation: { Model: 'Model' } },
      },
    })
    expect(localized.resolvedLanguage).toBe(example.language)
    const row = pricedRow(10, '1080p', false, 1.1)
    render(
      <I18nextProvider i18n={localized}>
        <CorrectionReview
          batch={{
            ...batch,
            rows: [
              {
                ...row,
                target_pricing: JSON.stringify({
                  ...JSON.parse(row.target_pricing ?? '{}'),
                  model_ratio: 5.5,
                  other_ratios: { video_input: 1.1, seconds: 1.25 },
                }),
              },
            ],
          }}
        />
      </I18nextProvider>
    )
    const table = screen.getByRole('table', { name: 'Adjustment records' })
    expect(within(table).getByText(example.tokens)).toBeVisible()
    expect(within(table).getByText(example.model)).toBeVisible()
    expect(within(table).getByText(example.video)).toBeVisible()
    expect(within(table).getByText(example.other)).toBeVisible()
    expect(within(table).getByText('1080p')).toBeVisible()
    expect(
      screen.getByRole('status', { name: 'Wallet balance change' })
    ).toHaveTextContent('Additional wallet charge')
  }
)

test('Seedance rows distinguish resolution and video input with their current unit prices', () => {
  render(
    <CorrectionReview
      batch={{
        ...batch,
        rows: [
          pricedRow(10, '720p', false, 1),
          pricedRow(11, '1080p', false, 77 / 70),
          pricedRow(12, '720p', true, 42 / 70),
          pricedRow(13, '1080p', true, 46 / 70),
        ],
      }}
    />
  )
  const table = screen.getByRole('table', { name: 'Adjustment records' })
  expect(
    within(table).getByRole('columnheader', { name: 'Pricing evidence' })
  ).toBeVisible()
  for (const example of [
    { id: 10, resolution: '720p', video: 'No', price: '¥70' },
    { id: 11, resolution: '1080p', video: 'No', price: '¥77' },
    { id: 12, resolution: '720p', video: 'Yes', price: '¥42' },
    { id: 13, resolution: '1080p', video: 'Yes', price: '¥46' },
  ]) {
    const row = within(table).getByText(`#${example.id}`).closest('tr')
    if (!row) throw new Error('Expected a pricing evidence row')
    expect(within(row).getByText(example.resolution)).toBeVisible()
    expect(within(row).getByText(example.video)).toBeVisible()
    expect(within(row).getByText('12,345')).toBeVisible()
    expect(within(row).getByText(example.price)).toBeVisible()
    expect(within(row).getByText('Unit price per million tokens')).toBeVisible()
  }
  expect(table.parentElement).toHaveClass('overflow-x-auto')
  expect(table).toHaveClass('min-w-[72rem]')
})

test('unavailable or malformed pricing evidence stays explicit without guessing a resolution', () => {
  render(
    <CorrectionReview
      batch={{
        ...batch,
        rows: [
          { ...pricedRow(10, '720p', false, 1), target_pricing: undefined },
          { ...pricedRow(11, '1080p', false, 1.1), target_pricing: '{broken' },
        ],
      }}
    />
  )
  expect(screen.getAllByText('Pricing evidence unavailable')).toHaveLength(2)
  expect(screen.queryByText('720p')).not.toBeInTheDocument()
  expect(screen.queryByText('1080p')).not.toBeInTheDocument()
})

test.each([
  {
    description: 'a negative token price other than the sentinel',
    pricing: { model_price: -2 },
  },
  {
    description: 'a negative per-call price',
    pricing: { pricing_mode: 'per_call', model_price: -1 },
  },
  { description: 'a negative model multiplier', pricing: { model_ratio: -1 } },
  { description: 'negative usage', pricing: { total_tokens: -1 } },
  {
    description: 'a negative input multiplier',
    pricing: { other_ratios: { video_input: -1 } },
  },
])(
  '$description is rejected rather than presented as valid pricing evidence',
  (example) => {
    const row = pricedRow(10, '720p', false, 1)
    render(
      <CorrectionReview
        batch={{
          ...batch,
          rows: [
            {
              ...row,
              target_pricing: JSON.stringify({
                ...JSON.parse(row.target_pricing ?? '{}'),
                ...example.pricing,
              }),
            },
          ],
        }}
      />
    )
    expect(screen.getByText('Pricing evidence unavailable')).toBeVisible()
    expect(screen.queryByText('720p')).not.toBeInTheDocument()
  }
)

test('an upstream resolution stays distinct from an automatic archived request', () => {
  const row = pricedRow(10, '720p', false, 1)
  render(
    <CorrectionReview
      batch={{
        ...batch,
        rows: [
          {
            ...row,
            target_pricing: JSON.stringify({
              ...JSON.parse(row.target_pricing ?? '{}'),
              resolution_source: 'upstream_response',
            }),
          },
        ],
      }}
    />
  )
  expect(screen.getByText('720p')).toBeVisible()
  expect(screen.getByText('Automatic')).toBeVisible()
  expect(screen.getByText('Upstream task response')).toBeVisible()
  expect(screen.getByText('Model ratio')).toBeVisible()
  expect(screen.getByText('5x')).toBeVisible()
})

test('blocked negative previews describe a future refund and keep the blocked notice', () => {
  render(
    <CorrectionReview
      batch={{ ...batch, net_delta: -500000, can_apply: false }}
    />
  )
  const notice = screen.getByRole('status', { name: 'Wallet balance change' })
  expect(notice).toHaveTextContent(
    '¥7 will be returned to the wallet after confirmation.'
  )
  expect(notice).not.toHaveTextContent('was returned')
  expect(screen.getByRole('alert')).toHaveTextContent('prevent execution')
})

test('pricing evidence remains visible on later preview pages', () => {
  render(
    <CorrectionReview
      batch={{
        ...batch,
        rows: Array.from({ length: 51 }, (_, index) =>
          pricedRow(
            index + 1,
            index === 50 ? '1080p' : '720p',
            false,
            index === 50 ? 1.1 : 1
          )
        ),
      }}
    />
  )
  expect(screen.queryByText('#51')).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Next' }))
  const row = screen.getByText('#51').closest('tr')
  if (!row) throw new Error('Expected the later-page evidence row')
  expect(within(row).getByText('1080p')).toBeVisible()
  expect(within(row).getByText('¥77')).toBeVisible()
})
