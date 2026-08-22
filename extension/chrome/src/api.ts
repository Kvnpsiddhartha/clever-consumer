import type { AlertRule, ProductPreview } from './types'

const API_BASE = 'http://localhost:18080'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...init?.headers,
    },
  })
  if (!response.ok) {
    const body = await response.json().catch(() => null)
    throw new Error(body?.message ?? `Request failed with ${response.status}`)
  }
  return response.json() as Promise<T>
}

export const api = {
  me() {
    return request<{ country: string }>('/v1/me')
  },
  createPreview(url: string, country: string) {
    return request<ProductPreview>('/v1/product-previews', {
      method: 'POST',
      body: JSON.stringify({ url, country }),
    })
  },
  createTracker(previewId: string, intervalHours: number, country: string, rules: AlertRule[]) {
    return request('/v1/trackers', {
      method: 'POST',
      body: JSON.stringify({ preview_id: previewId, interval_hours: intervalHours, country, rules }),
    })
  },
}
