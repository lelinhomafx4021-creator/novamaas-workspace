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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type Rule = { keyword: string; protocol: string }
type EditableRule = Rule & { id: string }

const withId = (rule: Rule): EditableRule => ({
  ...rule,
  id: crypto.randomUUID(),
})

const PROTOCOLS = [
  { value: 'seedance', label: 'Seedance (content with roles)' },
  { value: 'wan3', label: 'Wan 3 (input.media + parameters)' },
  { value: 'openai_video', label: 'OpenAI-compatible (seconds + metadata)' },
]

function parseRules(raw: string): Rule[] {
  try {
    const rules = JSON.parse(raw)
    return Array.isArray(rules) ? rules : []
  } catch {
    return []
  }
}

export function VideoStudioRulesCard({ value }: { value: string }) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [rules, setRules] = useState<EditableRule[]>(() =>
    parseRules(value).map(withId)
  )

  useEffect(() => setRules(parseRules(value).map(withId)), [value])

  const update = (id: string, patch: Partial<Rule>) =>
    setRules((current) =>
      current.map((rule) => (rule.id === id ? { ...rule, ...patch } : rule))
    )

  const save = () =>
    updateOption.mutateAsync({
      key: 'video_studio.model_rules',
      value: JSON.stringify(
        rules.map(({ keyword, protocol }) => ({ keyword, protocol }))
      ),
    })

  return (
    <SettingsSection title={t('Video Studio Models')}>
      <SettingsPageFormActions
        onSave={save}
        isSaving={updateOption.isPending}
      />
      <p className='text-muted-foreground text-sm'>
        {t(
          'A model appears in the video studio when its name contains a keyword below. Rules are checked in order and the first match decides the request format.'
        )}
      </p>
      <div className='space-y-2'>
        {rules.map((rule) => (
          <div className='flex items-center gap-2' key={rule.id}>
            <Input
              className='max-w-48'
              value={rule.keyword}
              placeholder='seedance'
              onChange={(event) =>
                update(rule.id, { keyword: event.target.value })
              }
            />
            <NativeSelect
              value={rule.protocol}
              onChange={(event) =>
                update(rule.id, { protocol: event.target.value })
              }
            >
              {PROTOCOLS.map((protocol) => (
                <NativeSelectOption key={protocol.value} value={protocol.value}>
                  {t(protocol.label)}
                </NativeSelectOption>
              ))}
            </NativeSelect>
            <Button
              type='button'
              size='icon'
              variant='ghost'
              aria-label={t('Remove')}
              onClick={() =>
                setRules(rules.filter((item) => item.id !== rule.id))
              }
            >
              <Trash2 />
            </Button>
          </div>
        ))}
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            setRules([...rules, withId({ keyword: '', protocol: 'seedance' })])
          }
        >
          <Plus />
          {t('Add rule')}
        </Button>
      </div>
    </SettingsSection>
  )
}
