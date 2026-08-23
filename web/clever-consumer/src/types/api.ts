export type User = {
  id: string
  email: string
  country: string
  timezone: string
  created_at: string
}

export type ProductPreview = {
  id: string
  url: string
  status: 'pending' | 'ready' | 'needs_confirmation' | 'failed'
  name: string
  image_url: string
  current_price: number
  currency: string
  availability: string
  country: string
  confidence: number
  error?: string
}

export type AlertRule = {
  id?: string
  type: 'target_price' | 'price_drop' | 'back_in_stock' | 'out_of_stock'
  threshold_price?: number
  enabled?: boolean
}

export type Tracker = {
  id: string
  preview_id: string
  product_url: string
  name: string
  image_url: string
  currency: string
  country: string
  interval_hours: number
  status: 'active' | 'paused' | 'needs_attention'
  current_price: number
  availability: string
  last_checked_at?: string
  next_check_at: string
  rules: AlertRule[]
}

export type Observation = {
  id: string
  tracker_id: string
  price: number
  currency: string
  availability: string
  method: string
  confidence: number
  observed_at: string
}

export type ScraperCollector = {
  id: string
  profile_id: string
  provider: string
  external_collector_id: string
  status: 'active' | 'provisioning' | 'healing' | 'failed'
  purpose: 'default' | 'load_shard' | string
  url_pattern: string
  request_count: number
  success_count: number
  failure_count: number
  consecutive_structural_failures: number
  last_error?: string
  active_since?: string
  created_at: string
  updated_at: string
}

export type CollectorOperationsProfile = {
  id: string
  domain: string
  status: string
  request_count: number
  product_count: number
  latest_product_url?: string
  updated_at: string
  collectors: ScraperCollector[]
}
