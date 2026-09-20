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
import axios from 'axios'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import type { CostAccountingAPIResponse } from './cost-accounting-types'

export type OperationState<TInput, TResult> = {
  phase: 'preview' | 'applied'
  fingerprint: string
  request: TInput
  result: TResult
}

type OperationVariables<TInput> = {
  request: TInput
  fingerprint: string
}

export function operationFingerprint(values: object): string {
  return JSON.stringify(values)
}

function costAccountingErrorMessage(error: unknown, fallback: string): string {
  if (axios.isAxiosError<CostAccountingAPIResponse<unknown>>(error)) {
    return error.response?.data?.message || error.message || fallback
  }
  return error instanceof Error ? error.message : fallback
}

export function useCostAccountingOperation<
  TInput,
  TResult extends { applied: number },
>(props: {
  mutationFn: (input: TInput) => Promise<TResult>
  appliedMessage: (result: TResult) => string
}) {
  const { t } = useTranslation()
  const [state, setState] = useState<OperationState<TInput, TResult>>()

  const previewMutation = useMutation({
    mutationFn: (variables: OperationVariables<TInput>) =>
      props.mutationFn(variables.request),
    onSuccess: (result, variables) => {
      setState({
        phase: 'preview',
        request: variables.request,
        result,
        fingerprint: variables.fingerprint,
      })
    },
    onError: (error) =>
      toast.error(
        costAccountingErrorMessage(error, t('Cost accounting operation failed'))
      ),
  })

  const applyMutation = useMutation({
    mutationFn: (variables: OperationVariables<TInput>) =>
      props.mutationFn(variables.request),
    onSuccess: (result, variables) => {
      setState({
        phase: 'applied',
        request: variables.request,
        result,
        fingerprint: variables.fingerprint,
      })
      toast.success(props.appliedMessage(result))
    },
    onError: (error) =>
      toast.error(
        costAccountingErrorMessage(error, t('Cost accounting operation failed'))
      ),
  })

  return {
    state,
    setState,
    previewMutation,
    applyMutation,
    isPending: previewMutation.isPending || applyMutation.isPending,
  }
}
