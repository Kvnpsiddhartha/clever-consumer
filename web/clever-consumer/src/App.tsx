import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api } from './api/client'
import type { AlertRule, Observation, ProductPreview, Tracker, User } from './types/api'

const defaultRules: AlertRule[] = [
  { type: 'price_drop' },
  { type: 'back_in_stock' },
]

function App() {
  const [user, setUser] = useState<User | null>(null)
  const [email, setEmail] = useState('')
  const [devLink, setDevLink] = useState('')
  const [url, setUrl] = useState('')
  const [country, setCountry] = useState('IN')
  const [timezone, setTimezone] = useState('Asia/Kolkata')
  const [intervalHours, setIntervalHours] = useState(24)
  const [targetPrice, setTargetPrice] = useState('')
  const [preview, setPreview] = useState<ProductPreview | null>(null)
  const [trackers, setTrackers] = useState<Tracker[]>([])
  const [selectedTracker, setSelectedTracker] = useState<Tracker | null>(null)
  const [observations, setObservations] = useState<Observation[]>([])
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    const token = new URLSearchParams(window.location.search).get('token')
    if (token) {
      verify(token)
      window.history.replaceState(null, '', '/')
      return
    }
    api.me().then(setUser).catch(() => setUser(null))
  }, [])

  useEffect(() => {
    if (!user) return
    setCountry(user.country)
    setTimezone(user.timezone)
    refreshTrackers()
  }, [user])

  const rules = useMemo(() => {
    const configured = [...defaultRules]
    const threshold = Number(targetPrice)
    if (threshold > 0) {
      configured.unshift({ type: 'target_price', threshold_price: threshold })
    }
    return configured
  }, [targetPrice])

  async function submitMagicLink(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const response = await api.requestMagicLink(email)
      setDevLink(response.dev_magic_link)
      setMessage('Magic link generated. Local development shows it here until SMTP delivery is wired.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to request magic link')
    } finally {
      setBusy(false)
    }
  }

  async function verify(token: string) {
    setBusy(true)
    try {
      const verified = await api.verifyMagicLink(token)
      setUser(verified)
      setMessage('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to verify magic link')
    } finally {
      setBusy(false)
    }
  }

  async function saveProfile() {
    const updated = await api.updateMe(country, timezone)
    setUser(updated)
  }

  async function refreshTrackers() {
    const response = await api.listTrackers()
    setTrackers(response.items)
  }

  async function submitPreview(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const created = await api.createPreview(url, country)
      setPreview(created)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Preview failed')
    } finally {
      setBusy(false)
    }
  }

  async function createTracker() {
    if (!preview) return
    setBusy(true)
    try {
      await api.createTracker(preview.id, intervalHours, country, rules)
      setPreview(null)
      setUrl('')
      setTargetPrice('')
      await refreshTrackers()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Tracker creation failed')
    } finally {
      setBusy(false)
    }
  }

  async function runNow(tracker: Tracker) {
    setBusy(true)
    try {
      await api.runTracker(tracker.id)
      await refreshTrackers()
      await loadObservations(tracker)
    } finally {
      setBusy(false)
    }
  }

  async function toggleTracker(tracker: Tracker) {
    if (tracker.status === 'paused') {
      await api.resumeTracker(tracker.id)
    } else {
      await api.pauseTracker(tracker.id)
    }
    await refreshTrackers()
  }

  async function loadObservations(tracker: Tracker) {
    setSelectedTracker(tracker)
    const response = await api.observations(tracker.id)
    setObservations(response.items)
  }

  if (!user) {
    return (
      <main className="auth-shell">
        <section className="auth-panel">
          <p className="eyebrow">Clever Consumer</p>
          <h1>Track product prices from any public product page.</h1>
          <form onSubmit={submitMagicLink} className="stack">
            <label>
              Email
              <input value={email} onChange={(event) => setEmail(event.target.value)} placeholder="you@example.com" />
            </label>
            <button disabled={busy}>{busy ? 'Sending...' : 'Send magic link'}</button>
          </form>
          {devLink && (
            <button className="secondary" onClick={() => verify(new URL(devLink).searchParams.get('token') ?? '')}>
              Use local magic link
            </button>
          )}
          {message && <p className="notice">{message}</p>}
        </section>
      </main>
    )
  }

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div>
          <p className="eyebrow">Clever Consumer</p>
          <h1>Watchlist</h1>
        </div>
        <div className="profile-box">
          <span>{user.email}</span>
          <label>
            Country
            <input value={country} maxLength={2} onChange={(event) => setCountry(event.target.value.toUpperCase())} />
          </label>
          <label>
            Timezone
            <input value={timezone} onChange={(event) => setTimezone(event.target.value)} />
          </label>
          <button className="secondary" onClick={saveProfile}>Save profile</button>
        </div>
      </aside>

      <section className="workspace">
        <form className="add-bar" onSubmit={submitPreview}>
          <input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="Paste a product URL" />
          <button disabled={busy || !url}>{busy ? 'Checking...' : 'Preview'}</button>
        </form>

        {message && <p className="notice">{message}</p>}

        {preview && (
          <section className="preview-panel">
            <div>
              <p className="eyebrow">Extraction preview</p>
              <h2>{preview.name}</h2>
              <p>{preview.url}</p>
            </div>
            <div className="price-lockup">
              <strong>{formatMoney(preview.current_price, preview.currency)}</strong>
              <span>{preview.availability.replaceAll('_', ' ')}</span>
            </div>
            <div className="controls-grid">
              <label>
                Interval
                <select value={intervalHours} onChange={(event) => setIntervalHours(Number(event.target.value))}>
                  <option value={6}>6 hours</option>
                  <option value={12}>12 hours</option>
                  <option value={24}>24 hours</option>
                  <option value={48}>2 days</option>
                  <option value={168}>7 days</option>
                </select>
              </label>
              <label>
                Target price
                <input value={targetPrice} onChange={(event) => setTargetPrice(event.target.value)} placeholder="Optional" />
              </label>
              <button onClick={createTracker} type="button" disabled={busy}>Confirm tracker</button>
            </div>
          </section>
        )}

        <section className="tracker-grid">
          {trackers.map((tracker) => (
            <article className="tracker-card" key={tracker.id}>
              <div>
                <p className="status">{tracker.status}</p>
                <h2>{tracker.name}</h2>
                <p>{tracker.country} · every {intervalLabel(tracker.interval_hours)}</p>
              </div>
              <div className="price-lockup">
                <strong>{formatMoney(tracker.current_price, tracker.currency)}</strong>
                <span>Next {formatDate(tracker.next_check_at)}</span>
              </div>
              <div className="button-row">
                <button className="secondary" onClick={() => loadObservations(tracker)}>History</button>
                <button className="secondary" onClick={() => runNow(tracker)} disabled={busy}>Run now</button>
                <button className="secondary" onClick={() => toggleTracker(tracker)}>
                  {tracker.status === 'paused' ? 'Resume' : 'Pause'}
                </button>
              </div>
            </article>
          ))}
        </section>

        {selectedTracker && (
          <section className="history-panel">
            <h2>{selectedTracker.name} history</h2>
            <div className="history-list">
              {observations.map((observation) => (
                <div key={observation.id} className="history-row">
                  <span>{formatDate(observation.observed_at)}</span>
                  <strong>{formatMoney(observation.price, observation.currency)}</strong>
                  <span>{observation.method}</span>
                </div>
              ))}
            </div>
          </section>
        )}
      </section>
    </main>
  )
}

function formatMoney(value: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value)
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function intervalLabel(hours: number) {
  if (hours <= 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  const remainder = hours % 24
  return remainder === 0 ? `${days}d` : `${days}d ${remainder}h`
}

export default App
