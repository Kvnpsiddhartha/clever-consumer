import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Activity,
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Check,
  Clock3,
  ExternalLink,
  Home,
  PanelLeftClose,
  PanelLeftOpen,
  Pause,
  Play,
  Plus,
  RefreshCw,
  ServerCog,
  Trash2,
  UserRound,
  Wrench,
  X,
} from 'lucide-react'
import { api } from './api/client'
import type { AlertRule, CollectorOperationsProfile, Observation, ProductPreview, ScraperCollector, Tracker, User } from './types/api'

const defaultRules: AlertRule[] = [
  { type: 'price_drop' },
  { type: 'back_in_stock' },
]

type SidebarTab = 'dashboard' | 'user' | 'scraper'
type RunPhase = 'starting' | 'running' | 'fetching' | 'saving' | 'complete' | 'failed'

type ProductRunStatus = {
  trackerId: string
  phase: RunPhase
  startedAtMs: number
  updatedAtMs: number
  finishedAtMs?: number
  observation?: Observation
  error?: string
}

function App() {
  const [user, setUser] = useState<User | null>(null)
  const [email, setEmail] = useState('')
  const [devLink, setDevLink] = useState('')
  const [sidebarOpen, setSidebarOpen] = useState(() =>
    typeof window === 'undefined' ? true : !window.matchMedia('(max-width: 860px)').matches,
  )
  const [activeSidebarTab, setActiveSidebarTab] = useState<SidebarTab>('dashboard')
  const [addModalOpen, setAddModalOpen] = useState(false)
  const [url, setUrl] = useState('')
  const [country, setCountry] = useState('IN')
  const [timezone, setTimezone] = useState('Asia/Kolkata')
  const [intervalHours, setIntervalHours] = useState(24)
  const [targetPrice, setTargetPrice] = useState('')
  const [preview, setPreview] = useState<ProductPreview | null>(null)
  const [trackers, setTrackers] = useState<Tracker[]>([])
  const [selectedTracker, setSelectedTracker] = useState<Tracker | null>(null)
  const [observations, setObservations] = useState<Observation[]>([])
  const [productRouteId, setProductRouteId] = useState(() => productRouteIdFromLocation())
  const [productTracker, setProductTracker] = useState<Tracker | null>(null)
  const [productObservations, setProductObservations] = useState<Observation[]>([])
  const [productLoading, setProductLoading] = useState(false)
  const [runStatus, setRunStatus] = useState<ProductRunStatus | null>(null)
  const [runNowMs, setRunNowMs] = useState(() => Date.now())
  const [collectorProfiles, setCollectorProfiles] = useState<CollectorOperationsProfile[]>([])
  const [collectorsLoading, setCollectorsLoading] = useState(false)
  const [healingCollectorId, setHealingCollectorId] = useState('')
  const [busy, setBusy] = useState(false)
  const [message, setMessage] = useState('')
  const addProductButtonRef = useRef<HTMLButtonElement>(null)
  const productUrlInputRef = useRef<HTMLInputElement>(null)
  const modalPanelRef = useRef<HTMLElement>(null)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('token')
    const authError = params.get('auth_error')
    if (token) {
      verify(token)
      window.history.replaceState(null, '', '/')
      return
    }
    if (authError) {
      setMessage('Google sign-in could not be completed. Try again or use a magic link.')
      window.history.replaceState(null, '', '/')
    }
    api.me().then(setUser).catch(() => setUser(null))
  }, [])

  useEffect(() => {
    function handlePopState() {
      setProductRouteId(productRouteIdFromLocation())
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    if (!user) return
    setCountry(user.country)
    setTimezone(user.timezone)
    refreshTrackers()
  }, [user])

  const loadProductPage = useCallback(async (trackerID: string) => {
    setProductLoading(true)
    setMessage('')
    try {
      const [tracker, response] = await Promise.all([
        api.getTracker(trackerID),
        api.observations(trackerID),
      ])
      setProductTracker(tracker)
      setProductObservations(response.items)
    } catch (error) {
      setProductTracker(null)
      setProductObservations([])
      setMessage(error instanceof Error ? error.message : 'Product could not be loaded')
    } finally {
      setProductLoading(false)
    }
  }, [])

  const refreshCollectors = useCallback(async () => {
    setCollectorsLoading(true)
    setMessage('')
    try {
      const response = await api.listCollectors()
      setCollectorProfiles(response.items)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Collectors could not be loaded')
    } finally {
      setCollectorsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!user || !productRouteId) return
    setActiveSidebarTab('dashboard')
    setSelectedTracker(null)
    loadProductPage(productRouteId)
  }, [loadProductPage, productRouteId, user])

  useEffect(() => {
    if (!user || activeSidebarTab !== 'scraper') return
    refreshCollectors()
  }, [activeSidebarTab, refreshCollectors, user])

  useEffect(() => {
    if (!runStatus || !isRunActive(runStatus)) return
    const ticker = window.setInterval(() => setRunNowMs(Date.now()), 1000)
    return () => window.clearInterval(ticker)
  }, [runStatus])

  useEffect(() => {
    if (!addModalOpen) return

    const focusFrame = window.requestAnimationFrame(() => productUrlInputRef.current?.focus())
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') {
        setAddModalOpen(false)
        setPreview(null)
        setUrl('')
        return
      }

      if (event.key !== 'Tab') return
      const focusable = modalPanelRef.current?.querySelectorAll<HTMLElement>(
        'button:not(:disabled), input:not(:disabled), select:not(:disabled), [href], [tabindex]:not([tabindex="-1"])',
      )
      if (!focusable?.length) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => {
      window.cancelAnimationFrame(focusFrame)
      window.removeEventListener('keydown', handleKeyDown)
      document.body.style.overflow = previousOverflow
      addProductButtonRef.current?.focus()
    }
  }, [addModalOpen])

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

  function startGoogleLogin() {
    window.location.assign(api.googleLoginUrl())
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

  async function saveProfile(event: FormEvent) {
    event.preventDefault()
    setBusy(true)
    setMessage('')
    try {
      const updated = await api.updateMe(country, timezone)
      setUser(updated)
      setMessage('Profile settings saved.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Unable to save profile settings')
    } finally {
      setBusy(false)
    }
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
      setAddModalOpen(false)
      await refreshTrackers()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Tracker creation failed')
    } finally {
      setBusy(false)
    }
  }

  function openAddProduct() {
    setAddModalOpen(true)
    setMessage('')
  }

  function closeAddProduct() {
    setAddModalOpen(false)
    setPreview(null)
    setUrl('')
  }

  function selectSidebarTab(tab: SidebarTab) {
    setActiveSidebarTab(tab)
    setMessage('')
    if (tab === 'dashboard') {
      setSelectedTracker(null)
      navigateToDashboard()
    } else if (productRouteId) {
      navigateToDashboard()
    }
    if (window.matchMedia('(max-width: 860px)').matches) setSidebarOpen(false)
  }

  function applyScraperDefaults() {
    setMessage('Scraper defaults updated for new trackers.')
  }

  async function healCollector(collector: ScraperCollector) {
    setHealingCollectorId(collector.id)
    setMessage('')
    try {
      const reason = collector.last_error || 'Manual Bright Data collector heal requested'
      await api.healCollector(collector.id, reason)
      await refreshCollectors()
      setMessage('Collector healing started.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Collector healing failed')
      await refreshCollectors().catch(() => undefined)
    } finally {
      setHealingCollectorId('')
    }
  }

  async function runNow(tracker: Tracker) {
    const startedAtMs = Date.now()
    const phaseTimers = [
      window.setTimeout(() => advanceRunPhase(tracker.id, startedAtMs, 'running'), 700),
      window.setTimeout(() => advanceRunPhase(tracker.id, startedAtMs, 'fetching'), 8000),
    ]

    setBusy(true)
    setMessage('')
    setRunNowMs(startedAtMs)
    setRunStatus({
      trackerId: tracker.id,
      phase: 'starting',
      startedAtMs,
      updatedAtMs: startedAtMs,
    })
    try {
      const observation = await api.runTracker(tracker.id)
      setRunStatus((current) => current?.trackerId === tracker.id && current.startedAtMs === startedAtMs
        ? { ...current, phase: 'saving', observation, updatedAtMs: Date.now() }
        : current)
      await refreshTrackers()
      if (productRouteId === tracker.id) {
        await loadProductPage(tracker.id)
      } else {
        await loadObservations(tracker)
      }
      const finishedAtMs = Date.now()
      setRunNowMs(finishedAtMs)
      setRunStatus((current) => current?.trackerId === tracker.id && current.startedAtMs === startedAtMs
        ? { ...current, phase: 'complete', observation, finishedAtMs, updatedAtMs: finishedAtMs }
        : current)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Manual run failed'
      const finishedAtMs = Date.now()
      setRunNowMs(finishedAtMs)
      setRunStatus((current) => current?.trackerId === tracker.id && current.startedAtMs === startedAtMs
        ? { ...current, phase: 'failed', error: message, finishedAtMs, updatedAtMs: finishedAtMs }
        : current)
      setMessage(message)
    } finally {
      phaseTimers.forEach((timer) => window.clearTimeout(timer))
      setBusy(false)
    }
  }

  function advanceRunPhase(trackerId: string, startedAtMs: number, phase: RunPhase) {
    setRunNowMs(Date.now())
    setRunStatus((current) => {
      if (!current || current.trackerId !== trackerId || current.startedAtMs !== startedAtMs || !isRunActive(current)) {
        return current
      }
      return { ...current, phase, updatedAtMs: Date.now() }
    })
  }

  async function toggleTracker(tracker: Tracker) {
    if (tracker.status === 'paused') {
      await api.resumeTracker(tracker.id)
    } else {
      await api.pauseTracker(tracker.id)
    }
    await refreshTrackers()
    if (productRouteId === tracker.id) {
      await loadProductPage(tracker.id)
    }
  }

  async function loadObservations(tracker: Tracker) {
    setSelectedTracker(tracker)
    setProductRouteId(null)
    setProductTracker(null)
    const response = await api.observations(tracker.id)
    setObservations(response.items)
  }

  function openProductPage(tracker: Tracker) {
    window.history.pushState(null, '', `/products/${encodeURIComponent(tracker.id)}`)
    setProductRouteId(tracker.id)
    setProductTracker(tracker)
    setSelectedTracker(null)
    setActiveSidebarTab('dashboard')
    setMessage('')
  }

  function navigateToDashboard() {
    if (window.location.pathname !== '/') {
      window.history.pushState(null, '', '/')
    }
    setProductRouteId(null)
    setProductTracker(null)
    setProductObservations([])
  }

  async function deleteTracker(tracker: Tracker) {
    const confirmed = window.confirm(`Delete "${tracker.name}"? This removes its price history and alert rules.`)
    if (!confirmed) return

    setBusy(true)
    setMessage('')
    try {
      await api.deleteTracker(tracker.id)
      await refreshTrackers()
      if (selectedTracker?.id === tracker.id) {
        setSelectedTracker(null)
        setObservations([])
      }
      if (productRouteId === tracker.id) {
        navigateToDashboard()
      }
      setMessage('Product deleted.')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Product deletion failed')
    } finally {
      setBusy(false)
    }
  }

  const currentProduct = productRouteId ? productTracker : null

  if (!user) {
    return (
      <main className="auth-shell">
        <section className="auth-panel" aria-labelledby="auth-title">
          <div className="auth-copy">
            <p className="eyebrow">Clever Consumer</p>
            <h1 id="auth-title">Sign in to track product prices.</h1>
            <p>Monitor public product pages, save watchlists, and get alerts from one account.</p>
          </div>

          <div className="login-card">
            <button className="provider-button" type="button" onClick={startGoogleLogin}>
              <span className="provider-mark" aria-hidden="true">G</span>
              Continue with Google
            </button>

            <div className="auth-divider" aria-hidden="true">
              <span />
              <strong>or</strong>
              <span />
            </div>

            <form onSubmit={submitMagicLink} className="stack">
              <label>
                Email
                <input
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  placeholder="you@example.com"
                  type="email"
                  autoComplete="email"
                  required
                />
              </label>
              <button className="primary" disabled={busy}>
                <span>{busy ? 'Sending...' : 'Send magic link'}</span>
                <ArrowRight aria-hidden="true" />
              </button>
            </form>

            {devLink && (
              <button className="secondary" onClick={() => verify(new URL(devLink).searchParams.get('token') ?? '')}>
                Use local magic link
              </button>
            )}
            {message && <p className="notice" role="status">{message}</p>}
          </div>
        </section>
      </main>
    )
  }

  return (
    <main className={`app-shell ${sidebarOpen ? '' : 'sidebar-collapsed'}`}>
      {sidebarOpen && <button className="sidebar-scrim" type="button" aria-label="Close sidebar" onClick={() => setSidebarOpen(false)} />}

      <aside className="sidebar" aria-label="Primary navigation">
        <div className="sidebar-title">
          <div className="brand-lockup">
            <span className="brand-mark" aria-hidden="true">CC</span>
            <div className="brand-copy">
              <p className="eyebrow">Clever Consumer</p>
              <h1>Watchlist</h1>
            </div>
          </div>
        </div>

        <nav className="nav-stack" aria-label="Primary">
          <button
            className={`nav-button ${activeSidebarTab === 'dashboard' ? 'active' : ''}`}
            type="button"
            aria-label="Dashboard home"
            aria-current={activeSidebarTab === 'dashboard' ? 'page' : undefined}
            title={sidebarOpen ? undefined : 'Dashboard home'}
            onClick={() => selectSidebarTab('dashboard')}
          >
            <Home aria-hidden="true" />
            <span>Dashboard home</span>
          </button>
          <button
            className={`nav-button ${activeSidebarTab === 'user' ? 'active' : ''}`}
            type="button"
            aria-label="User settings"
            aria-current={activeSidebarTab === 'user' ? 'page' : undefined}
            title={sidebarOpen ? undefined : 'User settings'}
            onClick={() => selectSidebarTab('user')}
          >
            <UserRound aria-hidden="true" />
            <span>User settings</span>
          </button>
          <button
            className={`nav-button ${activeSidebarTab === 'scraper' ? 'active' : ''}`}
            type="button"
            aria-label="Collectors"
            aria-current={activeSidebarTab === 'scraper' ? 'page' : undefined}
            title={sidebarOpen ? undefined : 'Collectors'}
            onClick={() => selectSidebarTab('scraper')}
          >
            <ServerCog aria-hidden="true" />
            <span>Collectors</span>
          </button>
        </nav>

      </aside>

      <section className="workspace">
        {activeSidebarTab === 'dashboard' && (
          <>
            <header className="workspace-header">
              <div className="workspace-title-row">
                <SidebarToggle open={sidebarOpen} onToggle={() => setSidebarOpen((open) => !open)} />
                <div>
                  <p className="eyebrow">{productRouteId ? 'Product detail' : 'Dashboard home'}</p>
                  <h1>{currentProduct ? currentProduct.name : 'Tracked products'}</h1>
                </div>
              </div>
              <div className="header-actions">
                {productRouteId && (
                  <button className="secondary" type="button" onClick={navigateToDashboard}>
                    <ArrowLeft aria-hidden="true" />
                    <span>All products</span>
                  </button>
                )}
                {!productRouteId && (
                  <button className="primary" type="button" ref={addProductButtonRef} onClick={openAddProduct}>
                    <span>Add product</span>
                    <Plus aria-hidden="true" />
                  </button>
                )}
              </div>
            </header>

            {message && <p className="notice" role="status" aria-live="polite">{message}</p>}

            {productRouteId && productLoading && (
              <section className="empty-state" aria-live="polite">
                <p className="eyebrow">Loading</p>
                <h2>Loading product.</h2>
              </section>
            )}

            {productRouteId && !productLoading && currentProduct && (
              <ProductDetail
                tracker={currentProduct}
                observations={productObservations}
                busy={busy}
                runStatus={runStatus?.trackerId === currentProduct.id ? runStatus : null}
                runNowMs={runNowMs}
                onRunNow={runNow}
                onToggle={toggleTracker}
                onDelete={deleteTracker}
              />
            )}

            {productRouteId && !productLoading && !currentProduct && (
              <section className="empty-state" aria-labelledby="product-missing-title">
                <p className="eyebrow">Product unavailable</p>
                <h2 id="product-missing-title">This tracked product could not be loaded.</h2>
              </section>
            )}

            {!productRouteId && trackers.length === 0 && (
              <section className="empty-state" aria-labelledby="empty-title">
                <p className="eyebrow">No trackers</p>
                <h2 id="empty-title">No tracked products yet.</h2>
              </section>
            )}

            {!productRouteId && (
              <section className="tracker-grid">
                {trackers.map((tracker) => (
                  <article className="tracker-card" key={tracker.id}>
                    <button className="tracker-link" type="button" onClick={() => openProductPage(tracker)} aria-label={`View ${tracker.name}`}>
                      <div className="product-media">
                        {tracker.image_url ? (
                          <img src={tracker.image_url} alt={tracker.name} loading="lazy" />
                        ) : (
                          <span>No image</span>
                        )}
                      </div>
                    </button>
                    <div className="tracker-card-body">
                      <div>
                        <p className={`status status-${tracker.status}`}>{tracker.status}</p>
                        <h2>{tracker.name}</h2>
                        <p className="product-description">{sourceLabel(tracker.product_url)} · {tracker.country} · every {intervalLabel(tracker.interval_hours)}</p>
                      </div>
                      <div className="price-lockup">
                        <strong>{formatMoney(tracker.current_price, tracker.currency)}</strong>
                        <span>{availabilityLabel(tracker.availability)}</span>
                      </div>
                      <div className="condition-list" aria-label="Price conditions">
                        {trackerConditions(tracker).map((condition) => (
                          <span key={condition}>{condition}</span>
                        ))}
                      </div>
                      <div className="tracker-meta">
                        <span>Next {formatDate(tracker.next_check_at)}</span>
                        {tracker.last_checked_at && <span>Last {formatDate(tracker.last_checked_at)}</span>}
                      </div>
                    </div>
                    <div className="button-row">
                      <button className="secondary small" type="button" onClick={() => openProductPage(tracker)}>
                        <ExternalLink aria-hidden="true" />
                        <span>View</span>
                      </button>
                      <button className="secondary small" type="button" onClick={() => loadObservations(tracker)}>
                        <Clock3 aria-hidden="true" />
                        <span>History</span>
                      </button>
                      <button className="secondary small" type="button" onClick={() => runNow(tracker)} disabled={busy}>
                        <Play aria-hidden="true" />
                        <span>Run now</span>
                      </button>
                      <button className="secondary small" type="button" onClick={() => toggleTracker(tracker)}>
                        {tracker.status === 'paused' ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
                        <span>{tracker.status === 'paused' ? 'Resume' : 'Pause'}</span>
                      </button>
                      <button className="secondary destructive small" type="button" onClick={() => deleteTracker(tracker)} disabled={busy}>
                        <Trash2 aria-hidden="true" />
                        <span>Delete</span>
                      </button>
                    </div>
                  </article>
                ))}
              </section>
            )}

            {!productRouteId && selectedTracker && (
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
          </>
        )}

        {activeSidebarTab === 'user' && (
          <section className="settings-page" aria-labelledby="user-settings-title">
            <header className="workspace-header">
              <div className="workspace-title-row">
                <SidebarToggle open={sidebarOpen} onToggle={() => setSidebarOpen((open) => !open)} />
                <div>
                  <p className="eyebrow">Settings</p>
                  <h1 id="user-settings-title">User settings</h1>
                </div>
              </div>
            </header>

            {message && <p className="notice" role="status" aria-live="polite">{message}</p>}

            <div className="settings-layout single-column">
              <section className="settings-section" aria-labelledby="profile-settings-title">
                <div className="settings-copy">
                  <p className="eyebrow">Account</p>
                  <h2 id="profile-settings-title">Profile preferences</h2>
                  <p>{user.email}</p>
                </div>
                <form className="settings-form" onSubmit={saveProfile}>
                  <label>
                    Country
                    <input value={country} maxLength={2} onChange={(event) => setCountry(event.target.value.toUpperCase())} />
                  </label>
                  <label>
                    Timezone
                    <input value={timezone} onChange={(event) => setTimezone(event.target.value)} />
                  </label>
                  <button className="primary settings-submit" disabled={busy}>
                    <span>{busy ? 'Saving...' : 'Save profile'}</span>
                    <Check aria-hidden="true" />
                  </button>
                </form>
              </section>
            </div>
          </section>
        )}

        {activeSidebarTab === 'scraper' && (
          <section className="settings-page collectors-page" aria-labelledby="collector-ops-title">
            <header className="workspace-header">
              <div className="workspace-title-row">
                <SidebarToggle open={sidebarOpen} onToggle={() => setSidebarOpen((open) => !open)} />
                <div>
                  <p className="eyebrow">Bright Data</p>
                  <h1 id="collector-ops-title">Collector operations</h1>
                </div>
              </div>
              <div className="header-actions">
                <button className="secondary" type="button" onClick={refreshCollectors} disabled={collectorsLoading}>
                  <RefreshCw aria-hidden="true" />
                  <span>{collectorsLoading ? 'Refreshing...' : 'Refresh'}</span>
                </button>
              </div>
            </header>

            {message && <p className="notice" role="status" aria-live="polite">{message}</p>}

            <div className="collector-summary-grid" aria-label="Collector totals">
              <MetricTile label="Domains" value={collectorProfiles.length} />
              <MetricTile label="Collectors" value={collectorTotal(collectorProfiles, 'all')} />
              <MetricTile label="Runs" value={collectorRunTotal(collectorProfiles)} />
              <MetricTile label="Errors" value={collectorErrorTotal(collectorProfiles)} tone={collectorErrorTotal(collectorProfiles) > 0 ? 'warning' : 'default'} />
            </div>

            <div className="settings-layout collector-settings-layout">
              <section className="settings-section" aria-labelledby="scraper-defaults-title">
                <div className="settings-copy">
                  <p className="eyebrow">Tracker defaults</p>
                  <h2 id="scraper-defaults-title">New product settings</h2>
                  <p>These defaults are applied when you add a product.</p>
                </div>
                <div className="settings-form">
                  <label>
                    Default interval
                    <select value={intervalHours} onChange={(event) => setIntervalHours(Number(event.target.value))}>
                      <option value={6}>6 hours</option>
                      <option value={12}>12 hours</option>
                      <option value={24}>24 hours</option>
                      <option value={48}>2 days</option>
                      <option value={168}>7 days</option>
                    </select>
                  </label>
                  <label>
                    Default target price
                    <input value={targetPrice} onChange={(event) => setTargetPrice(event.target.value)} placeholder="Optional" inputMode="decimal" />
                  </label>
                  <button className="primary settings-submit" type="button" onClick={applyScraperDefaults}>
                    <span>Apply defaults</span>
                    <Check aria-hidden="true" />
                  </button>
                </div>
              </section>

              <section className="settings-section" aria-labelledby="scraper-status-title">
                <div className="settings-copy">
                  <p className="eyebrow">Provider</p>
                  <h2 id="scraper-status-title">Bright Data flow</h2>
                </div>
                <div className="scraper-list">
                  <div>
                    <span>Scraper Studio collectors</span>
                    <strong>Enabled</strong>
                  </div>
                  <div>
                    <span>Unlocker and dataset fallback</span>
                    <strong>Enabled</strong>
                  </div>
                  <div>
                    <span>Self-healing refactor flow</span>
                    <strong>Enabled</strong>
                  </div>
                </div>
              </section>
            </div>

            {collectorsLoading && collectorProfiles.length === 0 && (
              <section className="empty-state" aria-live="polite">
                <p className="eyebrow">Loading</p>
                <h2>Loading collectors.</h2>
              </section>
            )}

            {!collectorsLoading && collectorProfiles.length === 0 && (
              <section className="empty-state" aria-labelledby="collectors-empty-title">
                <p className="eyebrow">No collector traffic</p>
                <h2 id="collectors-empty-title">Run a product preview to create collector telemetry.</h2>
              </section>
            )}

            {collectorProfiles.length > 0 && (
              <section className="collector-domain-list" aria-label="Bright Data collector domains">
                {collectorProfiles.map((profile) => (
                  <CollectorDomainPanel
                    key={profile.id}
                    profile={profile}
                    healingCollectorId={healingCollectorId}
                    onHeal={healCollector}
                  />
                ))}
              </section>
            )}
          </section>
        )}
      </section>

      {addModalOpen && (
        <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) closeAddProduct()
        }}>
          <section ref={modalPanelRef} className="modal-panel" role="dialog" aria-modal="true" aria-labelledby="add-product-title">
            <header className="modal-header">
              <div>
                <p className="eyebrow">Add tracker</p>
                <h2 id="add-product-title">Product details</h2>
              </div>
              <button className="icon-button" type="button" aria-label="Close add product" title="Close" onClick={closeAddProduct}>
                <X aria-hidden="true" />
              </button>
            </header>

            <form className="modal-form" onSubmit={submitPreview}>
              <label>
                Product URL
                <input ref={productUrlInputRef} value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://example.com/product" type="url" required />
              </label>
              <div className="controls-grid">
                <label>
                  Country
                  <input value={country} maxLength={2} onChange={(event) => setCountry(event.target.value.toUpperCase())} />
                </label>
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
                  <input value={targetPrice} onChange={(event) => setTargetPrice(event.target.value)} placeholder="Optional" inputMode="decimal" />
                </label>
              </div>
              <button className="primary modal-submit" disabled={busy || !url}>
                <span>{busy ? 'Checking...' : 'Preview product'}</span>
                <ArrowRight aria-hidden="true" />
              </button>
            </form>

            {preview && (
              <section className="preview-panel modal-preview">
                <div className="product-media">
                  {preview.image_url ? <img src={preview.image_url} alt={preview.name} /> : <span>No image</span>}
                </div>
                <div>
                  <p className="eyebrow">Extraction preview</p>
                  <h2>{preview.name}</h2>
                  <p>{preview.url}</p>
                </div>
                <div className="price-lockup">
                  <strong>{formatMoney(preview.current_price, preview.currency)}</strong>
                  <span>{availabilityLabel(preview.availability)}</span>
                </div>
                <div className="condition-list" aria-label="Configured price conditions">
                  {rules.map((rule) => (
                    <span key={`${rule.type}-${rule.threshold_price ?? 'default'}`}>{ruleLabel(rule, preview.currency)}</span>
                  ))}
                </div>
                <button className="primary" onClick={createTracker} type="button" disabled={busy}>
                  <span>Confirm tracker</span>
                  <ArrowRight aria-hidden="true" />
                </button>
              </section>
            )}
          </section>
        </div>
      )}
    </main>
  )
}

