import { FormEvent, useEffect, useMemo, useState } from 'react'
import { createRoot } from 'react-dom/client'
import { api } from './api'
import type { AlertRule, ProductPreview } from './types'
import './style.css'

function Popup() {
  const [tabUrl, setTabUrl] = useState('')
  const [country, setCountry] = useState('IN')
  const [intervalHours, setIntervalHours] = useState(24)
  const [targetPrice, setTargetPrice] = useState('')
  const [preview, setPreview] = useState<ProductPreview | null>(null)
  const [message, setMessage] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    chrome.tabs?.query({ active: true, currentWindow: true }, (tabs) => {
      setTabUrl(tabs[0]?.url ?? '')
    })
    api.me().then((me) => setCountry(me.country)).catch(() => setMessage('Sign in on the Clever Consumer web app first.'))
  }, [])

  const rules = useMemo<AlertRule[]>(() => {
    const value = Number(targetPrice)
    const output: AlertRule[] = [{ type: 'price_drop' }, { type: 'back_in_stock' }]
    if (value > 0) output.unshift({ type: 'target_price', threshold_price: value })
    return output
  }, [targetPrice])

  async function previewCurrentTab(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      setPreview(await api.createPreview(tabUrl, country))
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Preview failed')
    } finally {
      setBusy(false)
    }
  }

  async function confirmTracker() {
    if (!preview) return
    setBusy(true)
    try {
      await api.createTracker(preview.id, intervalHours, country, rules)
      setMessage('Tracker created.')
      setPreview(null)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Tracker creation failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main>
      <header>
        <strong>Clever Consumer</strong>
        <span>{country}</span>
      </header>
      <form onSubmit={previewCurrentTab}>
        <label>
          Current tab
          <input value={tabUrl} onChange={(event) => setTabUrl(event.target.value)} />
        </label>
        <button disabled={busy || !tabUrl}>{busy ? 'Checking...' : 'Preview product'}</button>
      </form>
      {preview && (
        <section>
          <p>{preview.name}</p>
          <strong>{preview.currency} {preview.current_price}</strong>
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
          <button onClick={confirmTracker} type="button" disabled={busy}>Create tracker</button>
        </section>
      )}
      {message && <p className="notice">{message}</p>}
    </main>
  )
}

createRoot(document.getElementById('root')!).render(<Popup />)
