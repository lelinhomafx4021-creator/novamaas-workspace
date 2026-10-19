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
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { SSE } from 'sse.js'

import { getFreshAuthHeaders } from '@/lib/api'

import type { QueryVideoTaskResult } from '../api'
import {
  API_ENDPOINTS,
  BASIC_CHECKS,
  CACHE_CHECKS,
  VIDEO_CHECKS,
} from '../constants'
import type {
  CacheMetrics,
  CheckResult,
  StressMetrics,
  SupplierTestEvent,
  SupplierTestModule,
  SupplierTestRunRequest,
  VideoMetrics,
  VideoTestRecord,
  VideoExchange,
} from '../types'

type StreamHandle = {
  close: () => void
  stream: () => void
  xhr?: XMLHttpRequest | null
  readyState?: number
  addEventListener: (
    type: string,
    listener: (event: Event & { data?: string; responseCode?: number }) => void
  ) => void
}

function errorMessageFromStream(
  event: Event & { data?: string },
  source: StreamHandle,
  fallback: string
): string {
  const chunks = [event.data, source.xhr?.responseText]
  for (const chunk of chunks) {
    const text = chunk?.trim()
    if (!text) continue
    try {
      const parsed = JSON.parse(text) as { message?: string; type?: string }
      if (parsed.message) return parsed.message
    } catch {
      const line = text.split('\n').find((entry) => entry.startsWith('data:'))
      if (line) {
        try {
          const parsed = JSON.parse(line.slice(5).trim()) as {
            message?: string
          }
          if (parsed.message) return parsed.message
        } catch {
          // keep looking
        }
      }
    }
  }
  return fallback
}

function resetChecks(current: CheckResult[], ids?: string[]): CheckResult[] {
  const selected = ids && ids.length > 0 ? new Set(ids) : null
  return current.map((check) => {
    if (selected && !selected.has(check.id)) {
      return check
    }
    return {
      ...check,
      status: 'idle',
      message: undefined,
      messageArgs: undefined,
    }
  })
}

function videoSupplier(value: string): string {
  try {
    return new URL(value).origin
  } catch {
    return value.trim().replace(/\/+$/, '')
  }
}

