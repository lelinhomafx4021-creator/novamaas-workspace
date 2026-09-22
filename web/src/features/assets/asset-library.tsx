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
import {
  FolderAddIcon,
  Image01Icon,
  Search01Icon,
  Upload01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
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
import { Card, CardContent } from '@/components/ui/card'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'

import {
  createAssetGroup,
  deleteAssetGroup,
  deleteMediaAsset,
  listAssetGroups,
  listMediaAssets,
  uploadMediaAsset,
} from './api'
import { assertAssetSuccess, assetErrorMessage } from './asset-utils'
import { AssetCard } from './components/asset-card'
import {
  AssetGroupDialog,
  type AssetGroupFormValues,
} from './components/asset-group-dialog'
import { AssetUploadDialog } from './components/asset-upload-dialog'
import type { MediaAsset } from './types'

const GROUPS_QUERY_KEY = ['asset-library', 'groups'] as const
const ASSETS_QUERY_KEY = ['asset-library', 'assets'] as const

export function AssetLibrary() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [selectedGroup, setSelectedGroup] = useState('')
  const [search, setSearch] = useState('')
  const [groupDialogOpen, setGroupDialogOpen] = useState(false)
  const [uploadDialogOpen, setUploadDialogOpen] = useState(false)
  const [uploadProgress, setUploadProgress] = useState(0)
  const [deleteTarget, setDeleteTarget] = useState<MediaAsset | null>(null)

  const groupsQuery = useQuery({
    queryKey: GROUPS_QUERY_KEY,
    queryFn: async () => assertAssetSuccess(await listAssetGroups()),
  })
  const assetsQuery = useQuery({
    queryKey: [...ASSETS_QUERY_KEY, selectedGroup],
    queryFn: async () =>
      assertAssetSuccess(await listMediaAssets(selectedGroup || undefined)),
  })
  const groups = useMemo(() => groupsQuery.data ?? [], [groupsQuery.data])
  const assets = useMemo(() => assetsQuery.data ?? [], [assetsQuery.data])
  const groupNames = useMemo(
    () => new Map(groups.map((group) => [group.id, group.name])),
    [groups]
  )
  const filteredAssets = useMemo(() => {
    const keyword = search.trim().toLocaleLowerCase()
    if (!keyword) return assets
    return assets.filter((asset) =>
      `${asset.name} ${asset.id} ${asset.content_type}`
        .toLocaleLowerCase()
        .includes(keyword)
    )
  }, [assets, search])

  const invalidateGroups = () =>
    queryClient.invalidateQueries({ queryKey: GROUPS_QUERY_KEY })
  const invalidateAssets = () =>
    queryClient.invalidateQueries({ queryKey: ASSETS_QUERY_KEY })
  const groupMutation = useMutation({
    mutationFn: async (values: AssetGroupFormValues) =>
      assertAssetSuccess(await createAssetGroup(values)),
    onSuccess: async () => {
      await invalidateGroups()
      setGroupDialogOpen(false)
      toast.success(t('Asset group created'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const uploadMutation = useMutation({
    mutationFn: async (formData: FormData) => {
      setUploadProgress(0)
      return assertAssetSuccess(
        await uploadMediaAsset(formData, setUploadProgress)
      )
    },
    onSuccess: async () => {
      await invalidateAssets()
      setUploadDialogOpen(false)
      setUploadProgress(0)
      toast.success(t('Asset uploaded'))
    },
    onError: (error) => {
      setUploadProgress(0)
      toast.error(assetErrorMessage(error))
    },
  })
  const deleteMutation = useMutation({
    mutationFn: async (id: string) =>
      assertAssetSuccess(await deleteMediaAsset(id)),
    onSuccess: async () => {
      await invalidateAssets()
      setDeleteTarget(null)
      toast.success(t('Asset deletion queued'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })
  const deleteGroupMutation = useMutation({
    mutationFn: async (id: string) =>
      assertAssetSuccess(await deleteAssetGroup(id)),
    onSuccess: async () => {
      setSelectedGroup('')
      await Promise.all([invalidateGroups(), invalidateAssets()])
      toast.success(t('Asset group deleted'))
    },
    onError: (error) => toast.error(assetErrorMessage(error)),
  })

  const loading = groupsQuery.isLoading || assetsQuery.isLoading
  const error = groupsQuery.error || assetsQuery.error

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Asset Library')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            size='sm'
            variant='outline'
            onClick={() => setGroupDialogOpen(true)}
          >
            <HugeiconsIcon icon={FolderAddIcon} data-icon='inline-start' />
            {t('New group')}
          </Button>
          <Button
            size='sm'
            disabled={groups.length === 0}
            onClick={() => setUploadDialogOpen(true)}
          >
            <HugeiconsIcon icon={Upload01Icon} data-icon='inline-start' />
            {t('Upload asset')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex flex-col gap-4 pb-4'>
            <Card size='sm'>
              <CardContent className='grid gap-3 py-3 md:grid-cols-[minmax(180px,240px)_minmax(220px,1fr)_auto] md:items-center'>
                <NativeSelect
                  aria-label={t('Asset group')}
                  value={selectedGroup}
                  onChange={(event) => setSelectedGroup(event.target.value)}
                >
                  <NativeSelectOption value=''>
                    {t('All groups')}
                  </NativeSelectOption>
                  {groups.map((group) => (
                    <NativeSelectOption key={group.id} value={group.id}>
                      {group.name}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
                <InputGroup>
                  <InputGroupAddon>
                    <HugeiconsIcon icon={Search01Icon} />
                  </InputGroupAddon>
                  <InputGroupInput
                    value={search}
                    aria-label={t('Search assets')}
                    placeholder={t('Search by name or asset ID')}
                    onChange={(event) => setSearch(event.target.value)}
                  />
                </InputGroup>
                <span className='text-muted-foreground text-xs tabular-nums md:text-right'>
                  {t('{{count}} assets', { count: filteredAssets.length })}
                </span>
              </CardContent>
            </Card>

            <p className='text-muted-foreground text-xs'>
              {t(
                'Copy an asset reference and use it in image_url, video_url, or audio_url fields.'
              )}
            </p>

            {loading && (
              <div className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5'>
                {Array.from({ length: 8 }, (_, index) => (
                  <Skeleton key={index} className='h-72' />
                ))}
              </div>
            )}
            {error && (
              <div className='text-destructive py-8 text-center text-sm'>
                {assetErrorMessage(error)}
              </div>
            )}
            {!loading && !error && assets.length === 0 && (
              <Empty className='min-h-72 border'>
                <EmptyHeader>
                  <EmptyMedia variant='icon'>
                    <HugeiconsIcon icon={Image01Icon} />
                  </EmptyMedia>
                  <EmptyTitle>{t('No assets yet')}</EmptyTitle>
                  <EmptyDescription>
                    {t(
                      'Create a group, then upload an image, video, or audio file.'
                    )}
                  </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                  <Button
                    onClick={() =>
                      groups.length
                        ? setUploadDialogOpen(true)
                        : setGroupDialogOpen(true)
                    }
                  >
                    {groups.length
                      ? t('Upload asset')
                      : t('Create asset group')}
                  </Button>
                </EmptyContent>
              </Empty>
            )}
            {!loading &&
              !error &&
              assets.length > 0 &&
              filteredAssets.length === 0 && (
                <Empty className='min-h-56 border'>
                  <EmptyHeader>
                    <EmptyTitle>{t('No matching assets')}</EmptyTitle>
                    <EmptyDescription>
                      {t('Try another group or search term.')}
                    </EmptyDescription>
                  </EmptyHeader>
                </Empty>
              )}
            {filteredAssets.length > 0 && (
              <div
                data-testid='asset-grid'
                data-density='compact'
                className='grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5'
              >
                {filteredAssets.map((asset) => (
                  <AssetCard
                    key={asset.id}
                    asset={asset}
                    groupName={
                      groupNames.get(asset.group_id) || t('Unknown group')
                    }
                    onDelete={setDeleteTarget}
                  />
                ))}
              </div>
            )}

            {groups.length > 0 && selectedGroup && assets.length === 0 && (
              <Button
                variant='ghost'
                className='text-destructive self-start'
                disabled={deleteGroupMutation.isPending}
                onClick={() => deleteGroupMutation.mutate(selectedGroup)}
              >
                {t('Delete empty group')}
              </Button>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AssetGroupDialog
        open={groupDialogOpen}
        onOpenChange={setGroupDialogOpen}
        onSubmit={(values) => groupMutation.mutate(values)}
        pending={groupMutation.isPending}
      />
      <AssetUploadDialog
        open={uploadDialogOpen}
        onOpenChange={setUploadDialogOpen}
        groups={groups}
        selectedGroup={selectedGroup}
        onSubmit={(values) => uploadMutation.mutate(values)}
        pending={uploadMutation.isPending}
        progress={uploadProgress}
      />
      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete asset?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The asset and its managed copies will be removed in the background. This action cannot be undone.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={() =>
                deleteTarget && deleteMutation.mutate(deleteTarget.id)
              }
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
