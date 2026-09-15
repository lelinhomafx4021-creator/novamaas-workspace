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
import type { BasicForm, CacheForm, CheckResult, StressForm } from './types'
import longText from './corpora/long.txt?raw'
import mediumText from './corpora/medium.txt?raw'
import minQpsText from './corpora/min-qps.txt?raw'
import shortText from './corpora/short.txt?raw'
import veryLongText from './corpora/very-long.txt?raw'

export const API_ENDPOINTS = {
  MODELS: '/api/supplier-test/models',
  RUNS: '/api/supplier-test/runs',
} as const

export const STRESS_CORPORA: Array<{
  id: string
  labelKey: string
  prompt: string
}> = [
  { id: 'min-qps', labelKey: 'Min QPS', prompt: minQpsText.trim() },
  { id: 'short', labelKey: 'Short text', prompt: shortText.trim() },
  { id: 'medium', labelKey: 'Medium text', prompt: mediumText.trim() },
  { id: 'long', labelKey: 'Long text', prompt: longText.trim() },
  {
    id: 'very-long',
    labelKey: 'Very long text',
    prompt: veryLongText.trim(),
  },
  { id: 'custom', labelKey: 'Custom prompt', prompt: '' },
]

export function resolveStressPrompt(form: StressForm): string {
  if (form.corpus === 'custom') {
    const custom = form.prompt.trim()
    if (custom) return custom
  }
  const corpus = STRESS_CORPORA.find((item) => item.id === form.corpus)
  if (corpus?.prompt) return corpus.prompt
  return shortText.trim()
}

export const DEFAULT_STRESS_FORM: StressForm = {
  concurrency: 10,
  rounds: 1,
  maxTokens: 256,
  targetTokens: 0,
  corpus: 'short',
  prompt: '',
  stream: true,
}

export const STRESS_PRESETS: Array<{
  id: string
  labelKey: string
  concurrency: number
  rounds: number
  maxTokens: number
  targetTokens: number
}> = [
  { id: 'min-qps', labelKey: 'Min QPS', concurrency: 1, rounds: 1, maxTokens: 8, targetTokens: 0 },
  { id: 'short', labelKey: 'Short text', concurrency: 10, rounds: 1, maxTokens: 64, targetTokens: 0 },
  { id: 'standard', labelKey: 'Standard stress', concurrency: 20, rounds: 2, maxTokens: 256, targetTokens: 0 },
  { id: 'long', labelKey: 'Long text', concurrency: 5, rounds: 1, maxTokens: 128, targetTokens: 0 },
]

export const TARGET_TOKEN_PRESETS: Array<{ label: string; value: number }> = [
  { label: 'As is (0)', value: 0 },
  { label: '1k', value: 1000 },
  { label: '8k', value: 8000 },
  { label: '16k', value: 16000 },
  { label: '32k', value: 32000 },
  { label: '64k', value: 64000 },
  { label: '100k', value: 100000 },
]

export const CACHE_WARM_TOKEN_PRESETS: Array<{ label: string; value: number }> = [
  { label: 'No pad (0)', value: 0 },
  { label: '1.5k', value: 1500 },
  { label: '8k', value: 8000 },
  { label: '32k', value: 32768 },
  { label: '70k', value: 70000 },
  { label: '128k', value: 128000 },
]

export const CACHE_WAIT_PRESETS = [0, 5, 30, 60]
export const CACHE_ROUND_PRESETS = [1, 3, 5, 10, 20]

export const DEFAULT_BASIC_PROMPT = '请用一句话介绍你自己。'

export const DEFAULT_BASIC_FORM: BasicForm = {
  prompt: DEFAULT_BASIC_PROMPT,
  maxTokens: 64,
  temperature: '',
  topP: '',
  stream: true,
}

export const CACHE_SIZE_TIERS: Array<{ value: number; labelKey: string }> = [
  { value: 1500, labelKey: '1500 tokens' },
  { value: 3000, labelKey: '3000 tokens' },
  { value: 8000, labelKey: '8000 tokens' },
  { value: 32768, labelKey: '32768 tokens (Gemini large cache)' },
  { value: 70000, labelKey: '70000 tokens (Long context cache)' },
  { value: 0, labelKey: 'Do not pad' },
]

export const CACHE_ROUND_OPTIONS = [1, 3, 5, 10, 20]

export const CACHE_CORPORA: Array<{
  id: string
  labelKey: string
  prompt: string
}> = [
  {
    id: 'auto',
    labelKey: 'Auto pad to selected size',
    prompt: '',
  },
  {
    id: 'zh',
    labelKey: 'Chinese prefix',
    prompt:
      '请阅读并记住下面的背景材料。之后我会问一个很短的问题，请根据这些材料回答。这段前缀用于测试提示缓存。',
  },
  {
    id: 'en',
    labelKey: 'English prefix',
    prompt:
      'Read and remember this background. I will then ask a short follow-up. This prefix is used to test prompt caching.',
  },
  {
    id: 'custom',
    labelKey: 'Custom prefix',
    prompt: '',
  },
]

export const DEFAULT_CACHE_FOLLOW_UP =
  '根据前面的内容，只用一个词回复：pong'

export const DEFAULT_CACHE_FORM: CacheForm = {
  corpus: 'auto',
  prompt: '',
  followUp: DEFAULT_CACHE_FOLLOW_UP,
  warmTokens: 1500,
  waitSeconds: 5,
  maxTokens: 16,
  rounds: 5,
  stream: true,
}

export const BASIC_CHECKS: CheckResult[] = [
  { id: 'connectivity', title: 'Connectivity', status: 'idle' },
  { id: 'stream_format', title: 'Stream format', status: 'idle' },
  { id: 'usage', title: 'Usage fields', status: 'idle' },
  { id: 'request_id', title: 'Request id', status: 'idle' },
  { id: 'sampling', title: 'Sampling parameters', status: 'idle' },
  { id: 'json_mode', title: 'JSON mode', status: 'idle' },
  { id: 'tool_call', title: 'Tool call', status: 'idle' },
  { id: 'thinking', title: 'Thinking mode', status: 'idle' },
  { id: 'auth_error', title: 'Auth error', status: 'idle' },
  { id: 'bad_request', title: 'Bad request', status: 'idle' },
]

export const CACHE_CHECKS: CheckResult[] = [
  { id: 'cache_warm', title: 'Cache warm', status: 'idle' },
  { id: 'cache_probe', title: 'Cache probe', status: 'idle' },
  { id: 'cache_tokens', title: 'Cached tokens', status: 'idle' },
  { id: 'cache_hit_rate', title: 'Cache hit rate', status: 'idle' },
]

export const MAX_CONCURRENCY = 1000
export const MAX_ROUNDS = 10000
export const MAX_TOKENS_CAP = 256000
export const MAX_CACHE_ROUNDS = 50
export const MAX_CACHE_WAIT_SECONDS = 600
export const STRESS_WARN_TOTAL = 50
