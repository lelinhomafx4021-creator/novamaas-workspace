import { apiRequest } from './request'

export interface TopUpInfo {
  enable_redemption: boolean
  payment_compliance_confirmed: boolean
}

export interface TopUpRecord {
  amount: number
  complete_time: number
  create_time: number
  id: number
  money: number
  payment_method: string
  payment_provider: string
  status: string
  trade_no: string
}

export interface SubscriptionPlan {
  allow_balance_pay: boolean
  currency: string
  custom_seconds: number
  duration_unit: string
  duration_value: number
  id: number
  max_purchase_per_user: number
  price_amount: number
  quota_reset_period: string
  subtitle: string
  title: string
  total_amount: number
}

export interface UserSubscription {
  amount_total: number
  amount_used: number
  end_time: number
  id: number
  plan_id: number
  start_time: number
  status: string
}

export interface SelfSubscriptionData {
  all_subscriptions: Array<{ subscription: UserSubscription }>
  billing_preference: string
  subscriptions: Array<{ subscription: UserSubscription }>
}

interface Page<T> {
  items: T[]
  page: number
  page_size: number
  total: number
}

export function getTopUpInfo() {
  return apiRequest<TopUpInfo>('/api/user/topup/info')
}

export function getTopUps(page = 1, pageSize = 20) {
  return apiRequest<Page<TopUpRecord>>(
    `/api/user/topup/self?p=${page}&page_size=${pageSize}`
  )
}

export function redeemCode(key: string) {
  return apiRequest<number>('/api/user/topup', {
    data: { key },
    method: 'POST',
  })
}

export async function getSubscriptionPlans() {
  const records = await apiRequest<Array<{ plan: SubscriptionPlan }>>(
    '/api/subscription/plans'
  )
  return records.map((record) => record.plan)
}

export function getSelfSubscriptions() {
  return apiRequest<SelfSubscriptionData>('/api/subscription/self')
}

export function buySubscriptionWithBalance(planId: number) {
  return apiRequest<null>('/api/subscription/balance/pay', {
    data: { plan_id: planId },
    method: 'POST',
  })
}

export function updateBillingPreference(billingPreference: string) {
  return apiRequest<{ billing_preference: string }>(
    '/api/subscription/self/preference',
    {
      data: { billing_preference: billingPreference },
      method: 'PUT',
    }
  )
}