export function useSupplierTestRun() {
  const { t } = useTranslation()
  const [videoRuns, setVideoRuns] = useState<VideoTestRecord[]>([])
  const activeVideoRef = useRef('')
  const videoRunsRef = useRef(videoRuns)
  videoRunsRef.current = videoRuns
  const sourceRef = useRef<StreamHandle | null>(null)
  const runIdRef = useRef(0)
  const [runningModule, setRunningModule] = useState<SupplierTestModule | null>(
    null
  )
  const [basicChecks, setBasicChecks] = useState<CheckResult[]>(BASIC_CHECKS)
  const [cacheChecks, setCacheChecks] = useState<CheckResult[]>(CACHE_CHECKS)
  const [videoChecks, setVideoChecks] = useState<CheckResult[]>(VIDEO_CHECKS)
  const [streamText, setStreamText] = useState('')
  const [basicStreamText, setBasicStreamText] = useState('')
  const [progress, setProgress] = useState({ completed: 0, total: 0 })
  const [metrics, setMetrics] = useState<StressMetrics | null>(null)
  const [cacheMetrics, setCacheMetrics] = useState<CacheMetrics | null>(null)
  const [videoMetrics, setVideoMetrics] = useState<VideoMetrics | null>(null)
  const [summaries, setSummaries] = useState({
    basic: '',
    cache: '',
    stress: '',
  })
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(
    () => () => {
      runIdRef.current += 1
      sourceRef.current?.close()
      sourceRef.current = null
    },
    []
  )

  const stop = useCallback(() => {
    runIdRef.current += 1
    const source = sourceRef.current
    sourceRef.current = null
    source?.close()
    setVideoRuns((current) =>
      current.map((item) =>
        item.id === activeVideoRef.current && item.outcome === 'Running'
          ? { ...item, outcome: 'Cancelled' }
          : item
      )
    )
    setRunningModule(null)
  }, [])

  const start = useCallback(
    async (
      payload: SupplierTestRunRequest,
      materials: VideoExchange[] = []
    ) => {
      const runId = ++runIdRef.current
      const previousSource = sourceRef.current
      sourceRef.current = null
      previousSource?.close()
      const module = payload.modules[0] ?? null
      setRunningModule(module)
      setErrorMessage('')
      setProgress({ completed: 0, total: 0 })
      if (module === 'basic') {
        setBasicChecks((current) => resetChecks(current, payload.basic.checks))
        setBasicStreamText('')
        setSummaries((current) => ({ ...current, basic: '' }))
      }
      if (module === 'cache') {
        setCacheChecks(
          CACHE_CHECKS.map((check) => ({ ...check, status: 'idle' }))
        )
        setCacheMetrics(null)
        setSummaries((current) => ({ ...current, cache: '' }))
      }
      if (module === 'stress') {
        setStreamText('')
        setMetrics(null)
        setSummaries((current) => ({ ...current, stress: '' }))
      }
      let pollingOffset = 0,
        pollingElapsedOffset = 0
      if (module === 'video') {
        const submit =
          !payload.video?.checks?.length ||
          payload.video.checks.includes('video_submit')
        const previous = !submit
          ? videoRunsRef.current.find(
              (item) =>
                videoSupplier(item.baseUrl) ===
                  videoSupplier(payload.base_url) &&
                item.metrics.task_id === payload.video?.task_id
            )
          : undefined
        pollingOffset = previous?.metrics.poll_count || 0
        pollingElapsedOffset = previous?.metrics.poll_ms || 0
        const id = previous?.id || crypto.randomUUID()
        activeVideoRef.current = id
        setVideoRuns((current) => {
          const sameSupplier =
            current[0] &&
            videoSupplier(current[0].baseUrl) ===
              videoSupplier(payload.base_url)
              ? current
              : []
          return previous
            ? sameSupplier.map((item) =>
                item.id === id
                  ? {
                      ...item,
                      outcome: 'Running',
                      issue: undefined,
                      metrics: {
                        ...item.metrics,
                        final: undefined,
                        last_query: undefined,
                        video_url: undefined,
                        status: '',
                      },
                    }
                  : item
              )
            : [
                ...sameSupplier,
                {
                  id,
                  baseUrl: payload.base_url,
                  model: payload.model,
                  startedAt: Date.now(),
                  outcome: 'Running',
                  materials,
                  metrics: {
                    task_id: payload.video?.task_id || '',
                    status: '',
                    elapsed_ms: 0,
                  },
                },
              ]
        })
      }
      if (module === 'video') {
        const videoChecksToRun = payload.video?.checks
        setVideoChecks((current) => resetChecks(current, videoChecksToRun))
        setVideoMetrics(null)
      }

      let headers: Record<string, string>
      try {
        headers = await getFreshAuthHeaders()
      } catch (error) {
        if (runIdRef.current !== runId) return
        if (module === 'video') {
          setVideoRuns((current) =>
            current.map((item) =>
              item.id === activeVideoRef.current
                ? { ...item, outcome: 'Failed' }
                : item
            )
          )
        }
        setRunningModule(null)
        toast.error(
          error instanceof Error ? error.message : t('Failed to start test')
        )
        return
      }
      if (runIdRef.current !== runId) return

      const source = new SSE(API_ENDPOINTS.RUNS, {
        headers: {
          ...headers,
          'Content-Type': 'application/json',
        },
        method: 'POST',
        payload: JSON.stringify(payload),
        withCredentials: true,
        start: false,
      }) as StreamHandle
      sourceRef.current = source
      let completed = false

      const finish = () => {
        if (runIdRef.current !== runId || sourceRef.current !== source) return
        if (module === 'video') {
          setVideoRuns((current) =>
            current.map((item) =>
              item.id === activeVideoRef.current && item.outcome === 'Running'
                ? {
                    ...item,
                    outcome:
                      item.metrics.final && item.metrics.video_url
                        ? 'Succeeded'
                        : 'Incomplete',
                  }
                : item
            )
          )
        }
        setRunningModule(null)
        sourceRef.current = null
        source.close()
      }

      const handleError = (message: string) => {
        setErrorMessage(message)
        if (module === 'video') {
          setVideoRuns((current) =>
            current.map((item) =>
              item.id === activeVideoRef.current
                ? { ...item, outcome: 'Incomplete', issue: message }
                : item
            )
          )
        }
        finish()
        toast.error(message)
      }

      source.addEventListener('message', (event) => {
        if (runIdRef.current !== runId || sourceRef.current !== source) return
        const data = event.data ?? ''
        if (data.trim() === '[DONE]') {
          completed = true
          finish()
          return
        }
        let parsed: SupplierTestEvent
        try {
          parsed = JSON.parse(data) as SupplierTestEvent
        } catch {
          handleError(t('Failed to parse stream'))
          return
        }
        if (parsed.video) {
          const next = parsed.video
          setVideoRuns((current) =>
            current.map((item) =>
              item.id === activeVideoRef.current
                ? {
                    ...item,
                    metrics: {
                      ...item.metrics,
                      ...next,
                      poll_count: pollingOffset + (next.poll_count || 0),
                      poll_ms: pollingElapsedOffset + (next.poll_ms || 0),
                      submit: next.submit || item.metrics.submit,
                      raw_request_json:
                        next.raw_request_json || item.metrics.raw_request_json,
                      raw_submit_response_json:
                        next.raw_submit_response_json ||
                        item.metrics.raw_submit_response_json,
                    },
                  }
                : item
            )
          )
        }
        if (parsed.module === 'video' && parsed.status === 'fail') {
          setVideoRuns((current) =>
            current.map((item) =>
              item.id === activeVideoRef.current
                ? {
                    ...item,
                    outcome: 'Failed',
                    issue: parsed.message
                      ? t(parsed.message, parsed.message_args)
                      : undefined,
                  }
                : item
            )
          )
        }
        if (parsed.video) {
          setVideoMetrics((prev) => {
            if (!prev) return parsed.video ?? null
            return {
              ...prev,
              ...parsed.video,
              raw_request_json:
                parsed.video?.raw_request_json || prev.raw_request_json,
              raw_submit_response_json:
                parsed.video?.raw_submit_response_json ||
                prev.raw_submit_response_json,
              endpoint_url: parsed.video?.endpoint_url || prev.endpoint_url,
            }
          })
        }
        if (parsed.type === 'error') {
          handleError(parsed.message || t('Failed to start test'))
          return
        }
        if (parsed.type === 'check' && parsed.check_id) {
          const applyCheck = (current: CheckResult[]) =>
            current.map((check) =>
              check.id === parsed.check_id
                ? {
                    ...check,
                    title: parsed.title || check.title,
                    status: parsed.status ?? check.status,
                    message: parsed.message,
                    messageArgs: parsed.message_args,
                  }
                : check
            )
          if (parsed.module === 'video') {
            setVideoChecks(applyCheck)
          } else if (parsed.module === 'cache') {
            setCacheChecks(applyCheck)
          } else if (parsed.module !== 'stress') {
            setBasicChecks(applyCheck)
          }
          return
        }
        if (parsed.type === 'stream' && parsed.text) {
          if (parsed.module === 'basic') {
            setBasicStreamText((text) => text + parsed.text)
            return
          }
          if (parsed.module === 'stress') {
            setStreamText((text) => text + parsed.text)
          }
          return
        }
        if (parsed.type === 'progress') {
          if (parsed.check_id) {
            const applyCheck = (current: CheckResult[]) =>
              current.map((check) =>
                check.id === parsed.check_id
                  ? {
                      ...check,
                      title: parsed.title || check.title,
                      status: parsed.status ?? check.status,
                      message: parsed.message,
                      messageArgs: parsed.message_args,
                    }
                  : check
              )
            if (parsed.module === 'video') {
              setVideoChecks(applyCheck)
            } else if (parsed.module === 'cache') {
              setCacheChecks(applyCheck)
            } else if (parsed.module !== 'stress') {
              setBasicChecks(applyCheck)
            }
          }
          setProgress({
            completed: parsed.completed ?? 0,
            total: parsed.total ?? 0,
          })
          return
        }
        if (parsed.type === 'metrics' && parsed.cache) {
          setCacheMetrics(parsed.cache)
          if (!parsed.metrics) {
            return
          }
        }
        if (parsed.type === 'metrics' && parsed.metrics) {
          setMetrics(parsed.metrics)
          setProgress({
            completed: parsed.completed ?? parsed.metrics.total,
            total: parsed.total ?? parsed.metrics.total,
          })
          return
        }
        if (parsed.type === 'summary' && parsed.summary) {
          const next = parsed.summary
          if (parsed.module === 'cache') {
            setSummaries((current) => ({ ...current, cache: next }))
          } else if (parsed.module === 'stress') {
            setSummaries((current) => ({ ...current, stress: next }))
          } else {
            setSummaries((current) => ({ ...current, basic: next }))
          }
          return
        }
        if (parsed.type === 'done') {
          completed = true
          setRunningModule(null)
        }
      })

      source.addEventListener('abort', () => {
        if (runIdRef.current !== runId || sourceRef.current !== source) return
        if (completed) finish()
        else handleError(t('Test stream ended unexpectedly'))
      })

      source.addEventListener('error', (event) => {
        if (runIdRef.current !== runId || sourceRef.current !== source) return
        if (completed) {
          finish()
          return
        }
        const code = event.responseCode ?? source.xhr?.status ?? 0
        if (code >= 200 && code < 300) {
          handleError(t('Test stream ended unexpectedly'))
          return
        }
        handleError(
          errorMessageFromStream(event, source, t('Failed to start test'))
        )
      })

      source.stream()
    },
    [t]
  )

  const updateVideoMetricsWithQueryResult = useCallback(
    (result: QueryVideoTaskResult) => {
      const taskId = result.task_id || ''
      const baseUrl = result.base_url || videoRunsRef.current[0]?.baseUrl || ''
      const existing = videoRunsRef.current.find(
        (item) =>
          videoSupplier(item.baseUrl) === videoSupplier(baseUrl) &&
          item.metrics.task_id === taskId
      )
      const state = (result.status || '').toLowerCase()
      const terminal = [
        'success',
        'succeeded',
        'failed',
        'failure',
        'cancelled',
        'expired',
      ].includes(state)
      const update = {
        task_id: taskId,
        status: result.status || '',
        elapsed_ms: existing?.metrics.elapsed_ms || 0,
        video_url:
          result.success && ['success', 'succeeded'].includes(state)
            ? result.video_url
            : undefined,
        last_query: result.exchange,
        raw_poll_response_json: result.raw_response,
        final: result.success && terminal ? result.exchange : undefined,
        queued: ['queued', 'pending'].includes(state)
          ? existing?.metrics.queued || result.exchange
          : existing?.metrics.queued,
        running: ['running', 'processing', 'in_progress'].includes(state)
          ? existing?.metrics.running || result.exchange
          : existing?.metrics.running,
        poll_count: (existing?.metrics.poll_count || 0) + 1,
        poll_ms:
          (existing?.metrics.poll_ms || 0) + (result.exchange?.elapsed_ms || 0),
      }
      setVideoRuns((current) => {
        let outcome = 'Failed'
        if (result.success && !terminal) outcome = 'Incomplete'
        else if (update.video_url) outcome = 'Succeeded'
        const next = {
          ...(existing || {
            id: crypto.randomUUID(),
            model: '',
            startedAt: Date.now(),
            materials: [],
          }),
          baseUrl,
          metrics: { ...existing?.metrics, ...update },
          outcome,
          issue: result.message || result.fail_reason,
        }
        return existing
          ? current.map((item) => (item.id === existing.id ? next : item))
          : [
              ...(current[0] &&
              videoSupplier(current[0].baseUrl) === videoSupplier(baseUrl)
                ? current
                : []),
              next,
            ]
      })
      setVideoMetrics((prev) => ({
        ...(prev?.task_id === taskId ? prev : null),
        ...update,
      }))
      if (!result.success) {
        setVideoChecks(
          VIDEO_CHECKS.map((check) => {
            let status: CheckResult['status'] = 'idle'
            if (check.id === 'video_poll') status = 'fail'
            else if (check.id === 'video_result') status = 'skip'
            return {
              ...check,
              status,
              message: check.id === 'video_poll' ? result.message : undefined,
            }
          })
        )
        return
      }
      if (result.status) {
        const normalized = result.status.toLowerCase()
        const isSuccess = normalized === 'succeeded' || normalized === 'success'
        const isFailed = ['failed', 'failure', 'cancelled', 'expired'].includes(
          normalized
        )

        let pollStatus: CheckResult['status'] = 'running'
        let pollMessage = 'Current status: {{status}}'
        let pollMessageArgs: CheckResult['messageArgs'] = {
          status: result.status,
        }
        if (isSuccess) {
          pollStatus = 'pass'
          pollMessage = 'Manual query: task completed'
          pollMessageArgs = undefined
        } else if (isFailed) {
          pollStatus = 'fail'
          pollMessage = result.fail_reason || 'Task failed'
          pollMessageArgs = undefined
        }

        setVideoChecks((current) =>
          current.map((check) => {
            if (check.id === 'video_poll') {
              return {
                ...check,
                status: pollStatus,
                message: pollMessage,
                messageArgs: pollMessageArgs,
              }
            }
            if (check.id === 'video_result') {
              if (isSuccess) {
                return {
                  ...check,
                  status: result.video_url ? 'pass' : 'fail',
                  message: result.video_url
                    ? 'Successfully fetched video playback URL'
                    : 'Task succeeded but no video URL returned',
                  messageArgs: undefined,
                }
              }
              if (isFailed) {
                return {
                  ...check,
                  status: 'fail',
                  message: result.fail_reason || 'Task failed',
                  messageArgs: undefined,
                }
              }
            }
            return check
          })
        )
      }
    },
    []
  )

  const updateVideoVerification = useCallback(
    (id: string, verification: VideoTestRecord['verification']) => {
      setVideoRuns((current) =>
        current.map((item) =>
          item.id === id ? { ...item, verification } : item
        )
      )
    },
    []
  )
  const clearVideoReport = useCallback(() => {
    setVideoRuns([])
    setVideoMetrics(null)
    activeVideoRef.current = ''
  }, [])

  return {
    videoRuns,
    updateVideoVerification,
    clearVideoReport,
    runningModule,
    basicChecks,
    cacheChecks,
    videoChecks,
    streamText,
    basicStreamText,
    progress,
    metrics,
    cacheMetrics,
    videoMetrics,
    summaries,
    errorMessage,
    start,
    stop,
    updateVideoMetricsWithQueryResult,
  }
}
