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
  CpuIcon,
  DatabaseSync01Icon,
  DeliverySent01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

export function ValueChain() {
  const { t } = useTranslation()

  const values = [
    {
      number: '01',
      icon: DatabaseSync01Icon,
      title: t('Unified gateway access'),
      description: t(
        'Bring multi-provider tokens into one managed pool with unified routing, pricing, quotas and observability.'
      ),
      status: t('Available now'),
      footer: t('One supply layer'),
    },
    {
      number: '02',
      icon: DeliverySent01Icon,
      title: t('Evaluated token supply'),
      description: t(
        'Make channel security, reliability, cost and enterprise fit easier to compare before you choose.'
      ),
      status: t('Roadmap'),
      footer: t('A clearer supply shelf'),
      roadmap: true,
    },
    {
      number: '03',
      icon: CpuIcon,
      title: t('Enterprise operations'),
      description: t(
        'Deepen tenant controls, metered billing and security governance, with compute leasing on the horizon.'
      ),
      status: t('Expanding'),
      footer: t('Built for enterprise'),
      roadmap: true,
    },
  ]

  return (
    <section
      className='maas-deferred-section relative px-5 py-24 sm:px-6 md:py-32'
      aria-labelledby='value-chain-title'
    >
      <div className='mx-auto max-w-7xl'>
        <AnimateInView className='grid gap-6 lg:grid-cols-[0.75fr_1fr] lg:items-end'>
          <div>
            <p className='maas-section-kicker'>{t('The value chain')}</p>
            <h2
              id='value-chain-title'
              className='mt-4 max-w-2xl text-3xl leading-tight font-semibold tracking-[-0.035em] text-balance md:text-5xl'
            >
              {t('Access, source, operate')}
            </h2>
          </div>
          <p className='text-muted-foreground max-w-xl text-base leading-7 text-pretty lg:justify-self-end'>
            {t(
              'Start with a working gateway, then make supply easier to compare and every transaction easier to govern.'
            )}
          </p>
        </AnimateInView>

        <div className='mt-14 grid gap-4 lg:grid-cols-3'>
          {values.map((item, index) => (
            <AnimateInView
              key={item.number}
              delay={index * 90}
              animation='scale-in'
            >
              <Card className='maas-value-card h-full gap-0 rounded-3xl py-0'>
                <CardHeader className='gap-5 px-6 pt-6 md:px-7 md:pt-7'>
                  <div className='flex items-center justify-between gap-4'>
                    <span className='maas-value-icon flex size-11 items-center justify-center rounded-2xl'>
                      <HugeiconsIcon
                        icon={item.icon}
                        className='size-5'
                        strokeWidth={1.7}
                        aria-hidden='true'
                      />
                    </span>
                    <span className='text-muted-foreground/60 font-mono text-xs'>
                      {item.number}
                    </span>
                  </div>
                  <div>
                    <CardTitle className='text-xl tracking-tight'>
                      {item.title}
                    </CardTitle>
                    <CardDescription className='mt-3 leading-6 text-pretty'>
                      {item.description}
                    </CardDescription>
                  </div>
                </CardHeader>
                <CardContent className='mt-8 px-6 md:px-7'>
                  <div className='maas-value-rail h-1.5 overflow-hidden rounded-full'>
                    <div
                      className='maas-value-progress h-full rounded-full'
                      style={{ width: item.roadmap ? '38%' : '100%' }}
                    />
                  </div>
                </CardContent>
                <CardFooter className='maas-value-footer mt-6 justify-between rounded-b-3xl px-6 py-4 md:px-7'>
                  <span className='text-muted-foreground text-xs'>
                    {item.footer}
                  </span>
                  <Badge
                    variant={item.roadmap ? 'outline' : 'secondary'}
                    className={item.roadmap ? 'maas-roadmap-badge' : undefined}
                  >
                    {item.status}
                  </Badge>
                </CardFooter>
              </Card>
            </AnimateInView>
          ))}
        </div>
      </div>
    </section>
  )
}
