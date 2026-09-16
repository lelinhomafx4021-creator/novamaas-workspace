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
  assessCache,
  assessStress,
  displayMeasured,
  overallLabel,
  VERDICT_LABEL,
  type Assessment,
  type Verdict,
} from './baselines'
import {
  CACHE_ROUND_PRESETS,
  CACHE_WAIT_PRESETS,
  CORPORA,
  DEFAULT_BASIC_FORM,
  DEFAULT_CACHE_FORM,
  DEFAULT_STRESS_FORM,
  LOAD_PRESETS,
  MAX_CACHE_ROUNDS,
  MAX_CACHE_WAIT_SECONDS,
  MAX_CONCURRENCY,
  MAX_ROUNDS,
  MAX_TOKENS_CAP,
  STRESS_WARN_TOTAL,
  resolveCorpusPrompt,
  thisPlatformBaseURL,
} from './constants'
import { useSupplierTestRun } from './hooks/use-supplier-test-run'
import {
  buildHtmlReport,
  buildMarkdownReport,
  downloadFile,
  stampFileName,
  type ReportInput,
} from './report'
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
        prompt: resolveCorpusPrompt(cache),
        follow_up: cache.followUp,
        wait_seconds: cache.waitSeconds,
        max_tokens: cache.maxTokens,
        rounds: cache.rounds,
        stream: cache.stream,
      },
      stress: {
        concurrency: stress.concurrency,
        rounds: stress.rounds,
        max_tokens: stress.maxTokens,
        prompt: resolveCorpusPrompt(stress),
        break_cache: stress.breakCache,
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
    }
    void run.start(buildPayload(module, checks))
  }

  const stressAssessment = run.metrics ? assessStress(run.metrics) : null
  const cacheAssessment = run.cacheMetrics
    ? assessCache(run.cacheMetrics)
    : null

  const reportInput = (): ReportInput => ({
    baseUrl: target.baseUrl.trim(),
    model: target.model.trim(),
    basicChecks: run.basicChecks,
    cacheChecks: run.cacheChecks,
    summaries: run.summaries,
    stressAssessment,
    cacheAssessment,
    errorMessage: run.errorMessage,
    statusLabel,
    t: (key, options) => t(key, options),
  })

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
            try {
              await navigator.clipboard.writeText(
                buildMarkdownReport(reportInput())
              )
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
            downloadFile(
              `supplier-test-${stampFileName()}.md`,
              buildMarkdownReport(reportInput()),
              'text/markdown'
            )
            toast.success(t('Markdown report exported'))
          }}
        >
          <Download />
          {t('Export Markdown')}
        </Button>
        <Button
          variant='outline'
          disabled={busy || !hasReport}
          onClick={() => {
            downloadFile(
              `supplier-test-${stampFileName()}.html`,
              buildHtmlReport(reportInput()),
              'text/html'
            )
            toast.success(t('HTML report exported'))
          }}
        >
          <Download />
          {t('Export HTML')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <TitledCard
            title={t('Target')}
            description={t(
              'Paste an upstream Base URL, or use this platform with a token from Keys. Fetching /v1/models is optional.'
            )}
            icon={<ClipboardCheck />}
            action={
              <div className='flex flex-wrap gap-2'>
                <Button
                  variant='outline'
                  disabled={busy}
                  onClick={() => {
                    const baseUrl = thisPlatformBaseURL()
                    if (!baseUrl) {
                      toast.error(t('Enter a base URL first'))
                      return
                    }
                    setTarget((current) => ({ ...current, baseUrl }))
                    toast.success(
                      t(
                        'Filled this platform Base URL. Paste a token from Keys, then fetch models.'
                      )
                    )
                  }}
                >
                  {t('Use this platform')}
                </Button>
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
              </div>
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
              'Empty temperature / top_p are not sent. Missing vendor fields are skipped, not failed.'
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
            {run.basicStreamText ? (
              <pre className='bg-muted mt-4 max-h-48 overflow-auto rounded-lg p-3 text-sm whitespace-pre-wrap'>
                {run.basicStreamText}
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
              'Corpus is the input prefix, sent as-is. Max tokens only caps the reply. The first request warms cache; later rounds check the hit.'
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
              <StreamSwitch
                id='cache-stream'
                checked={cache.stream}
                disabled={busy}
                onChange={(checked) =>
                  setCache((current) => ({ ...current, stream: checked }))
                }
              />
            </div>
            <div className='mt-4'>
              <CorpusPicker
                id='cache-prompt'
                form={cache}
                disabled={busy}
                onChange={(next) =>
                  setCache((current) => ({
                    ...current,
                    corpus: next.corpus,
                    prompt: next.prompt,
                  }))
                }
              />
            </div>
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
            {cacheAssessment && cacheAssessment.rows.length > 0 ? (
              <AssessmentTable assessment={cacheAssessment} />
            ) : null}
          </TitledCard>

          <TitledCard
            title={t('Stress test')}
            description={t(
              'Corpus is sent as-is. Allow cache reuses it; break cache puts a random prefix in front of each request. Max tokens only caps the reply.'
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
              {LOAD_PRESETS.map((preset) => (
                <Button
                  key={preset.id}
                  type='button'
                  variant='outline'
                  size='sm'
                  disabled={busy}
                  onClick={() =>
                    setStress((current) => ({
                      ...current,
                      concurrency: preset.concurrency,
                      rounds: preset.rounds,
                      maxTokens: preset.maxTokens,
                    }))
                  }
                >
                  {t(preset.labelKey)}
                </Button>
              ))}
            </div>
            <div className='mt-3 flex flex-wrap gap-2'>
              <Button
                type='button'
                variant={stress.breakCache ? 'outline' : 'secondary'}
                size='sm'
                disabled={busy}
                onClick={() =>
                  setStress((current) => ({ ...current, breakCache: false }))
                }
              >
                {t('Allow prompt cache')}
              </Button>
              <Button
                type='button'
                variant={stress.breakCache ? 'secondary' : 'outline'}
                size='sm'
                disabled={busy}
                onClick={() =>
                  setStress((current) => ({ ...current, breakCache: true }))
                }
              >
                {t('Break prompt cache')}
              </Button>
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
            <div className='mt-4'>
              <CorpusPicker
                id='stress-prompt'
                form={stress}
                disabled={busy}
                onChange={(next) =>
                  setStress((current) => ({
                    ...current,
                    corpus: next.corpus,
                    prompt: next.prompt,
                  }))
                }
              />
            </div>
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
            {stressAssessment ? (
              <AssessmentTable assessment={stressAssessment} />
            ) : null}
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

function CorpusPicker(props: {
  id: string
  form: { corpus: string; prompt: string }
  disabled: boolean
  onChange: (next: { corpus: string; prompt: string }) => void
}) {
  const { t } = useTranslation()
  return (
    <div className='space-y-4'>
      <FieldSelect
        label={t('Corpus')}
        value={props.form.corpus}
        disabled={props.disabled}
        items={CORPORA.map((item) => ({
          value: item.id,
          label: t(item.labelKey),
        }))}
        onChange={(corpus) =>
          props.onChange({ corpus, prompt: props.form.prompt })
        }
      />
      {props.form.corpus === 'custom' ? (
        <div className='space-y-2'>
          <Label htmlFor={props.id}>{t('Prompt')}</Label>
          <Textarea
            id={props.id}
            rows={3}
            placeholder={t('Write a short prompt, or pick a built-in corpus')}
            value={props.form.prompt}
            disabled={props.disabled}
            onChange={(event) =>
              props.onChange({
                corpus: props.form.corpus,
                prompt: event.target.value,
              })
            }
          />
        </div>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t(
            'Using built-in corpus ({{chars}} characters). Replace files in supplier-test/corpora to change the text.',
            { chars: resolveCorpusPrompt(props.form).length }
          )}
        </p>
      )}
    </div>
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

function statusLabel(status: CheckStatus): string {
  if (status === 'pass') return 'Passed'
  if (status === 'fail') return 'Failed'
  if (status === 'skip') return 'Skipped'
  if (status === 'running') return 'Running'
  return 'Idle'
}

function verdictClass(verdict: Verdict): string {
  if (verdict === 'ok') return 'text-success font-medium'
  if (verdict === 'slow') return 'text-warning font-medium'
  return 'text-muted-foreground'
}

function AssessmentTable(props: { assessment: Assessment }) {
  const { t } = useTranslation()
  return (
    <div className='mt-4 space-y-2 overflow-x-auto'>
      <div className={verdictClass(props.assessment.overall)}>
        {t(overallLabel(props.assessment.overall))}
      </div>
      <Table className='min-w-[640px]'>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Metric')}</TableHead>
            <TableHead>{t('Measured')}</TableHead>
            <TableHead>{t('Threshold')}</TableHead>
            <TableHead>{t('Verdict')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.assessment.rows.map((row) => (
            <TableRow key={row.id}>
              <TableCell className='whitespace-nowrap'>{t(row.label)}</TableCell>
              <TableCell className='font-medium whitespace-nowrap'>
                {displayMeasured(row, t)}
              </TableCell>
              <TableCell className='text-muted-foreground'>
                {t(row.threshold)}
              </TableCell>
              <TableCell className={verdictClass(row.verdict)}>
                {t(VERDICT_LABEL[row.verdict])}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
