export type ProductPreview = {
  id: string
  url: string
  status: string
  name: string
  current_price: number
  currency: string
  availability: string
  country: string
  confidence: number
}

export type AlertRule = {
  type: 'target_price' | 'price_drop' | 'back_in_stock' | 'out_of_stock'
  threshold_price?: number
}
