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
import { useMutation } from '@tanstack/react-query'
import {
  Activity,
  ClipboardCheck,
  Copy,
  Database,
  Download,
  Loader2,
  Square,
  Zap,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { PasswordInput } from '@/components/password-input'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Progress } from '@/components/ui/progress'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Textarea } from '@/components/ui/textarea'
import { TitledCard } from '@/components/ui/titled-card'

import { fetchSupplierModels } from './api'
import {
  CACHE_CORPORA,
  CACHE_ROUND_PRESETS,
  CACHE_WAIT_PRESETS,
  CACHE_WARM_TOKEN_PRESETS,
  DEFAULT_BASIC_FORM,
  DEFAULT_CACHE_FORM,
  DEFAULT_STRESS_FORM,
  MAX_CACHE_ROUNDS,
  MAX_CACHE_WAIT_SECONDS,
  MAX_CONCURRENCY,
  MAX_ROUNDS,
  MAX_TOKENS_CAP,
  STRESS_CORPORA,
  STRESS_PRESETS,
  STRESS_WARN_TOTAL,
  TARGET_TOKEN_PRESETS,
  resolveStressPrompt,
} from './constants'
import { useSupplierTestRun } from './hooks/use-supplier-test-run'
import type {
  BasicForm,
  CacheForm,
  CheckResult,
  CheckStatus,
  StressForm,
  SupplierTestModule,
  SupplierTestRunRequest,
  TargetForm,
} from './types'

function statusVariant(status: CheckStatus): StatusVariant {
  if (status === 'pass') return 'success'
  if (status === 'fail') return 'danger'
  if (status === 'skip') return 'warning'
  if (status === 'running') return 'info'
  return 'neutral'
}

function formatMs(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '-'
  return `${value.toFixed(0)} ms`
}

function formatRate(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}

function axiosErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object' && 'response' in error) {
    const message = (
      error as { response?: { data?: { message?: string } } }
    ).response?.data?.message
    if (message) return message
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}

function optionalNumber(raw: string): number | undefined {
  const trimmed = raw.trim()
  if (trimmed === '') return undefined
  const value = Number(trimmed)
  if (!Number.isFinite(value)) return undefined
  return value
}

