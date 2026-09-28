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
  ArrowRight01Icon,
  Building02Icon,
  CloudServerIcon,
  ComputerProgramming01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { AnimateInView } from '@/components/animate-in-view'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

export function MarketNetwork() {
  const { t } = useTranslation()

  const steps = [
    {
      number: '01',
      icon: CloudServerIcon,
      title: t('List the supply'),
      description: t('Show token channels and service details'),
    },
    {
      number: '02',
      icon: Building02Icon,
      title: t('Review what matters'),
      description: t('Examine security, reliability and enterprise readiness'),
    },
    {
      number: '03',
      icon: ComputerProgramming01Icon,
      title: t('Match the need'),
      description: t('Compare options or find a better-fit channel'),
    },
  ]

  return (
    <section
      className='maas-deferred-section relative px-5 py-24 sm:px-6 md:py-32'
      aria-labelledby='market-network-title'
    >
      <div className='mx-auto grid max-w-7xl gap-10 lg:grid-cols-[minmax(0,0.85fr)_minmax(0,1.15fr)] lg:items-center lg:gap-16'>
        <AnimateInView animation='fade-right' className='max-w-xl'>
          <p className='maas-section-kicker'>{t('The token sourcing layer')}</p>
          <h2
            id='market-network-title'
            className='mt-4 text-3xl leading-tight font-semibold tracking-[-0.035em] text-balance md:text-5xl'
          >
            {t('Make token supply visible before you buy')}
          </h2>
          <p className='text-muted-foreground mt-6 text-base leading-7 text-pretty'>
            {t(
              'Our next step is a supply shelf: providers present token channels, the platform reviews security, reliability and enterprise fit, and customers compare options or describe what they need.'
            )}
          </p>
          <div className='mt-7 flex flex-wrap items-center gap-3'>
            <Badge variant='outline' className='maas-roadmap-badge'>
              {t('Roadmap')}
            </Badge>
            <span className='text-muted-foreground text-sm'>
              {t('A clearer way to compare supply')}
            </span>
          </div>
          <Button
            variant='outline'
            className='maas-secondary-action mt-8 rounded-full px-5'
            render={<Link to='/pricing' />}
          >
            {t('Explore model supply')}
            <HugeiconsIcon icon={ArrowRight01Icon} data-icon='inline-end' />
          </Button>
        </AnimateInView>

        <AnimateInView animation='fade-left' delay={100}>
          <div className='maas-sourcing-panel overflow-hidden rounded-[1.75rem] p-4 sm:p-6'>
            <div className='flex items-center justify-between gap-4 px-1 pb-5'>
              <span className='font-mono text-[10px] tracking-[0.15em] uppercase sm:text-xs'>
                {t('Planned sourcing flow')}
              </span>
              <span
                className='maas-sourcing-signal size-2 shrink-0 rounded-full'
                aria-hidden='true'
              />
            </div>
            <div className='grid gap-3'>
              {steps.map((step) => (
                <div
                  key={step.number}
                  className='maas-sourcing-step flex items-start gap-4 rounded-2xl p-4 sm:p-5'
                >
                  <span className='maas-market-icon flex size-11 shrink-0 items-center justify-center rounded-2xl'>
                    <HugeiconsIcon
                      icon={step.icon}
                      className='size-5'
                      strokeWidth={1.7}
                      aria-hidden='true'
                    />
                  </span>
                  <div className='min-w-0 flex-1'>
                    <div className='flex items-start justify-between gap-3'>
                      <h3 className='text-base font-semibold'>{step.title}</h3>
                      <span className='text-muted-foreground/60 font-mono text-xs'>
                        {step.number}
                      </span>
                    </div>
                    <p className='text-muted-foreground mt-1 text-sm leading-6'>
                      {step.description}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </AnimateInView>
      </div>
    </section>
  )
}
