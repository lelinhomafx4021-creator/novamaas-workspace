import { useEffect, useMemo, useState } from 'react'

import { Button, Input, Picker, Text, View } from '@tarojs/components'
import Taro from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getPricingCatalog, type ModelPricing } from '@/api/models'
import { PageShell } from '@/components/page-shell'
import { usePageTitle } from '@/hooks/use-page-title'

import './index.scss'

const PAGE_SIZE = 30

export default function ModelsPage() {
  const { t } = useTranslation()
  const [models, setModels] = useState<ModelPricing[]>([])
  const [query, setQuery] = useState('')
  const [vendor, setVendor] = useState('')
  const [endpoint, setEndpoint] = useState('')
  const [group, setGroup] = useState('')
  const [limit, setLimit] = useState(PAGE_SIZE)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  usePageTitle('nav.models')

  const load = () => {
    setLoading(true)
    setError(false)
    getPricingCatalog()
      .then(setModels)
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const vendors = useMemo(
    () => [...new Set(models.map((model) => model.vendor_name).filter(Boolean) as string[])].sort(),
    [models]
  )
  const endpoints = useMemo(
    () => [...new Set(models.flatMap((model) => model.supported_endpoint_types ?? []))].sort(),
    [models]
  )
  const groups = useMemo(
    () => [...new Set(models.flatMap((model) => model.enable_groups ?? []))].sort(),
    [models]
  )
  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return models.filter((model) => {
      if (normalized && !`${model.model_name} ${model.description ?? ''}`.toLowerCase().includes(normalized)) return false
      if (vendor && model.vendor_name !== vendor) return false
      if (endpoint && !model.supported_endpoint_types?.includes(endpoint)) return false
      return !group || model.enable_groups.includes(group)
    })
  }, [endpoint, group, models, query, vendor])

  const picker = (label: string, values: string[], value: string, onChange: (value: string) => void) => {
    const options = ['', ...values]
    return (
      <Picker
        mode='selector'
        range={[t('common.all'), ...values]}
        value={Math.max(0, options.indexOf(value))}
        onChange={(event) => {
          onChange(options[Number(event.detail.value)] ?? '')
          setLimit(PAGE_SIZE)
        }}
      >
        <View className='mobile-picker'>{label}: {value || t('common.all')}</View>
      </Picker>
    )
  }

  return (
    <PageShell title={t('models.title')} description={t('models.description')}>
      <Input
        className='mobile-input'
        value={query}
        placeholder={t('models.search')}
        onInput={(event) => {
          setQuery(event.detail.value)
          setLimit(PAGE_SIZE)
        }}
      />
      <View className='models-filters'>
        {picker(t('models.vendor'), vendors, vendor, setVendor)}
        {picker(t('models.endpoint'), endpoints, endpoint, setEndpoint)}
        {picker(t('models.group'), groups, group, setGroup)}
      </View>
      <Text className='mobile-muted'>{t('models.count', { count: filtered.length })}</Text>

      {loading ? <Text className='mobile-empty'>{t('common.loading')}</Text> : null}
      {error ? (
        <View className='mobile-card mobile-stack'>
          <Text className='mobile-error'>{t('models.loadError')}</Text>
          <Button className='mobile-button mobile-button--secondary' onClick={load}>{t('common.retry')}</Button>
        </View>
      ) : null}
      {!loading && !error && filtered.length === 0 ? <Text className='mobile-empty'>{t('models.empty')}</Text> : null}
      <View className='mobile-stack'>
        {filtered.slice(0, limit).map((model) => (
          <View
            className='mobile-card model-card'
            key={model.model_name}
            onClick={() => Taro.navigateTo({ url: `/pages/model-detail/index?name=${encodeURIComponent(model.model_name)}` })}
          >
            <View className='mobile-row'>
              <Text className='mobile-card__title'>{model.model_name}</Text>
              <Text className='model-card__arrow'>›</Text>
            </View>
            <Text className='mobile-card__meta'>{model.vendor_name || t('common.unknown')}</Text>
            {model.description ? <Text className='mobile-card__meta'>{model.description}</Text> : null}
            <View className='mobile-chip-row'>
              {(model.supported_endpoint_types ?? []).slice(0, 4).map((item) => (
                <Text className='mobile-chip' key={item}>{item}</Text>
              ))}
            </View>
          </View>
        ))}
      </View>
      {limit < filtered.length ? (
        <Button className='mobile-button mobile-button--secondary' onClick={() => setLimit((value) => value + PAGE_SIZE)}>
          {t('common.loadMore')}
        </Button>
      ) : null}
    </PageShell>
  )
}
