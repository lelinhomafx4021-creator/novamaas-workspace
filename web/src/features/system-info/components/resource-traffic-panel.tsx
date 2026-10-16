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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Empty, EmptyDescription, EmptyHeader } from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  formatResourceBytes,
  formatResourceNumber,
} from '../lib/resource-monitor-format'
import type {
  ResourceTrafficCategory,
  ResourceTrafficRow,
  ResourceTrafficScope,
} from '../types'

const SCOPES: ResourceTrafficScope[] = [
  'inbound',
  'outbound',
  'database',
  'cache',
]
const SCOPE_LABELS: Record<ResourceTrafficScope, string> = {
  inbound: 'Inbound HTTP',
  outbound: 'Outbound HTTP',
  database: 'Database',
  cache: 'Cache',
}
const CATEGORY_LABELS: Record<ResourceTrafficCategory, string> = {
  relay: 'Relay',
  video: 'Video',
  assets: 'Assets',
  storage: 'Storage',
  webhook: 'Webhook',
  admin: 'Administration',
  other: 'Other',
}

function ResourceTrafficTable(props: { rows: ResourceTrafficRow[] }) {
  const { t } = useTranslation()
  if (props.rows.length === 0) {
    return (
      <Empty className='min-h-36'>
        <EmptyHeader>
          <EmptyDescription>
            {t('No application traffic recorded in this period.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className='overflow-x-auto'>
      <Table className='min-w-[760px]'>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Category')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Errors')}</TableHead>
            <TableHead className='text-right'>{t('Received bytes')}</TableHead>
            <TableHead className='text-right'>{t('Sent bytes')}</TableHead>
            <TableHead className='text-right'>{t('Active requests')}</TableHead>
            <TableHead className='text-right'>{t('Redirects')}</TableHead>
            <TableHead className='text-right'>{t('Connections')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.rows.map((row) => (
            <TableRow key={row.category}>
              <TableCell className='font-medium'>
                {t(CATEGORY_LABELS[row.category])}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceNumber(row.requests)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceNumber(row.errors)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceBytes(row.received_bytes)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceBytes(row.sent_bytes)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceNumber(row.active)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceNumber(row.redirects)}
              </TableCell>
              <TableCell className='text-right font-mono text-xs tabular-nums'>
                {formatResourceNumber(row.connections)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function ResourceTrafficPanel(props: { traffic: ResourceTrafficRow[] }) {
  const { t } = useTranslation()
  const [scope, setScope] = useState<ResourceTrafficScope>('inbound')
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Application traffic')}</CardTitle>
        <CardDescription>
          {t(
            'Selected period totals. Active requests and connections are live.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <Tabs
          value={scope}
          onValueChange={(value) => setScope(value as ResourceTrafficScope)}
          className='gap-4'
        >
          <TabsList
            variant='line'
            className='max-w-full overflow-x-auto'
            aria-label={t('Traffic scope')}
          >
            {SCOPES.map((item) => (
              <TabsTrigger key={item} value={item}>
                {t(SCOPE_LABELS[item])}
              </TabsTrigger>
            ))}
          </TabsList>
          {SCOPES.map((item) => (
            <TabsContent key={item} value={item}>
              <ResourceTrafficTable
                rows={props.traffic.filter((row) => row.scope === item)}
              />
            </TabsContent>
          ))}
        </Tabs>
        <p className='text-muted-foreground text-xs leading-relaxed'>
          {scope === 'inbound' || scope === 'outbound'
            ? t(
                'HTTP bytes count application request and response bodies, including streams. Headers and TLS overhead are excluded. Redirect responses do not include the final media download.'
              )
            : t(
                'Database and cache bytes count tracked application socket traffic. Local SQLite and in-memory cache have no network bytes.'
              )}
        </p>
      </CardContent>
    </Card>
  )
}
