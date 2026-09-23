export type PlanCatalogCurrency = 'CNY' | 'USD' | 'EUR' | 'HKD'
export type PlanCatalogPeriod = 'monthly' | 'quarterly' | 'yearly' | 'one_time' | 'custom'
export type PlanCatalogAccent = 'indigo' | 'emerald' | 'amber' | 'rose' | 'slate'

export interface PlanCatalogItem {
  id: number
  name: string
  subtitle: string
  description: string
  price: string
  original_price?: string | null
  currency: PlanCatalogCurrency
  billing_period: PlanCatalogPeriod
  badge: string
  accent: PlanCatalogAccent
  group_name: string
  provider: string
  rate_multiplier: string
  daily_limit_usd?: string | null
  weekly_limit_usd?: string | null
  monthly_limit_usd?: string | null
  benefits: string[]
  payment_url: string
  is_published: boolean
  is_featured: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export interface PlanCatalogInput {
  name: string
  subtitle: string
  description: string
  price: string
  original_price?: string | null
  currency: PlanCatalogCurrency
  billing_period: PlanCatalogPeriod
  badge: string
  accent: PlanCatalogAccent
  group_name: string
  provider: string
  rate_multiplier: string
  daily_limit_usd?: string | null
  weekly_limit_usd?: string | null
  monthly_limit_usd?: string | null
  benefits: string[]
  payment_url: string
  is_published: boolean
  is_featured: boolean
  sort_order: number
}
