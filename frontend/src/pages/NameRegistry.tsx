import { useState, useEffect } from 'react'
import { api } from '../api'

interface NameEntry {
  name: string
  pubkey: string
  timestamp: number
  is_mine: boolean
}

interface PendingName {
  name: string
  startedAt: number
}

const PENDING_STORAGE_KEY = 'dmail_pending_names'

function loadPendingNames(): PendingName[] {
  try {
    return JSON.parse(localStorage.getItem(PENDING_STORAGE_KEY) || '[]')
  } catch {
    return []
  }
}

function savePendingNames(names: PendingName[]) {
  localStorage.setItem(PENDING_STORAGE_KEY, JSON.stringify(names))
}

export default function NameRegistry() {
  const [name, setName] = useState('')
  const [myNames, setMyNames] = useState<NameEntry[]>([])
  const [pendingNames, setPendingNames] = useState<PendingName[]>(loadPendingNames)
  const [checking, setChecking] = useState(false)
  const [available, setAvailable] = useState<boolean | null>(null)
  const [registering, setRegistering] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const loadMyNames = async () => {
    try {
      const res = await api.getMyNames()
      setMyNames(res)

      // Remove any pending names that now appear in registered names.
      const registeredSet = new Set(res.map(n => n.name))
      const updated = loadPendingNames().filter(p => !registeredSet.has(p.name))
      savePendingNames(updated)
      setPendingNames(updated)
    } catch {}
  }

  useEffect(() => {
    loadMyNames()
    // Poll every 30s to catch completed registrations.
    const interval = setInterval(loadMyNames, 30000)
    return () => clearInterval(interval)
  }, [])

  const handleCheck = async () => {
    if (!name) return
    setChecking(true)
    setAvailable(null)
    setError('')
    try {
      await api.resolveName(name)
      setAvailable(false)
    } catch {
      setAvailable(true)
    } finally {
      setChecking(false)
    }
  }

  const handleRegister = async () => {
    if (!name) return
    setRegistering(true)
    setError('')
    setSuccess('')
    try {
      await api.registerName(name)
      // Track as pending.
      const newPending = [...pendingNames, { name, startedAt: Date.now() }]
      savePendingNames(newPending)
      setPendingNames(newPending)
      setSuccess(`Registration of ${name}.dmail started! Proof-of-work is computing in the background.`)
      setName('')
      setAvailable(null)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setRegistering(false)
    }
  }

  const isValid = /^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(name) && name.length >= 3 && name.length <= 32

  const formatElapsed = (startedAt: number) => {
    const mins = Math.floor((Date.now() - startedAt) / 60000)
    if (mins < 1) return 'just started'
    if (mins === 1) return '1 minute ago'
    return `${mins} minutes ago`
  }

  return (
    <div style={{ maxWidth: 650 }}>
      <div className="page-header">
        <h2>Name Registry</h2>
      </div>

      <div className="card" style={{ marginBottom: '1.5rem' }}>
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '1rem', marginBottom: '1rem' }}>
          <div className="icon-circle">@</div>
          <div>
            <h3 className="card-title" style={{ marginBottom: '0.25rem' }}>Claim Your .dmail Name</h3>
            <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', margin: 0 }}>
              Get a human-readable address like <strong>alice.dmail</strong> instead of a long public key.
              Names are permanent and secured by proof-of-work (~10 min).
            </p>
          </div>
        </div>

        {error && <div className="error-banner">{error}</div>}
        {success && <div className="success-banner">{success}</div>}

        <div className="field" style={{ marginBottom: '0.75rem' }}>
          <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            Choose a name
          </label>
          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <input
              type="text"
              placeholder="yourname"
              value={name}
              onChange={e => {
                setName(e.target.value.toLowerCase())
                setAvailable(null)
                setError('')
                setSuccess('')
              }}
              style={{ flex: 1 }}
            />
            <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem', whiteSpace: 'nowrap', fontWeight: 600 }}>
              .dmail
            </span>
          </div>
          {name && !isValid && (
            <p style={{ fontSize: '0.75rem', color: 'var(--danger)', marginTop: '0.25rem' }}>
              3-32 chars, lowercase letters, numbers, and hyphens. Must start/end with letter or number.
            </p>
          )}
          {available === true && (
            <p style={{ fontSize: '0.8rem', color: 'var(--success)', marginTop: '0.35rem' }}>
              {name}.dmail is available!
            </p>
          )}
          {available === false && (
            <p style={{ fontSize: '0.8rem', color: 'var(--danger)', marginTop: '0.35rem' }}>
              {name}.dmail is already taken.
            </p>
          )}
        </div>

        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button className="secondary" disabled={!isValid || checking} onClick={handleCheck}>
            {checking ? 'Checking...' : 'Check Availability'}
          </button>
          <button className="primary" disabled={!isValid || available !== true || registering} onClick={handleRegister}>
            {registering ? 'Submitting...' : 'Register Name'}
          </button>
        </div>
      </div>

      {/* Pending registrations */}
      {pendingNames.length > 0 && (
        <div style={{ marginBottom: '1.5rem' }}>
          <h3 className="section-title">Pending Registrations</h3>
          <div className="card-list">
            {pendingNames.map(p => (
              <div key={p.name} className="card-list-item">
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <div className="spinner" />
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>{p.name}.dmail</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                      Computing proof-of-work... started {formatElapsed(p.startedAt)}
                    </div>
                  </div>
                </div>
                <span className="status-badge status-pending">Pending</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Registered names */}
      <div>
        <h3 className="section-title">Your Names</h3>
        {myNames.length === 0 && pendingNames.length === 0 ? (
          <div className="empty-state-card">
            <div className="empty-icon">@</div>
            <h3>No names registered</h3>
            <p>Register a .dmail name to make your address easy to remember and share.</p>
          </div>
        ) : myNames.length === 0 ? (
          <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', padding: '0.5rem 0' }}>
            No completed registrations yet. Pending names will appear here once proof-of-work finishes.
          </p>
        ) : (
          <div className="card-list">
            {myNames.map(n => (
              <div key={n.name} className="card-list-item">
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                  <div className="icon-circle" style={{ background: 'var(--success)', fontSize: '0.8rem' }}>&#10003;</div>
                  <div>
                    <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>{n.name}.dmail</div>
                    <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>{n.pubkey}</div>
                  </div>
                </div>
                <span className="status-badge status-active">Active</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
