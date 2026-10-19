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
  ArrowLeft01Icon,
  ArrowRight01Icon,
  BookOpen01Icon,
  Search01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { Link } from '@tanstack/react-router'
import { type ReactNode, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { DocumentationSwitcher } from '@/features/admin-manual/entry'
import { cn } from '@/lib/utils'

import { Document } from './document'

import '@/styles/api-docs.css'

export type APIDocument = { id: string; title: string; content: string }
const GATEWAY_URL = 'https://gateway.ai.shilijia.xyz'

export function DocumentationPage(props: {
  documents: APIDocument[]
  article?: string
  kind: 'api' | 'admin'
  manualAvailable: boolean
  status?: ReactNode
}) {
  const { t } = useTranslation()
  const [search, setSearch] = useState('')
  const [menuOpen, setMenuOpen] = useState(false)
  const article = props.article ?? props.documents[0]?.id
  const to = props.kind === 'admin' ? '/admin-manual' : '/docs'
  const title =
    props.kind === 'admin'
      ? t('Platform integration guide')
      : t('API documentation')
  const query = search.trim().toLocaleLowerCase()
  const results = useMemo(
    () =>
      props.documents.flatMap((doc) => {
        const text = `${doc.title} ${doc.content}`
          .replaceAll(/[#`*|]/g, '')
          .replaceAll(/\s+/g, ' ')
        const match = text.toLocaleLowerCase().indexOf(query)
        if (query && match < 0) return []
        const start = Math.max(0, match - 30)
        return [
          {
            ...doc,
            excerpt: text.slice(start, start + 110),
          },
        ]
      }),
    [props.documents, query]
  )
  const selectedIndex = props.documents.findIndex((doc) => doc.id === article)
  const selected = props.documents[selectedIndex]
  const previous = props.documents[selectedIndex - 1]
  const next = props.documents[selectedIndex + 1]

  let content: ReactNode = props.status
  if (!content && !selected) {
    content = (
      <div role='alert' className='space-y-4'>
        <h1 className='text-xl font-semibold'>{t('Document not found')}</h1>
        <Link to={to} search={{ article: 'quickstart' }}>
          {t('Quick start')}
        </Link>
      </div>
    )
  } else if (!content && selected) {
    content = (
      <article key={selected.id} className='min-w-0'>
        <header className='mb-10 flex flex-col gap-4'>
          <div className='flex items-center gap-2'>
            <span className='border-primary/20 bg-primary/10 text-primary inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-medium'>
              <HugeiconsIcon icon={BookOpen01Icon} size={13} aria-hidden />
              {title}
            </span>
          </div>
          <h1 className='text-foreground text-3xl font-extrabold tracking-tight sm:text-4xl'>
            {selected.title}
          </h1>
          {props.kind === 'api' && selected.id === 'quickstart' && (
            <section
              aria-label={t('Gateway URL')}
              className='border-border/80 from-muted/60 via-muted/30 to-card/60 hover:border-border flex min-w-0 items-center justify-between gap-4 rounded-xl border bg-gradient-to-r p-4 shadow-xs transition-all'
            >
              <div className='min-w-0 space-y-1'>
                <div className='flex items-center gap-2'>
                  <span className='relative flex size-2'>
                    <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75' />
                    <span className='relative inline-flex size-2 rounded-full bg-emerald-500' />
                  </span>
                  <span className='text-muted-foreground text-[11px] font-semibold tracking-wider uppercase'>
                    {t('Gateway URL')}
                  </span>
                </div>
                <a
                  href={GATEWAY_URL}
                  target='_blank'
                  rel='noopener noreferrer'
                  className='text-foreground hover:text-primary block font-mono text-sm font-semibold tracking-tight break-all transition-colors hover:underline'
                >
                  {GATEWAY_URL}
                </a>
              </div>
              <CopyButton
                value={GATEWAY_URL}
                size='sm'
                className='shrink-0'
                aria-label={`${t('Copy')} ${t('Gateway URL')}`}
              >
                {t('Copy')}
              </CopyButton>
            </section>
          )}
        </header>
        <Document content={selected.content} to={to} />
        <footer className='border-border/60 mt-14 grid grid-cols-2 gap-4 border-t pt-8 text-sm'>
          {[previous, next].map(
            (neighbor, index) =>
              neighbor && (
                <Link
                  key={neighbor.id}
                  to={to}
                  search={{ article: neighbor.id }}
                  className={cn(
                    'group flex flex-col justify-between gap-2.5 rounded-xl border border-border/70 bg-card/60 p-4 transition-all duration-200 hover:border-primary/50 hover:bg-muted/40 hover:shadow-xs',
                    index === 1 && 'col-start-2 text-right'
                  )}
                >
                  <span
                    className={cn(
                      'text-muted-foreground flex items-center gap-1.5 text-xs font-medium',
                      index === 1 && 'justify-end'
                    )}
                  >
                    <HugeiconsIcon
                      icon={index === 0 ? ArrowLeft01Icon : ArrowRight01Icon}
                      size={14}
                      aria-hidden
                      className={cn(
                        'transition-transform duration-200',
                        index === 0
                          ? 'group-hover:-translate-x-1'
                          : 'order-last group-hover:translate-x-1'
                      )}
                    />
                    {index === 0 ? t('Previous') : t('Next')}
                  </span>
                  <span className='group-hover:text-primary text-foreground text-sm font-semibold transition-colors sm:text-base'>
                    {neighbor.title}
                  </span>
                </Link>
              )
          )}
        </footer>
      </article>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      headerProps={{ activeNavigationHref: '/docs' }}
    >
      <main className='api-docs mx-auto grid w-full max-w-[1440px] gap-8 px-4 pt-24 pb-20 sm:px-6 lg:grid-cols-[17rem_minmax(0,1fr)] lg:gap-12 lg:px-8'>
        <aside className='lg:border-border/60 min-w-0 self-start border-b pb-6 lg:sticky lg:top-24 lg:max-h-[calc(100svh-7rem)] lg:overflow-y-auto lg:border-r lg:border-b-0 lg:pr-6'>
          {props.manualAvailable && <DocumentationSwitcher kind={props.kind} />}
          <Button
            variant='outline'
            className='mb-3 w-full justify-between lg:hidden'
            aria-expanded={menuOpen}
            aria-controls='docs-navigation'
            onClick={() => setMenuOpen(!menuOpen)}
          >
            {t('Documentation menu')}
            <span aria-hidden>{menuOpen ? '−' : '+'}</span>
          </Button>
          <div
            id='docs-navigation'
            className={cn(
              'flex-col gap-5 lg:flex',
              menuOpen ? 'flex' : 'hidden'
            )}
          >
            <div className='flex items-center justify-between px-1'>
              <div className='flex items-center gap-2.5'>
                <span className='border-primary/20 bg-primary/10 text-primary flex size-7 items-center justify-center rounded-lg border'>
                  <HugeiconsIcon icon={BookOpen01Icon} size={16} aria-hidden />
                </span>
                <span className='text-foreground text-sm font-bold tracking-tight'>
                  {title}
                </span>
              </div>
              {props.kind === 'api' && (
                <span className='border-border/70 bg-muted/50 text-muted-foreground rounded-md border px-1.5 py-0.5 font-mono text-[10px] font-semibold'>
                  REST
                </span>
              )}
            </div>
            <label htmlFor='docs-search' className='sr-only'>
              {t('Search documentation')}
            </label>
            <InputGroup className='bg-background/80'>
              <InputGroupAddon>
                <HugeiconsIcon icon={Search01Icon} size={15} aria-hidden />
              </InputGroupAddon>
              <InputGroupInput
                id='docs-search'
                value={search}
                placeholder={t('Search paths, parameters, errors…')}
                onChange={(event) => setSearch(event.target.value)}
                type='search'
              />
            </InputGroup>
            {query && (
              <p
                className='text-muted-foreground text-xs font-medium'
                role='status'
              >
                {t('{{count}} results', { count: results.length })}
              </p>
            )}
            <nav aria-label={title} className='flex flex-col gap-1'>
              {!props.status &&
                results.map((doc, docIdx) => {
                  const isActive = doc.id === article
                  return (
                    <Link
                      key={doc.id}
                      to={to}
                      search={{ article: doc.id }}
                      onClick={() => setMenuOpen(false)}
                      aria-current={isActive ? 'page' : undefined}
                      className={cn(
                        'group block rounded-lg px-3 py-2 text-sm transition-all',
                        isActive
                          ? 'bg-primary/10 text-primary font-semibold shadow-2xs'
                          : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground'
                      )}
                    >
                      <span className='flex items-center justify-between gap-2'>
                        <span className='flex items-center gap-2.5 truncate'>
                          <span
                            className={cn(
                              'font-mono text-[11px] tabular-nums',
                              isActive
                                ? 'text-primary'
                                : 'text-muted-foreground/60'
                            )}
                            aria-hidden
                          >
                            {String(docIdx + 1).padStart(2, '0')}
                          </span>
                          <span className='truncate'>{doc.title}</span>
                        </span>
                        {isActive && (
                          <HugeiconsIcon
                            icon={ArrowRight01Icon}
                            size={14}
                            aria-hidden
                            className='shrink-0'
                          />
                        )}
                      </span>
                      {query && (
                        <p className='text-muted-foreground bg-muted/40 mt-1.5 line-clamp-2 rounded p-1.5 text-xs leading-5 font-normal'>
                          {doc.excerpt}
                        </p>
                      )}
                    </Link>
                  )
                })}
            </nav>
            {query && results.length === 0 && (
              <div className='space-y-2 text-sm'>
                <p className='text-muted-foreground'>
                  {t('No documentation found')}
                </p>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => setSearch('')}
                >
                  {t('Clear search')}
                </Button>
              </div>
            )}
          </div>
        </aside>
        <div className='min-w-0'>{content}</div>
      </main>
    </PublicLayout>
  )
}