export function SupplierTest() {
  const { t } = useTranslation()
  const [target, setTarget] = useState<TargetForm>({
    baseUrl: '',
    apiKey: '',
    model: '',
  })
  const [models, setModels] = useState<string[]>([])
  const [basic, setBasic] = useState<BasicForm>(DEFAULT_BASIC_FORM)
  const [cache, setCache] = useState<CacheForm>(DEFAULT_CACHE_FORM)
  const [stress, setStress] = useState<StressForm>(DEFAULT_STRESS_FORM)
  const run = useSupplierTestRun()
  const stressTotal = stress.concurrency * stress.rounds
  const busy = run.runningModule !== null
  const hasReport =
    Boolean(run.summaries.basic || run.summaries.cache || run.summaries.stress) ||
    run.basicChecks.some((check) => check.status !== 'idle') ||
    run.cacheChecks.some((check) => check.status !== 'idle') ||
    run.metrics !== null
  const progressValue =
    run.progress.total > 0
      ? Math.min(100, (run.progress.completed / run.progress.total) * 100)
      : 0

  const modelsMutation = useMutation({
    mutationFn: fetchSupplierModels,
    onSuccess: (ids) => {
      setModels(ids)
      if (ids.length > 0 && !target.model.trim()) {
        setTarget((current) => ({ ...current, model: ids[0] ?? '' }))
      }
      toast.success(t('Fetched {{count}} models', { count: ids.length }))
    },
    onError: (error) => {
      toast.error(axiosErrorMessage(error, t('Failed to fetch models')))
    },
  })

  const validateTarget = (): boolean => {
    if (!target.baseUrl.trim()) {
      toast.error(t('Enter a base URL first'))
      return false
    }
    if (!target.model.trim()) {
      toast.error(t('Enter a model ID first'))
      return false
    }
    return true
  }

  const buildPayload = (
    module: SupplierTestModule,
    checks?: string[]
  ): SupplierTestRunRequest => {
    const temperature = optionalNumber(basic.temperature)
    const topP = optionalNumber(basic.topP)
    return {
      base_url: target.baseUrl.trim(),
      api_key: target.apiKey,
      model: target.model.trim(),
      modules: [module],
      basic: {
        prompt: basic.prompt,
        max_tokens: basic.maxTokens,
        stream: basic.stream,
        ...(temperature === undefined ? {} : { temperature }),
        ...(topP === undefined ? {} : { top_p: topP }),
        ...(checks && checks.length > 0 ? { checks } : {}),
      },
      cache: {
        prompt: cache.prompt,
        follow_up: cache.followUp,
        warm_tokens: cache.warmTokens,
        wait_seconds: cache.waitSeconds,
        max_tokens: cache.maxTokens,
        rounds: cache.rounds,
        stream: cache.stream,
      },
      stress: {
        concurrency: stress.concurrency,
        rounds: stress.rounds,
        max_tokens: stress.maxTokens,
        prompt: resolveStressPrompt(stress),
        target_tokens: stress.targetTokens > 0 ? stress.targetTokens : undefined,
        stream: stress.stream,
      },
    }
  }

  const startModule = (module: SupplierTestModule, checks?: string[]) => {
    if (!validateTarget()) return
    if (module === 'basic') {
      if (
        !Number.isInteger(basic.maxTokens) ||
        basic.maxTokens < 1 ||
        basic.maxTokens > MAX_TOKENS_CAP
      ) {
        toast.error(
          t('Max tokens must be between 1 and {{max}}', { max: MAX_TOKENS_CAP })
        )
        return
      }
    }
    if (module === 'cache') {
      if (cache.warmTokens < 0 || cache.warmTokens > MAX_TOKENS_CAP) {
        toast.error(
          t('Cache prefix tokens must be between 0 and {{max}}', {
            max: MAX_TOKENS_CAP,
          })
        )
        return
      }
      if (cache.waitSeconds < 0 || cache.waitSeconds > MAX_CACHE_WAIT_SECONDS) {
        toast.error(
          t('Cache wait must be between 0 and {{max}} seconds', {
            max: MAX_CACHE_WAIT_SECONDS,
          })
        )
        return
      }
      if (
        !Number.isInteger(cache.rounds) ||
        cache.rounds < 1 ||
        cache.rounds > MAX_CACHE_ROUNDS
      ) {
        toast.error(
          t('Cache rounds must be between 1 and {{max}}', {
            max: MAX_CACHE_ROUNDS,
          })
        )
        return
      }
      if (
        !Number.isInteger(cache.maxTokens) ||
        cache.maxTokens < 1 ||
        cache.maxTokens > MAX_TOKENS_CAP
      ) {
        toast.error(
          t('Cache max tokens must be between 1 and {{max}}', {
            max: MAX_TOKENS_CAP,
          })
        )
        return
      }
    }
    if (module === 'stress') {
      if (
        !Number.isInteger(stress.concurrency) ||
        stress.concurrency < 1 ||
        stress.concurrency > MAX_CONCURRENCY
      ) {
        toast.error(
          t('Concurrency must be between 1 and {{max}}', {
            max: MAX_CONCURRENCY,
          })
        )
        return
      }
      if (
        !Number.isInteger(stress.rounds) ||
        stress.rounds < 1 ||
        stress.rounds > MAX_ROUNDS
      ) {
        toast.error(t('Rounds must be between 1 and {{max}}', { max: MAX_ROUNDS }))
        return
      }
      if (
        !Number.isInteger(stress.maxTokens) ||
        stress.maxTokens < 1 ||
        stress.maxTokens > MAX_TOKENS_CAP
      ) {
        toast.error(
          t('Max tokens must be between 1 and {{max}}', { max: MAX_TOKENS_CAP })
        )
        return
      }
      if (stress.targetTokens < 0 || stress.targetTokens > MAX_TOKENS_CAP) {
        toast.error(
          t('Target tokens must be between 0 and {{max}}', {
            max: MAX_TOKENS_CAP,
          })
        )
        return
      }
    }
    void run.start(buildPayload(module, checks))
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Supplier Test')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {busy ? (
          <Button variant='outline' onClick={run.stop}>
            <Square />
            {t('Stop')}
          </Button>
        ) : null}
        <Button
          variant='outline'
          disabled={busy || !hasReport}
          onClick={async () => {
            const report = buildTestReport({
              baseUrl: target.baseUrl.trim(),
              model: target.model.trim(),
              basicChecks: run.basicChecks,
              cacheChecks: run.cacheChecks,
              summaries: run.summaries,
              metrics: run.metrics,
              errorMessage: run.errorMessage,
              statusLabel,
              t,
            })
            try {
              await navigator.clipboard.writeText(report)
              toast.success(t('Report copied to clipboard'))
            } catch {
              toast.error(t('Failed to copy report'))
            }
          }}
        >
          <Copy />
          {t('Copy report')}
        </Button>
        <Button
          variant='outline'
          disabled={busy || !hasReport}
          onClick={() => {
            downloadTextFile(
              `supplier-test-${stampFileName()}.txt`,
              buildTestReport({
                baseUrl: target.baseUrl.trim(),
                model: target.model.trim(),
                basicChecks: run.basicChecks,
                cacheChecks: run.cacheChecks,
                summaries: run.summaries,
                metrics: run.metrics,
                errorMessage: run.errorMessage,
                statusLabel,
                t,
              })
            )
            toast.success(t('Report exported'))
          }}
        >
          <Download />
          {t('Export report')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <TitledCard
            title={t('Target')}
            description={t(
              'Paste Base URL. Type the model ID. Fetching /v1/models and API key are optional.'
            )}
            icon={<ClipboardCheck />}
            action={
              <Button
                variant='outline'
                onClick={() => {
                  if (!target.baseUrl.trim()) {
                    toast.error(t('Enter a base URL first'))
                    return
                  }
                  modelsMutation.mutate({
                    base_url: target.baseUrl.trim(),
                    api_key: target.apiKey,
                  })
                }}
                disabled={busy || modelsMutation.isPending}
              >
                {modelsMutation.isPending ? (
                  <Loader2 className='animate-spin' />
                ) : null}
                {t('Fetch models')}
              </Button>
            }
          >
            <div className='grid gap-4 md:grid-cols-3'>
              <div className='space-y-2'>
                <Label htmlFor='supplier-base-url'>{t('Base URL')}</Label>
                <Input
                  id='supplier-base-url'
                  placeholder='https://api.example.com'
                  value={target.baseUrl}
                  disabled={busy}
                  onChange={(event) =>
                    setTarget((current) => ({
                      ...current,
                      baseUrl: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='supplier-api-key'>{t('API Key')}</Label>
                <PasswordInput
                  id='supplier-api-key'
                  placeholder={t('Supplier API key')}
                  value={target.apiKey}
                  disabled={busy}
                  onChange={(event) =>
                    setTarget((current) => ({
                      ...current,
                      apiKey: event.target.value,
                    }))
                  }
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='supplier-model'>{t('Model')}</Label>
                <Input
                  id='supplier-model'
                  list='supplier-model-options'
                  placeholder={t('Type a model ID')}
                  value={target.model}
                  disabled={busy}
                  onChange={(event) =>
                    setTarget((current) => ({
                      ...current,
                      model: event.target.value,
                    }))
                  }
                />
                <datalist id='supplier-model-options'>
                  {models.map((id) => (
                    <option key={id} value={id} />
                  ))}
                </datalist>
              </div>
            </div>
          </TitledCard>

          <TitledCard
            title={t('Basic acceptance')}
            description={t(
              'Run one check or the full suite. Empty temperature / top_p are omitted. Missing vendor fields are skipped, not failed.'
            )}
            icon={<ClipboardCheck />}
            action={
              <Button onClick={() => startModule('basic')} disabled={busy}>
                {run.runningModule === 'basic' ? (
                  <Loader2 className='animate-spin' />
                ) : null}
                {t('Run all basic checks')}
              </Button>
            }
          >
            <div className='grid gap-4 md:grid-cols-4'>
              <NumberField
                id='basic-max-tokens'
                label={t('Max tokens')}
                value={basic.maxTokens}
                disabled={busy}
                min={1}
                max={MAX_TOKENS_CAP}
                presets={[
                  { label: '64', value: 64 },
                  { label: '256', value: 256 },
                  { label: '1k', value: 1024 },
                  { label: '4k', value: 4096 },
                ]}
                onChange={(value) =>
                  setBasic((current) => ({ ...current, maxTokens: value }))
                }
              />
              <OptionalNumberField
                id='basic-temperature'
                label={t('Temperature')}
                value={basic.temperature}
                disabled={busy}
                min={0}
                max={2}
                step={0.1}
                placeholder={t('Leave empty to omit')}
                onChange={(value) =>
                  setBasic((current) => ({ ...current, temperature: value }))
                }
              />
              <OptionalNumberField
                id='basic-top-p'
                label={t('Top P')}
                value={basic.topP}
                disabled={busy}
                min={0}
                max={1}
                step={0.05}
                placeholder={t('Leave empty to omit')}
                onChange={(value) =>
                  setBasic((current) => ({ ...current, topP: value }))
                }
              />
              <StreamSwitch
                id='basic-stream'
                checked={basic.stream}
                disabled={busy}
                onChange={(checked) =>
                  setBasic((current) => ({ ...current, stream: checked }))
                }
              />
            </div>
            <div className='mt-4 space-y-2'>
              <Label htmlFor='basic-prompt'>{t('Prompt')}</Label>
              <Textarea
                id='basic-prompt'
                rows={3}
                value={basic.prompt}
                disabled={busy}
                onChange={(event) =>
                  setBasic((current) => ({
                    ...current,
                    prompt: event.target.value,
                  }))
                }
              />
            </div>
            {run.runningModule === 'basic' && run.streamText ? (
              <pre className='bg-muted mt-4 max-h-48 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
                {run.streamText}
              </pre>
            ) : null}
            {run.summaries.basic ? (
              <p className='text-muted-foreground mt-4 text-sm'>
                {run.summaries.basic}
              </p>
            ) : null}
            <div className='mt-4'>
              <CheckTable
                checks={run.basicChecks}
                busy={busy}
                onRun={(id) => startModule('basic', [id])}
              />
            </div>
          </TitledCard>

          <TitledCard
            title={t('Cache test')}
            description={t(
              'Pick a size and corpus. The server pads the prefix. Do not paste a huge block here.'
            )}
            icon={<Database />}
            action={
              <Button onClick={() => startModule('cache')} disabled={busy}>
                {run.runningModule === 'cache' ? (
                  <Loader2 className='animate-spin' />
                ) : null}
                {t('Run cache test')}
              </Button>
            }
          >
            <div className='grid gap-4 md:grid-cols-4'>
              <NumberField
                id='cache-warm-tokens'
                label={t('Warm tokens')}
                value={cache.warmTokens}
                disabled={busy}
                min={0}
                max={MAX_TOKENS_CAP}
                presets={CACHE_WARM_TOKEN_PRESETS}
                onChange={(value) =>
                  setCache((current) => ({
                    ...current,
                    warmTokens: value,
                  }))
                }
              />
              <NumberField
                id='cache-wait'
                label={t('Wait seconds')}
                value={cache.waitSeconds}
                disabled={busy}
                min={0}
                max={MAX_CACHE_WAIT_SECONDS}
                presets={CACHE_WAIT_PRESETS.map((s) => ({
                  label: `${s}s`,
                  value: s,
                }))}
                onChange={(value) =>
                  setCache((current) => ({ ...current, waitSeconds: value }))
                }
              />
              <NumberField
                id='cache-rounds'
                label={t('Probe rounds')}
                value={cache.rounds}
                disabled={busy}
                min={1}
                max={MAX_CACHE_ROUNDS}
                presets={CACHE_ROUND_PRESETS.map((r) => ({
                  label: String(r),
                  value: r,
                }))}
                onChange={(value) =>
                  setCache((current) => ({
                    ...current,
                    rounds: value,
                  }))
                }
              />
              <NumberField
                id='cache-max-tokens'
                label={t('Max tokens')}
                value={cache.maxTokens}
                disabled={busy}
                min={1}
                max={MAX_TOKENS_CAP}
                presets={[
                  { label: '16', value: 16 },
                  { label: '64', value: 64 },
                  { label: '256', value: 256 },
                ]}
                onChange={(value) =>
                  setCache((current) => ({ ...current, maxTokens: value }))
                }
              />
            </div>
            <div className='mt-4 grid gap-4 md:grid-cols-2'>
              <FieldSelect
                label={t('Cache corpus')}
                value={cache.corpus}
                disabled={busy}
                items={CACHE_CORPORA.map((item) => ({
                  value: item.id,
                  label: t(item.labelKey),
                }))}
                onChange={(value) => {
                  const corpus = CACHE_CORPORA.find((item) => item.id === value)
                  setCache((current) => ({
                    ...current,
                    corpus: value,
                    prompt:
                      corpus?.id === 'custom'
                        ? current.prompt
                        : (corpus?.prompt ?? ''),
                  }))
                }}
              />
              <StreamSwitch
                id='cache-stream'
                checked={cache.stream}
                disabled={busy}
                onChange={(checked) =>
                  setCache((current) => ({ ...current, stream: checked }))
                }
              />
            </div>
            {cache.corpus === 'custom' ? (
              <div className='mt-4 space-y-2'>
                <Label htmlFor='cache-prompt'>{t('Prefix override')}</Label>
                <Textarea
                  id='cache-prompt'
                  rows={3}
                  placeholder={t(
                    'Short prefix only. The server pads it to the selected size.'
                  )}
                  value={cache.prompt}
                  disabled={busy}
                  onChange={(event) =>
                    setCache((current) => ({
                      ...current,
                      prompt: event.target.value,
                    }))
                  }
                />
              </div>
            ) : null}
            <div className='mt-4 space-y-2'>
              <Label htmlFor='cache-follow-up'>{t('Follow-up question')}</Label>
              <Input
                id='cache-follow-up'
                value={cache.followUp}
                disabled={busy}
                onChange={(event) =>
                  setCache((current) => ({
                    ...current,
                    followUp: event.target.value,
                  }))
                }
              />
            </div>
            {run.summaries.cache ? (
              <p className='text-muted-foreground mt-4 text-sm'>
                {run.summaries.cache}
              </p>
            ) : null}
            <div className='mt-4'>
              <CheckTable checks={run.cacheChecks} busy={busy} />
            </div>
          </TitledCard>

          <TitledCard
            title={t('Stress test')}
            description={t(
              'Pick a built-in corpus, including long text. Replace the files if you need different wording. Stream is optional.'
            )}
            icon={<Zap />}
            action={
              <Button onClick={() => startModule('stress')} disabled={busy}>
                {run.runningModule === 'stress' ? (
                  <Loader2 className='animate-spin' />
                ) : null}
                {t('Run stress test')}
              </Button>
            }
          >
            <div className='flex flex-wrap gap-2'>
              {STRESS_PRESETS.map((preset) => (
                <Button
                  key={preset.id}
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={busy}
                  onClick={() =>
                    setStress((current) => ({
                      ...current,
                      corpus: preset.id,
                      concurrency: preset.concurrency,
                      rounds: preset.rounds,
                      maxTokens: preset.maxTokens,
                      targetTokens: preset.targetTokens ?? 0,
                    }))
                  }
                >
                  {t(preset.labelKey)}
                </Button>
              ))}
            </div>
            <div className='mt-4 grid gap-4 md:grid-cols-4'>
              <NumberField
                id='stress-concurrency'
                label={t('Concurrency')}
                value={stress.concurrency}
                disabled={busy}
                min={1}
                max={MAX_CONCURRENCY}
                presets={[
                  { label: '1', value: 1 },
                  { label: '5', value: 5 },
                  { label: '10', value: 10 },
                  { label: '20', value: 20 },
                  { label: '50', value: 50 },
                ]}
                onChange={(value) =>
                  setStress((current) => ({ ...current, concurrency: value }))
                }
              />
              <NumberField
                id='stress-rounds'
                label={t('Rounds')}
                value={stress.rounds}
                disabled={busy}
                min={1}
                max={MAX_ROUNDS}
                presets={[
                  { label: '1', value: 1 },
                  { label: '2', value: 2 },
                  { label: '5', value: 5 },
                  { label: '10', value: 10 },
                ]}
                onChange={(value) =>
                  setStress((current) => ({ ...current, rounds: value }))
                }
              />
              <NumberField
                id='stress-max-tokens'
                label={t('Max tokens')}
                value={stress.maxTokens}
                disabled={busy}
                min={1}
                max={MAX_TOKENS_CAP}
                presets={[
                  { label: '64', value: 64 },
                  { label: '256', value: 256 },
                  { label: '512', value: 512 },
                  { label: '1k', value: 1024 },
                  { label: '4k', value: 4096 },
                ]}
                onChange={(value) =>
                  setStress((current) => ({ ...current, maxTokens: value }))
                }
              />
              <StreamSwitch
                id='stress-stream'
                checked={stress.stream}
                disabled={busy}
                onChange={(checked) =>
                  setStress((current) => ({ ...current, stream: checked }))
                }
              />
            </div>
            <div className='mt-4 grid gap-4 md:grid-cols-2'>
              <NumberField
                id='stress-target-tokens'
                label={t('Target tokens (prompt padding)')}
                value={stress.targetTokens}
                disabled={busy}
                min={0}
                max={MAX_TOKENS_CAP}
                presets={TARGET_TOKEN_PRESETS}
                onChange={(value) =>
                  setStress((current) => ({
                    ...current,
                    targetTokens: value,
                  }))
                }
              />
              <FieldSelect
                label={t('Stress corpus')}
                value={stress.corpus}
                disabled={busy}
                items={STRESS_CORPORA.map((item) => ({
                  value: item.id,
                  label: t(item.labelKey),
                }))}
                onChange={(value) =>
                  setStress((current) => ({ ...current, corpus: value }))
                }
              />
            </div>
            {stress.corpus === 'custom' ? (
              <div className='mt-4 space-y-2'>
                <Label htmlFor='stress-prompt'>{t('Prompt')}</Label>
                <Textarea
                  id='stress-prompt'
                  rows={3}
                  placeholder={t('Write a short prompt, or pick a built-in corpus')}
                  value={stress.prompt}
                  disabled={busy}
                  onChange={(event) =>
                    setStress((current) => ({
                      ...current,
                      prompt: event.target.value,
                    }))
                  }
                />
              </div>
            ) : (
              <p className='text-muted-foreground mt-4 text-sm'>
                {t(
                  'Using built-in corpus ({{chars}} characters). Replace files in supplier-test/corpora to change the text.',
                  { chars: resolveStressPrompt(stress).length }
                )}
              </p>
            )}
            {stressTotal >= STRESS_WARN_TOTAL ? (
              <Alert className='mt-4'>
                <AlertDescription>
                  {t(
                    'This run will send {{count}} requests. That can consume a large amount of upstream quota.',
                    { count: stressTotal }
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            {run.runningModule === 'stress' && run.progress.total > 0 ? (
              <div className='mt-4 space-y-2'>
                <div className='text-muted-foreground text-sm'>
                  {t('Progress')}: {run.progress.completed}/{run.progress.total}
                </div>
                <Progress value={progressValue} />
              </div>
            ) : null}
            {run.runningModule === 'stress' && run.streamText ? (
              <pre className='bg-muted mt-4 max-h-64 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
                {run.streamText}
              </pre>
            ) : null}
            {run.summaries.stress ? (
              <p className='text-muted-foreground mt-4 text-sm'>
                {run.summaries.stress}
              </p>
            ) : null}
            {run.metrics ? <StressMetricsDashboard metrics={run.metrics} /> : null}
          </TitledCard>

          {run.errorMessage ? (
            <Alert variant='destructive'>
              <AlertDescription>{run.errorMessage}</AlertDescription>
            </Alert>
          ) : null}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function FieldSelect(props: {
  label: string
  value: string
  disabled: boolean
  items: Array<{ value: string; label: string }>
  onChange: (value: string) => void
}) {
  return (
    <div className='space-y-2'>
      <Label>{props.label}</Label>
      <Select
        items={props.items}
        value={props.value}
        disabled={props.disabled}
        onValueChange={(value) => {
          if (value) props.onChange(value)
        }}
      >
        <SelectTrigger className='w-full'>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {props.items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}

function StreamSwitch(props: {
  id: string
  checked: boolean
  disabled: boolean
  onChange: (checked: boolean) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex items-end gap-3 pb-2'>
      <Switch
        id={props.id}
        checked={props.checked}
        disabled={props.disabled}
        onCheckedChange={(checked) => props.onChange(Boolean(checked))}
      />
      <Label htmlFor={props.id}>{t('Stream')}</Label>
    </div>
  )
}

function NumberField(props: {
  id: string
  label: string
  value: number
  min: number
  max: number
  step?: number
  disabled: boolean
  presets?: Array<{ label: string; value: number }>
  onChange: (value: number) => void
}) {
  return (
    <div className='space-y-1.5'>
      <Label htmlFor={props.id}>{props.label}</Label>
      <Input
        id={props.id}
        type='number'
        min={props.min}
        max={props.max}
        step={props.step}
        value={props.value}
        disabled={props.disabled}
        onChange={(event) => {
          const raw = event.target.value
          const next =
            props.step && props.step < 1
              ? Number.parseFloat(raw)
              : Number.parseInt(raw, 10)
          props.onChange(Number.isNaN(next) ? 0 : next)
        }}
      />
      {props.presets && props.presets.length > 0 ? (
        <div className='flex flex-wrap gap-1 pt-0.5'>
          {props.presets.map((preset) => {
            const active = props.value === preset.value
            return (
              <Button
                key={preset.label}
                type='button'
                variant={active ? 'secondary' : 'outline'}
                size='xs'
                disabled={props.disabled}
                className='h-5.5 px-1.5 text-[11px] font-normal'
                onClick={() => props.onChange(preset.value)}
              >
                {preset.label}
              </Button>
            )
          })}
        </div>
      ) : null}
    </div>
  )
}

function OptionalNumberField(props: {
  id: string
  label: string
  value: string
  min: number
  max: number
  step?: number
  disabled: boolean
  placeholder?: string
  onChange: (value: string) => void
}) {
  return (
    <div className='space-y-2'>
      <Label htmlFor={props.id}>{props.label}</Label>
      <Input
        id={props.id}
        type='number'
        min={props.min}
        max={props.max}
        step={props.step}
        value={props.value}
        disabled={props.disabled}
        placeholder={props.placeholder}
        onChange={(event) => props.onChange(event.target.value)}
      />
    </div>
  )
}

function CheckTable(props: {
  checks: CheckResult[]
  busy: boolean
  onRun?: (id: string) => void
}) {
  const { t } = useTranslation()
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t('Check')}</TableHead>
          <TableHead>{t('Result')}</TableHead>
          <TableHead>{t('Detail')}</TableHead>
          {props.onRun ? <TableHead className='w-24'>{t('Action')}</TableHead> : null}
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.checks.map((check) => (
          <TableRow key={check.id}>
            <TableCell>{t(check.title)}</TableCell>
            <TableCell>
              <StatusBadge
                variant={statusVariant(check.status)}
                copyable={false}
                label={t(statusLabel(check.status))}
              />
            </TableCell>
            <TableCell className='text-muted-foreground max-w-xl whitespace-pre-wrap'>
              {check.message ?? ''}
            </TableCell>
            {props.onRun ? (
              <TableCell>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={props.busy}
                  onClick={() => props.onRun?.(check.id)}
                >
                  {t('Run this check')}
                </Button>
              </TableCell>
            ) : null}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

function stampFileName(): string {
  const now = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}`
}

function downloadTextFile(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const href = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = href
  link.download = filename
  link.click()
  URL.revokeObjectURL(href)
}

function buildTestReport(input: {
  baseUrl: string
  model: string
  basicChecks: CheckResult[]
  cacheChecks: CheckResult[]
  summaries: { basic: string; cache: string; stress: string }
  metrics: ReturnType<typeof useSupplierTestRun>['metrics']
  errorMessage: string
  statusLabel: (status: CheckStatus) => string
  t: (key: string) => string
}): string {
  const lines = [
    '========================================',
    input.t('Supplier Test Report'),
    `Time: ${new Date().toLocaleString()}`,
    `Base URL: ${input.baseUrl || '-'}`,
    `Model: ${input.model || '-'}`,
    '========================================',
    '',
    `## ${input.t('Basic acceptance')}`,
  ]
  for (const check of input.basicChecks) {
    const detail = check.message ? ` — ${check.message}` : ''
    lines.push(`- ${input.t(check.title)}: ${input.t(input.statusLabel(check.status))}${detail}`)
  }
  if (input.summaries.basic) {
    lines.push(`  ${input.summaries.basic}`)
  }

  lines.push('', `## ${input.t('Cache test')}`)
  for (const check of input.cacheChecks) {
    const detail = check.message ? ` — ${check.message}` : ''
    lines.push(`- ${input.t(check.title)}: ${input.t(input.statusLabel(check.status))}${detail}`)
  }
  if (input.summaries.cache) {
    lines.push(`  ${input.summaries.cache}`)
  }

  lines.push('', `## ${input.t('Stress test')}`)
  if (input.metrics) {
    const m = input.metrics
    lines.push(
      `- ${input.t('Total requests')}: ${m.total}`,
      `- ${input.t('Succeeded')}: ${m.succeeded}`,
      `- ${input.t('Failed')}: ${m.failed}`,
      `- ${input.t('Error rate')}: ${(m.error_rate * 100).toFixed(1)}%`,
      `- ${input.t('Duration')}: ${m.elapsed_ms >= 1000 ? `${(m.elapsed_ms / 1000).toFixed(2)} s` : `${m.elapsed_ms.toFixed(0)} ms`}`,
      `- ${input.t('Throughput')}: ${Number.isFinite(m.tokens_per_sec) ? m.tokens_per_sec.toFixed(1) : '-'} tok/s`,
      `- TTFT: ${input.t('Avg')} ${formatMs(m.ttft_avg_ms)} | P50 ${formatMs(m.ttft_p50_ms)} | P90 ${formatMs(m.ttft_p90_ms)}`,
      `- TPOT: ${input.t('Avg')} ${formatMs(m.tpot_avg_ms)} | P50 ${formatMs(m.tpot_p50_ms)} | P90 ${formatMs(m.tpot_p90_ms)}`
    )
  }
  if (input.summaries.stress) {
    lines.push(`  ${input.summaries.stress}`)
  }

  if (input.errorMessage) {
    lines.push('', `## ${input.t('Error')}`, input.errorMessage)
  }
  return `${lines.join('\n')}\n`
}

function statusLabel(status: CheckStatus): string {
  if (status === 'pass') return 'Passed'
  if (status === 'fail') return 'Failed'
  if (status === 'skip') return 'Skipped'
  if (status === 'running') return 'Running'
  return 'Idle'
}

function StressMetricsDashboard(props: {
  metrics: NonNullable<ReturnType<typeof useSupplierTestRun>['metrics']>
}) {
  const { t } = useTranslation()
  const metrics = props.metrics
  const errorRatePercent = metrics.error_rate * 100

  let stabilityLabel = t('Excellent stability (0% errors)')
  let stabilityVariant: StatusVariant = 'success'
  if (metrics.error_rate > 0 && metrics.error_rate < 0.05) {
    stabilityLabel = t('Minor failures ({{rate}}% errors)', {
      rate: errorRatePercent.toFixed(1),
    })
    stabilityVariant = 'warning'
  } else if (metrics.error_rate >= 0.05) {
    stabilityLabel = t('High failure rate ({{rate}}% errors)', {
      rate: errorRatePercent.toFixed(1),
    })
    stabilityVariant = 'danger'
  }

  let ttftRating = t('Fast first token (< 1s)')
  let ttftVariant: StatusVariant = 'success'
  if (metrics.ttft_avg_ms > 3000) {
    ttftRating = t('Higher first token latency (> 3s)')
    ttftVariant = 'neutral'
  } else if (metrics.ttft_avg_ms > 1000) {
    ttftRating = t('Normal first token (1s - 3s)')
    ttftVariant = 'info'
  }

  let speedRating = t('High generation speed (> 40 tok/s)')
  let speedVariant: StatusVariant = 'success'
  if (
    metrics.tpot_avg_ms > 60 ||
    (metrics.tokens_per_sec > 0 && metrics.tokens_per_sec < 15)
  ) {
    speedRating = t('Moderate generation speed (< 15 tok/s)')
    speedVariant = 'neutral'
  } else if (
    metrics.tpot_avg_ms > 25 ||
    (metrics.tokens_per_sec > 0 && metrics.tokens_per_sec < 40)
  ) {
    speedRating = t('Standard generation speed (15 - 40 tok/s)')
    speedVariant = 'info'
  }

  return (
    <div className='mt-6 space-y-4'>
      {/* Overview stats bar */}
      <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-4'>
        <div className='bg-muted/40 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Success rate')}
          </div>
          <div className='mt-1 flex items-baseline gap-2'>
            <span className='text-xl font-bold'>
              {metrics.succeeded}/{metrics.total}
            </span>
            <StatusBadge
              variant={stabilityVariant}
              copyable={false}
              label={stabilityLabel}
            />
          </div>
        </div>

        <div className='bg-muted/40 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Total duration')}
          </div>
          <div className='mt-1 text-xl font-bold'>
            {metrics.elapsed_ms >= 1000
              ? `${(metrics.elapsed_ms / 1000).toFixed(2)} s`
              : `${metrics.elapsed_ms.toFixed(0)} ms`}
          </div>
        </div>

        <div className='bg-muted/40 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Throughput')}
          </div>
          <div className='mt-1 text-xl font-bold'>
            {Number.isFinite(metrics.tokens_per_sec) &&
            metrics.tokens_per_sec > 0
              ? `${metrics.tokens_per_sec.toFixed(1)} tok/s`
              : '-'}
          </div>
        </div>

        <div className='bg-muted/40 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs font-medium'>
            {t('Failed requests')}
          </div>
          <div className='mt-1 flex items-baseline gap-2'>
            <span className='text-xl font-bold'>{metrics.failed}</span>
            <span className='text-muted-foreground text-xs'>
              {formatRate(metrics.error_rate)} {t('error rate')}
            </span>
          </div>
        </div>
      </div>

      {/* Latency Breakdown (TTFT & TPOT) */}
      <div className='grid gap-4 md:grid-cols-2'>
        <div className='bg-muted/20 rounded-lg border p-4'>
          <div className='flex items-center justify-between pb-2'>
            <div className='font-semibold text-sm'>
              {t('TTFT (Time to First Token)')}
            </div>
            {metrics.ttft_avg_ms > 0 ? (
              <StatusBadge
                variant={ttftVariant}
                copyable={false}
                label={ttftRating}
              />
            ) : null}
          </div>
          <p className='text-muted-foreground mb-3 text-xs'>
            {t(
              'Measures latency from sending the request until the first token is received.'
            )}
          </p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Average')}</TableHead>
                <TableHead>{t('P50')}</TableHead>
                <TableHead>{t('P90')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow>
                <TableCell className='font-medium'>
                  {formatMs(metrics.ttft_avg_ms)}
                </TableCell>
                <TableCell>{formatMs(metrics.ttft_p50_ms)}</TableCell>
                <TableCell>{formatMs(metrics.ttft_p90_ms)}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>

        <div className='bg-muted/20 rounded-lg border p-4'>
          <div className='flex items-center justify-between pb-2'>
            <div className='font-semibold text-sm'>
              {t('TPOT (Time Per Output Token)')}
            </div>
            {metrics.tpot_avg_ms > 0 ? (
              <StatusBadge
                variant={speedVariant}
                copyable={false}
                label={speedRating}
              />
            ) : null}
          </div>
          <p className='text-muted-foreground mb-3 text-xs'>
            {t(
              'Measures generation latency per output token (inter-token time).'
            )}
          </p>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Average')}</TableHead>
                <TableHead>{t('P50')}</TableHead>
                <TableHead>{t('P90')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow>
                <TableCell className='font-medium'>
                  {formatMs(metrics.tpot_avg_ms)}
                </TableCell>
                <TableCell>{formatMs(metrics.tpot_p50_ms)}</TableCell>
                <TableCell>{formatMs(metrics.tpot_p90_ms)}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </div>

      {/* General Informative Assessment Note */}
      <div className='bg-background/50 text-muted-foreground space-y-1 rounded-lg border border-dashed p-3 text-xs'>
        <div className='text-foreground flex items-center gap-1.5 font-medium'>
          <Activity className='size-3.5' />
          {t('General Quality Assessment')}
        </div>
        <div>
          {t(
            'Reference indicators: stability {{stability}}, TTFT avg {{ttft}}, TPOT avg {{tpot}}, throughput {{tps}} tok/s. Latencies vary naturally with model size, concurrency, prompt length, and reasoning overhead.',
            {
              stability:
                errorRatePercent === 0
                  ? t('100% available')
                  : `${formatRate(metrics.error_rate)} ${t('error rate')}`,
              ttft: formatMs(metrics.ttft_avg_ms),
              tpot: formatMs(metrics.tpot_avg_ms),
              tps:
                Number.isFinite(metrics.tokens_per_sec) &&
                metrics.tokens_per_sec > 0
                  ? metrics.tokens_per_sec.toFixed(1)
                  : '-',
            }
          )}
        </div>
      </div>
    </div>
  )
}
