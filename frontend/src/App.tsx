import { useState, useEffect, useCallback } from 'react'
import { Routes, Route, NavLink, useNavigate } from 'react-router-dom'
import { api, Identity, Contact, Status } from './api'
import { isLoggedIn, clearAuth } from './auth'
import Onboarding from './pages/Onboarding'
import Inbox from './pages/Inbox'
import MessageDetail from './pages/MessageDetail'
import Compose from './pages/Compose'
import Contacts from './pages/Contacts'
import NameRegistry from './pages/NameRegistry'
import Login from './pages/Login'
import Signup from './pages/Signup'

export default function App() {
  const [identity, setIdentity] = useState<Identity | null>(null)
  const [contacts, setContacts] = useState<Contact[]>([])
  const [status, setStatus] = useState<Status | null>(null)
  const [onboarded, setOnboarded] = useState(false)
  const [loading, setLoading] = useState(true)
  const [daemonError, setDaemonError] = useState(false)
  const [needsAuth, setNeedsAuth] = useState(false)
  const navigate = useNavigate()

  const loadContacts = useCallback(async () => {
    try {
      setContacts(await api.listContacts())
    } catch {}
  }, [])

  const initApp = useCallback(async () => {
    setLoading(true)
    try {
      const [id, st] = await Promise.all([api.getIdentity(), api.getStatus()])
      setIdentity(id)
      setStatus(st)
      setDaemonError(false)
      setNeedsAuth(false)
      await loadContacts()

      if (id.address) {
        setOnboarded(true)
      }
    } catch (err: any) {
      // If we get 401, it means multi-tenant mode requires login.
      if (err.message?.includes('unauthorized') || err.message?.includes('missing') || err.message?.includes('invalid')) {
        // Check if status endpoint works (it's public in multi-tenant mode).
        try {
          const st = await api.getStatus()
          setStatus(st)
          setNeedsAuth(true)
          setDaemonError(false)
        } catch {
          setDaemonError(true)
        }
      } else {
        setDaemonError(true)
      }
    }
    setLoading(false)
  }, [loadContacts])

  useEffect(() => {
    initApp()

    // Poll status every 10s.
    const interval = setInterval(async () => {
      try {
        setStatus(await api.getStatus())
        setDaemonError(false)
      } catch {
        setDaemonError(true)
      }
    }, 10000)

    return () => clearInterval(interval)
  }, [initApp])

  // Helper to resolve petnames.
  const resolveName = useCallback(
    (pubkey: string) => {
      const contact = contacts.find(c => c.pubkey === pubkey)
      return contact ? contact.petname : pubkey
    },
    [contacts],
  )

  const handleLogin = () => {
    setNeedsAuth(false)
    initApp()
    navigate('/')
  }

  const handleLogout = () => {
    clearAuth()
    setIdentity(null)
    setOnboarded(false)
    setNeedsAuth(true)
    navigate('/login')
  }

  if (loading) {
    return (
      <div className="onboarding">
        <div className="spinner" style={{ width: 32, height: 32 }} />
        <p style={{ marginTop: '1rem', color: 'var(--text-muted)' }}>Connecting to daemon...</p>
      </div>
    )
  }

  if (daemonError) {
    return (
      <div className="onboarding">
        <h1>Dmail</h1>
        <p className="subtitle">Cannot connect to the Dmail daemon.</p>
        <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', maxWidth: 400 }}>
          Make sure <code>dmaild</code> is running on port 7777. Start it with:
        </p>
        <div className="mnemonic-box" style={{ fontSize: '0.9rem', marginTop: '1rem' }}>
          ./dmaild --port 7777
        </div>
        <button className="primary" style={{ marginTop: '1.5rem' }} onClick={() => window.location.reload()}>
          Retry
        </button>
      </div>
    )
  }

  // Multi-tenant auth mode: show login/signup if no valid token.
  if (needsAuth && !isLoggedIn()) {
    return (
      <Routes>
        <Route path="/signup" element={<Signup onLogin={handleLogin} />} />
        <Route path="*" element={<Login onLogin={handleLogin} />} />
      </Routes>
    )
  }

  if (!onboarded) {
    return (
      <Onboarding
        identity={identity!}
        onComplete={() => {
          setOnboarded(true)
          navigate('/')
        }}
      />
    )
  }

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-header">
          <h1>Dmail</h1>
          <div className="address">{identity?.address}</div>
        </div>
        <nav>
          <NavLink to="/" end className={({ isActive }) => isActive ? 'active' : ''}>
            Inbox
          </NavLink>
          <NavLink to="/sent" className={({ isActive }) => isActive ? 'active' : ''}>
            Sent
          </NavLink>
          <NavLink to="/compose" className={({ isActive }) => isActive ? 'active' : ''}>
            Compose
          </NavLink>
          <NavLink to="/contacts" className={({ isActive }) => isActive ? 'active' : ''}>
            Contacts
          </NavLink>
          <NavLink to="/names" className={({ isActive }) => isActive ? 'active' : ''}>
            Names
          </NavLink>
        </nav>
        <div className="sidebar-footer">
          {status ? `${status.connected_peers} peers` : 'Disconnected'}
          {status && status.pending_pow_tasks > 0 && (
            <span className="badge">{status.pending_pow_tasks} stamping</span>
          )}
          {isLoggedIn() && (
            <button
              className="secondary"
              style={{ marginTop: '0.5rem', fontSize: '0.75rem', padding: '0.25rem 0.5rem', width: '100%' }}
              onClick={handleLogout}
            >
              Sign Out
            </button>
          )}
        </div>
      </aside>
      <main className="main">
        <Routes>
          <Route path="/" element={<Inbox folder="inbox" resolveName={resolveName} />} />
          <Route path="/sent" element={<Inbox folder="sent" resolveName={resolveName} />} />
          <Route path="/message/:id" element={<MessageDetail resolveName={resolveName} />} />
          <Route
            path="/compose"
            element={<Compose contacts={contacts} identity={identity!} />}
          />
          <Route
            path="/contacts"
            element={<Contacts contacts={contacts} onUpdate={loadContacts} />}
          />
          <Route path="/names" element={<NameRegistry />} />
        </Routes>
      </main>
    </div>
  )
}
