import { useEffect, useState } from 'react'

import { Text, View } from '@tarojs/components'
import { useRouter } from '@tarojs/taro'
import { useTranslation } from 'react-i18next'

import { getPricingCatalog, type ModelPricing } from '@/api/models'
import { PageShell } from '@/components/page-shell'
import { useQuotaDisplay } from '@/currency/context'
import { usePageTitle } from '@/hooks/use-page-title'

export default function ModelDetailPage() {
  const { t } = useTranslation()
  const { formatBillingCurrencyFromUSD } = useQuotaDisplay()
  const router = useRouter()
  const modelName = decodeURIComponent(router.params.name ?? '')
  const [model, setModel] = useState<ModelPricing | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  usePageTitle('models.detail')

  useEffect(() => {
    getPricingCatalog()
      .then((items) => {
        const found = items.find((item) => item.model_name === modelName)
        setModel(found ?? null)
        setError(!found)
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }, [modelName])

  return (
    <PageShell title={modelName || t('models.detail')} description={t('models.detailDescription')}>
      {loading ? <Text className='mobile-empty'>{t('common.loading')}</Text> : null}
      {error ? <Text className='mobile-error'>{t('models.loadError')}</Text> : null}
      {model ? (
        <>
          <View className='mobile-card'>
            <Text className='mobile-card__title'>{t('models.capabilities')}</Text>
            <Text className='mobile-card__meta'>{model.description || t('common.unknown')}</Text>
            <View className='mobile-chip-row'>
              {[...(model.capabilities ?? []), ...(model.input_modalities ?? []), ...(model.output_modalities ?? [])].map((item) => (
                <Text className='mobile-chip' key={item}>{item}</Text>
              ))}
            </View>
            <Text className='mobile-card__meta'>{t('models.context')}: {model.context_length ?? '—'}</Text>
            <Text className='mobile-card__meta'>{t('models.maxOutput')}: {model.max_output_tokens ?? '—'}</Text>
          </View>
          <View className='mobile-card'>
            <Text className='mobile-card__title'>{t('models.endpoints')}</Text>
            <View className='mobile-chip-row'>
              {(model.supported_endpoint_types ?? []).map((item) => <Text className='mobile-chip' key={item}>{item}</Text>)}
            </View>
            <Text className='mobile-card__meta'>{t('models.groups')}: {model.enable_groups.join(', ') || '—'}</Text>
          </View>
          <View className='mobile-card'>
            <Text className='mobile-card__title'>{t('models.pricing')}</Text>
            <Text className='mobile-card__meta'>{t('models.pricingNotice')}</Text>
            <Text className='mobile-card__meta'>{t('models.billingMode')}: {model.billing_mode || model.quota_type || '—'}</Text>
            <Text className='mobile-card__meta'>{t('models.modelRatio')}: {model.model_ratio ?? '—'}</Text>
            <Text className='mobile-card__meta'>{t('models.completionRatio')}: {model.completion_ratio ?? '—'}</Text>
            <Text className='mobile-card__meta'>
              {t('models.modelPrice')}: {model.model_price === undefined
                ? '—'
                : formatBillingCurrencyFromUSD(model.model_price)}
            </Text>
          </View>
        </>
      ) : null}
    </PageShell>
  )
}
