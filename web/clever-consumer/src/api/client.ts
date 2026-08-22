import type { AlertRule, Observation, ProductPreview, Tracker, User } from '../types/api'

const API_BASE = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:18080'

type ItemList<T> = {
  items: T[]
}

type MagicLinkResponse = {
  id: string
  expires_at: string
  dev_magic_link: string
}

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
  if (response.status === 204) {
    return undefined as T
  }
  return response.json() as Promise<T>
}

export const api = {
  requestMagicLink(email: string) {
    return request<MagicLinkResponse>('/v1/auth/magic-links', {
      method: 'POST',
      body: JSON.stringify({ email }),
    })
  },
  verifyMagicLink(token: string) {
    return request<User>('/v1/auth/verify', {
      method: 'POST',
      body: JSON.stringify({ token }),
    })
  },
  me() {
    return request<User>('/v1/me')
  },
  updateMe(country: string, timezone: string) {
    return request<User>('/v1/me', {
      method: 'PATCH',
      body: JSON.stringify({ country, timezone }),
    })
  },
  createPreview(url: string, country: string) {
    return request<ProductPreview>('/v1/product-previews', {
      method: 'POST',
      body: JSON.stringify({ url, country }),
    })
  },
  createTracker(previewId: string, intervalHours: number, country: string, rules: AlertRule[]) {
    return request<Tracker>('/v1/trackers', {
      method: 'POST',
      body: JSON.stringify({ preview_id: previewId, interval_hours: intervalHours, country, rules }),
    })
  },
  listTrackers() {
    return request<ItemList<Tracker>>('/v1/trackers')
  },
  pauseTracker(id: string) {
    return request<void>(`/v1/trackers/${id}/pause`, { method: 'POST' })
  },
  resumeTracker(id: string) {
    return request<void>(`/v1/trackers/${id}/resume`, { method: 'POST' })
  },
  runTracker(id: string) {
    return request<Observation>(`/v1/trackers/${id}/run`, { method: 'POST' })
  },
  observations(id: string) {
    return request<ItemList<Observation>>(`/v1/trackers/${id}/observations`)
  },
}
