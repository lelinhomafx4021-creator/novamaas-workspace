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
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Controller, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

import {
  createAssetAccessKey,
  deleteAssetAccessKey,
  listAssetAccessKeys,
} from '../api'
import { assertAssetSuccess, assetErrorMessage } from '../asset-utils'
import type { AssetAccessKey, CreatedAssetAccessKey } from '../types'
import {
  AssetAccessKeyList,
  CreatedAssetAccessKeyPanel,
} from './asset-access-key-list'
import { AssetApiIntegrationGuide } from './asset-api-integration-guide'

const ACCESS_KEYS_QUERY_KEY = ['asset-library', 'access-keys'] as const
const MAX_ACCESS_KEYS = 5

export function AssetApiAccessDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [createdKey, setCreatedKey] = useState<CreatedAssetAccessKey | null>(
    null
  )
  const [revokeTarget, setRevokeTarget] = useState<AssetAccessKey | null>(null)
  const schema = z.object({
    name: z.string().trim().min(1, t('Key name is required')).max(64),
  })
  const form = useForm<{ name: string }>({
    resolver: zodResolver(schema),
    defaultValues: { name: '' },
  })
  const keysQuery = useQuery({
    queryKey: ACCESS_KEYS_QUERY_KEY,
    queryFn: async () => assertAssetSuccess(await listAssetAccessKeys()),
    enabled: props.open,
  })
  const keys = keysQuery.data ?? []
  const createMutation = useMutation({
    mutationFn: async (values: { name: string }) =>
      assertAssetSuccess(await createAssetAccessKey(values.name.trim())),
    onSuccess: async (key) => {
      setCreatedKey(key)
      form.reset()
      await queryClient.invalidateQueries({ queryKey: ACCESS_KEYS_QUERY_KEY })
      toast.success(t('Access key created'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const revokeMutation = useMutation({
    mutationFn: async (id: number) =>
      assertAssetSuccess(await deleteAssetAccessKey(id)),
    onSuccess: async () => {
      setRevokeTarget(null)
      await queryClient.invalidateQueries({ queryKey: ACCESS_KEYS_QUERY_KEY })
      toast.success(t('Access key revoked'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })

  const handleOpenChange = (open: boolean) => {
    if (!open && createdKey) return
    if (!open) {
      setCreatedKey(null)
      setRevokeTarget(null)
      form.reset()
    }
    props.onOpenChange(open)
  }

  return (
    <>
      <Dialog open={props.open} onOpenChange={handleOpenChange}>
        <DialogContent
          className='max-h-[calc(100vh-2rem)] overflow-y-auto sm:max-w-3xl'
          showCloseButton={!createdKey}
        >
          <DialogHeader>
            <DialogTitle>{t('Asset Library API')}</DialogTitle>
            <DialogDescription>
              {t(
                'Create AK/SK credentials for downstream clients and connect with the Volcengine-compatible Action API.'
              )}
            </DialogDescription>
          </DialogHeader>

          <AssetApiIntegrationGuide />

          <section
            aria-labelledby='asset-access-keys-title'
            className='grid gap-3'
          >
            <div className='grid gap-1'>
              <h2 id='asset-access-keys-title' className='font-medium'>
                {t('Access keys')}
              </h2>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Create and revoke credentials for your downstream clients.'
                )}
              </p>
            </div>

            {createdKey && (
              <CreatedAssetAccessKeyPanel
                accessKey={createdKey}
                onSaved={() => setCreatedKey(null)}
              />
            )}

            <form
              onSubmit={form.handleSubmit((values) =>
                createMutation.mutate(values)
              )}
            >
              <FieldGroup className='gap-2 sm:grid sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end'>
                <Controller
                  control={form.control}
                  name='name'
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor='asset-access-key-name'>
                        {t('Key name')}
                      </FieldLabel>
                      <Input
                        {...field}
                        id='asset-access-key-name'
                        maxLength={64}
                        placeholder={t('Example: production uploader')}
                        aria-invalid={fieldState.invalid}
                        disabled={
                          createMutation.isPending ||
                          keys.length >= MAX_ACCESS_KEYS
                        }
                      />
                      <FieldDescription>
                        {t('Up to 5 active access keys per account.')}
                      </FieldDescription>
                      <FieldError errors={[fieldState.error]} />
                    </Field>
                  )}
                />
                <Button
                  type='submit'
                  disabled={
                    createMutation.isPending || keys.length >= MAX_ACCESS_KEYS
                  }
                >
                  {createMutation.isPending && (
                    <Spinner data-icon='inline-start' />
                  )}
                  {t('Create access key')}
                </Button>
              </FieldGroup>
            </form>

            <AssetAccessKeyList
              keys={keys}
              loading={keysQuery.isLoading}
              error={keysQuery.error}
              onRevoke={setRevokeTarget}
            />
          </section>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={revokeTarget !== null}
        onOpenChange={(open) => !open && setRevokeTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Revoke access key?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Downstream clients using this key will immediately lose access. This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              disabled={revokeMutation.isPending}
              onClick={() =>
                revokeTarget && revokeMutation.mutate(revokeTarget.id)
              }
            >
              {revokeMutation.isPending && <Spinner data-icon='inline-start' />}
              {t('Revoke')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
