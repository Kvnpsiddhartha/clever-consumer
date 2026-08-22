import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
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
  Settings2,
  Trash2,
  UserRound,
  X,
} from 'lucide-react'
import { api } from './api/client'
import type { AlertRule, Observation, ProductPreview, Tracker, User } from './types/api'

const defaultRules: AlertRule[] = [
  { type: 'price_drop' },
  { type: 'back_in_stock' },
]

type SidebarTab = 'dashboard' | 'user' | 'scraper'

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

  useEffect(() => {
    if (!user || !productRouteId) return
    setActiveSidebarTab('dashboard')
    setSelectedTracker(null)
    loadProductPage(productRouteId)
  }, [loadProductPage, productRouteId, user])

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

  async function runNow(tracker: Tracker) {
    setBusy(true)
    setMessage('')
    try {
      await api.runTracker(tracker.id)
      await refreshTrackers()
      if (productRouteId === tracker.id) {
        await loadProductPage(tracker.id)
      } else {
        await loadObservations(tracker)
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : 'Manual run failed')
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
            aria-label="Scraper settings"
            aria-current={activeSidebarTab === 'scraper' ? 'page' : undefined}
            title={sidebarOpen ? undefined : 'Scraper settings'}
            onClick={() => selectSidebarTab('scraper')}
          >
            <Settings2 aria-hidden="true" />
            <span>Scraper settings</span>
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
          <section className="settings-page" aria-labelledby="scraper-settings-title">
            <header className="workspace-header">
              <div className="workspace-title-row">
                <SidebarToggle open={sidebarOpen} onToggle={() => setSidebarOpen((open) => !open)} />
                <div>
                  <p className="eyebrow">Settings</p>
                  <h1 id="scraper-settings-title">Scraper settings</h1>
                </div>
              </div>
            </header>

            {message && <p className="notice" role="status" aria-live="polite">{message}</p>}

            <div className="settings-layout">
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
                  <p className="eyebrow">Services</p>
                  <h2 id="scraper-status-title">Scraper status</h2>
                </div>
                <div className="scraper-list">
                  <div>
                    <span>Product preview scraper</span>
                    <strong>Configured</strong>
                  </div>
                  <div>
                    <span>Price and availability checks</span>
                    <strong>Configured</strong>
                  </div>
                </div>
              </section>
            </div>
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
  onRunNow,
  onToggle,
  onDelete,
}: {
  tracker: Tracker
  observations: Observation[]
  busy: boolean
  onRunNow: (tracker: Tracker) => void
  onToggle: (tracker: Tracker) => void
  onDelete: (tracker: Tracker) => void
}) {
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
            <button className="secondary small" type="button" onClick={() => onRunNow(tracker)} disabled={busy}>
              <Play aria-hidden="true" />
              <span>Run now</span>
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

      <section className="history-panel product-history" aria-labelledby="product-history-title">
        <h2 id="product-history-title">Price history</h2>
        {observations.length > 0 ? (
          <div className="history-list">
            {observations.map((observation) => (
              <div key={observation.id} className="history-row">
                <span>{formatDate(observation.observed_at)}</span>
                <strong>{formatMoney(observation.price, observation.currency)}</strong>
                <span>{observation.method}</span>
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

function ruleLabel(rule: AlertRule, currency: string) {
  if (rule.type === 'target_price' && rule.threshold_price) {
    return `Target ${formatMoney(rule.threshold_price, currency)}`
  }
  return rule.type.replaceAll('_', ' ')
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