function ProductDetail({
  tracker,
  observations,
  busy,
  runStatus,
  runNowMs,
  onRunNow,
  onToggle,
  onDelete,
}: {
  tracker: Tracker
  observations: Observation[]
  busy: boolean
  runStatus: ProductRunStatus | null
  runNowMs: number
  onRunNow: (tracker: Tracker) => void
  onToggle: (tracker: Tracker) => void
  onDelete: (tracker: Tracker) => void
}) {
  const latestObservation = latestRun(observations)
  const running = runStatus ? isRunActive(runStatus) : false
  const runElapsedMs = runStatus ? (runStatus.finishedAtMs ?? runNowMs) - runStatus.startedAtMs : 0
  const displayedObservations = [...observations].reverse()

  return (
    <div className="product-detail-layout">
      <section className="product-detail-panel" aria-labelledby="product-detail-title">
        <div className="product-media product-detail-media">
          {tracker.image_url ? (
            <img src={tracker.image_url} alt={tracker.name} />
          ) : (
            <span>No image</span>
          )}
        </div>

        <div className="product-detail-summary">
          <div>
            <p className={`status status-${tracker.status}`}>{tracker.status}</p>
            <h2 id="product-detail-title">{tracker.name}</h2>
            <p className="product-description">{tracker.product_url}</p>
          </div>

          <div className="price-lockup detail-price">
            <strong>{formatMoney(tracker.current_price, tracker.currency)}</strong>
            <span>{availabilityLabel(tracker.availability)}</span>
          </div>

          <div className="condition-list" aria-label="Price conditions">
            {trackerConditions(tracker).map((condition) => (
              <span key={condition}>{condition}</span>
            ))}
          </div>

          <div className="product-detail-meta">
            <div>
              <span>Source</span>
              <strong>{sourceLabel(tracker.product_url)}</strong>
            </div>
            <div>
              <span>Country</span>
              <strong>{tracker.country}</strong>
            </div>
            <div>
              <span>Interval</span>
              <strong>{intervalLabel(tracker.interval_hours)}</strong>
            </div>
            <div>
              <span>Next check</span>
              <strong>{formatDate(tracker.next_check_at)}</strong>
            </div>
            {tracker.last_checked_at && (
              <div>
                <span>Last checked</span>
                <strong>{formatDate(tracker.last_checked_at)}</strong>
              </div>
            )}
          </div>

          <div className="button-row product-detail-actions">
            <button className="secondary small" type="button" onClick={() => onRunNow(tracker)} disabled={busy || running}>
              {running ? <RefreshCw className="run-spin" aria-hidden="true" /> : <Play aria-hidden="true" />}
              <span>{running ? 'Running...' : 'Run now'}</span>
            </button>
            <button className="secondary small" type="button" onClick={() => onToggle(tracker)}>
              {tracker.status === 'paused' ? <Play aria-hidden="true" /> : <Pause aria-hidden="true" />}
              <span>{tracker.status === 'paused' ? 'Resume' : 'Pause'}</span>
            </button>
            <a className="button-link secondary small" href={tracker.product_url} target="_blank" rel="noreferrer">
              <span>Open source</span>
              <ExternalLink aria-hidden="true" />
            </a>
            <button className="secondary destructive small" type="button" onClick={() => onDelete(tracker)} disabled={busy}>
              <Trash2 aria-hidden="true" />
              <span>Delete</span>
            </button>
          </div>
        </div>
      </section>

      <RunStatusCard
        runStatus={runStatus}
        elapsedMs={runElapsedMs}
        latestObservation={latestObservation}
        tracker={tracker}
      />

      <section className="history-panel product-history" aria-labelledby="product-history-title">
        <div className="section-title-row">
          <div>
            <p className="eyebrow">Runs</p>
            <h2 id="product-history-title">Price history</h2>
          </div>
          <span>{observations.length} total</span>
        </div>
        {observations.length > 0 ? (
          <div className="history-list">
            {displayedObservations.map((observation) => (
              <div key={observation.id} className="history-row">
                <div>
                  <strong>{formatDate(observation.observed_at)}</strong>
                  <span>{observation.method}</span>
                </div>
                <div>
                  <strong>{formatMoney(observation.price, observation.currency)}</strong>
                  <span>{availabilityLabel(observation.availability)}</span>
                </div>
                <div>
                  <strong>{Math.round(observation.confidence * 100)}%</strong>
                  <span>confidence</span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="empty-copy">No price observations yet.</p>
        )}
      </section>
    </div>
  )
}

function RunStatusCard({
  runStatus,
  elapsedMs,
  latestObservation,
  tracker,
}: {
  runStatus: ProductRunStatus | null
  elapsedMs: number
  latestObservation?: Observation
  tracker: Tracker
}) {
  const phase = runStatus?.phase ?? 'complete'
  const running = runStatus ? isRunActive(runStatus) : false
  const statusClass = runStatus ? `run-status-${runStatus.phase}` : 'run-status-ready'
  const statusLabel = runStatus ? runPhaseLabel(runStatus.phase) : 'Ready'
  const referenceObservation = runStatus?.observation ?? latestObservation

  return (
    <section className={`run-status-card ${statusClass}`} aria-labelledby="run-status-title" aria-live="polite">
      <header className="run-status-header">
        <div>
          <p className="eyebrow">Run monitor</p>
          <h2 id="run-status-title">{runStatus ? runPhaseTitle(runStatus.phase) : 'Ready for manual run'}</h2>
        </div>
        <span className="run-status-pill">
          {running ? <RefreshCw className="run-spin" aria-hidden="true" /> : phase === 'failed' ? <AlertTriangle aria-hidden="true" /> : <Check aria-hidden="true" />}
          {statusLabel}
        </span>
      </header>

      <div className="run-status-grid">
        <div>
          <span>Elapsed</span>
          <strong>{runStatus ? formatElapsed(elapsedMs) : 'Idle'}</strong>
        </div>
        <div>
          <span>Started</span>
          <strong>{runStatus ? formatTime(runStatus.startedAtMs) : 'Not running'}</strong>
        </div>
        <div>
          <span>Latest price</span>
          <strong>{referenceObservation ? formatMoney(referenceObservation.price, referenceObservation.currency) : formatMoney(tracker.current_price, tracker.currency)}</strong>
        </div>
        <div>
          <span>Source method</span>
          <strong>{referenceObservation?.method ?? 'waiting'}</strong>
        </div>
      </div>

      {runStatus?.error ? (
        <p className="run-error">
          <AlertTriangle aria-hidden="true" />
          <span>{runStatus.error}</span>
        </p>
      ) : (
        <ol className="run-step-list" aria-label="Manual run progress">
          {runPhases.map((step) => (
            <li key={step} className={`run-step run-step-${runStepState(step, runStatus?.phase)}`}>
              {runStepState(step, runStatus?.phase) === 'done' ? <Check aria-hidden="true" /> : <Clock3 aria-hidden="true" />}
              <span>{runPhaseLabel(step)}</span>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}

function MetricTile({ label, value, tone = 'default' }: { label: string; value: number | string; tone?: 'default' | 'warning' }) {
  return (
    <div className={`metric-tile ${tone === 'warning' ? 'metric-warning' : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function CollectorDomainPanel({
  profile,
  healingCollectorId,
  onHeal,
}: {
  profile: CollectorOperationsProfile
  healingCollectorId: string
  onHeal: (collector: ScraperCollector) => void
}) {
  const totals = collectorProfileTotals(profile)
  return (
    <article className="collector-domain-panel">
      <header className="collector-domain-header">
        <div>
          <p className="eyebrow">{profile.status}</p>
          <h2>{profile.domain}</h2>
          {profile.latest_product_url && <p>{profile.latest_product_url}</p>}
        </div>
        <div className="collector-domain-stats" aria-label={`${profile.domain} collector statistics`}>
          <MetricTile label="Requests" value={profile.request_count} />
          <MetricTile label="Products" value={profile.product_count} />
          <MetricTile label="Collectors" value={profile.collectors.length} />
          <MetricTile label="Success" value={`${totals.successRate}%`} tone={totals.failures > 0 ? 'warning' : 'default'} />
        </div>
      </header>

      {profile.collectors.length === 0 ? (
        <div className="collector-empty">
          <Activity aria-hidden="true" />
          <span>Generic scrape path is active while a Bright Data collector is discovered or provisioned.</span>
        </div>
      ) : (
        <div className="collector-table" role="table" aria-label={`${profile.domain} collectors`}>
          <div className="collector-table-head" role="row">
            <span role="columnheader">Collector</span>
            <span role="columnheader">Status</span>
            <span role="columnheader">Runs</span>
            <span role="columnheader">Errors</span>
            <span role="columnheader">Healing</span>
          </div>
          {profile.collectors.map((collector) => (
            <div className="collector-row" role="row" key={collector.id}>
              <div role="cell">
                <strong>{collector.external_collector_id || collector.id}</strong>
                <span>{collector.provider} · {collectorPurposeLabel(collector.purpose)}</span>
              </div>
              <div role="cell">
                <span className={`collector-status collector-status-${collector.status}`}>
                  {collector.status === 'healing' ? <Wrench aria-hidden="true" /> : <ServerCog aria-hidden="true" />}
                  {statusLabel(collector.status)}
                </span>
                <span>Updated {formatDate(collector.updated_at)}</span>
              </div>
              <div role="cell">
                <strong>{collector.request_count}</strong>
                <span>{collector.success_count} ok</span>
              </div>
              <div role="cell">
                <strong>{collector.failure_count}</strong>
                <span>{collector.consecutive_structural_failures} structural</span>
              </div>
              <div role="cell" className="collector-heal-cell">
                {collector.last_error ? (
                  <p className="collector-error">
                    <AlertTriangle aria-hidden="true" />
                    <span>{collector.last_error}</span>
                  </p>
                ) : (
                  <span className="collector-clean">No recent scrape error</span>
                )}
                <button
                  className="secondary small"
                  type="button"
                  onClick={() => onHeal(collector)}
                  disabled={!collector.external_collector_id || collector.status === 'healing' || healingCollectorId === collector.id}
                >
                  <Wrench aria-hidden="true" />
                  <span>{healingCollectorId === collector.id ? 'Healing...' : 'Heal'}</span>
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </article>
  )
}

function productRouteIdFromLocation() {
  const match = window.location.pathname.match(/^\/products\/([^/]+)$/)
  return match ? decodeURIComponent(match[1]) : null
}

function formatMoney(value: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(value)
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function formatTime(value: number) {
  return new Intl.DateTimeFormat(undefined, { timeStyle: 'medium' }).format(new Date(value))
}

function formatElapsed(value: number) {
  const totalSeconds = Math.max(0, Math.floor(value / 1000))
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  if (minutes === 0) return `${seconds}s`
  return `${minutes}m ${seconds.toString().padStart(2, '0')}s`
}

function intervalLabel(hours: number) {
  if (hours <= 24) return `${hours}h`
  const days = Math.floor(hours / 24)
  const remainder = hours % 24
  return remainder === 0 ? `${days}d` : `${days}d ${remainder}h`
}

function availabilityLabel(value: string) {
  return value.replaceAll('_', ' ')
}

function sourceLabel(value: string) {
  try {
    return new URL(value).hostname.replace(/^www\./, '')
  } catch {
    return value
  }
}

function trackerConditions(tracker: Tracker) {
  const configured = tracker.rules.length > 0 ? tracker.rules : defaultRules
  return configured.map((rule) => ruleLabel(rule, tracker.currency))
}

const runPhases: RunPhase[] = ['starting', 'running', 'fetching', 'saving']

function isRunActive(status: ProductRunStatus) {
  return status.phase !== 'complete' && status.phase !== 'failed'
}

function latestRun(observations: Observation[]) {
  return observations.at(-1)
}

function runPhaseLabel(phase: RunPhase) {
  switch (phase) {
    case 'starting':
      return 'Starting'
    case 'running':
      return 'Running scraper'
    case 'fetching':
      return 'Fetching details'
    case 'saving':
      return 'Saving result'
    case 'complete':
      return 'Completed'
    case 'failed':
      return 'Failed'
  }
}

function runPhaseTitle(phase: RunPhase) {
  switch (phase) {
    case 'starting':
      return 'Starting manual run'
    case 'running':
      return 'Scraper is running'
    case 'fetching':
      return 'Fetching product details'
    case 'saving':
      return 'Saving price observation'
    case 'complete':
      return 'Run completed'
    case 'failed':
      return 'Run failed'
  }
}

function runStepState(step: RunPhase, current?: RunPhase) {
  if (!current) return 'pending'
  if (current === 'failed') return 'pending'
  const currentIndex = runPhases.indexOf(current)
  const stepIndex = runPhases.indexOf(step)
  if (current === 'complete' || stepIndex < currentIndex) return 'done'
  if (stepIndex === currentIndex) return 'active'
  return 'pending'
}

function ruleLabel(rule: AlertRule, currency: string) {
  if (rule.type === 'target_price' && rule.threshold_price) {
    return `Target ${formatMoney(rule.threshold_price, currency)}`
  }
  return rule.type.replaceAll('_', ' ')
}

function collectorTotal(profiles: CollectorOperationsProfile[], status: 'all' | ScraperCollector['status']) {
  return profiles.reduce((sum, profile) => {
    return sum + profile.collectors.filter((collector) => status === 'all' || collector.status === status).length
  }, 0)
}

function collectorRunTotal(profiles: CollectorOperationsProfile[]) {
  return profiles.reduce((sum, profile) => sum + profile.collectors.reduce((collectorSum, collector) => collectorSum + collector.request_count, 0), 0)
}

function collectorErrorTotal(profiles: CollectorOperationsProfile[]) {
  return profiles.reduce((sum, profile) => sum + profile.collectors.reduce((collectorSum, collector) => collectorSum + collector.failure_count, 0), 0)
}

function collectorProfileTotals(profile: CollectorOperationsProfile) {
  const successes = profile.collectors.reduce((sum, collector) => sum + collector.success_count, 0)
  const failures = profile.collectors.reduce((sum, collector) => sum + collector.failure_count, 0)
  const total = successes + failures
  return {
    failures,
    successRate: total === 0 ? 0 : Math.round((successes / total) * 100),
  }
}

function collectorPurposeLabel(value: string) {
  return value.replaceAll('_', ' ')
}

function statusLabel(value: string) {
  return value.replaceAll('_', ' ')
}

function SidebarToggle({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  return (
    <button
      className="sidebar-toggle"
      type="button"
      aria-label={open ? 'Minimize sidebar' : 'Maximize sidebar'}
      aria-expanded={open}
      title={open ? 'Minimize sidebar' : 'Maximize sidebar'}
      onClick={onToggle}
    >
      {open ? <PanelLeftClose aria-hidden="true" /> : <PanelLeftOpen aria-hidden="true" />}
    </button>
  )
}

export default App
