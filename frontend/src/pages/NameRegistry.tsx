import { useState, useEffect } from 'react'
import { api } from '../api'

interface NameEntry {
  name: string
  pubkey: string
  timestamp: number
  is_mine: boolean
}

export default function NameRegistry() {
  const [name, setName] = useState('')
  const [myNames, setMyNames] = useState<NameEntry[]>([])
  const [checking, setChecking] = useState(false)
  const [available, setAvailable] = useState<boolean | null>(null)
  const [registering, setRegistering] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const loadMyNames = async () => {
    try {
      const res = await api.getMyNames()
      setMyNames(res)
    } catch {}
  }

  useEffect(() => {
    loadMyNames()
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
      setSuccess(`Registration of ${name}.dmail started. PoW computation takes ~10 minutes.`)
      setName('')
      setAvailable(null)
      loadMyNames()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setRegistering(false)
    }
  }

  const isValid = /^[a-z0-9][a-z0-9-]*[a-z0-9]$/.test(name) && name.length >= 3 && name.length <= 32

  return (
    <div className="compose-form">
      <div className="page-header">
        <h2>Name Registry</h2>
      </div>

      <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginBottom: '1.5rem' }}>
        Register a human-readable <strong>.dmail</strong> name for your address.
        Names are permanent and secured by proof-of-work.
      </p>

      {error && <div className="error-banner">{error}</div>}
      {success && (
        <div style={{
          background: '#0b3b1b',
          border: '1px solid var(--success)',
          borderRadius: 'var(--radius)',
          padding: '0.75rem 1rem',
          color: 'var(--success)',
          fontSize: '0.85rem',
          marginBottom: '1rem',
        }}>
          {success}
        </div>
      )}

      <div className="field">
        <label>Choose a name</label>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <input
            type="text"
            placeholder="alice"
            value={name}
            onChange={e => {
              setName(e.target.value.toLowerCase())
              setAvailable(null)
              setError('')
            }}
            style={{ flex: 1 }}
          />
          <span style={{ color: 'var(--text-muted)', fontSize: '0.9rem', whiteSpace: 'nowrap' }}>
            .dmail
          </span>
        </div>
        {name && !isValid && (
          <p style={{ fontSize: '0.75rem', color: 'var(--danger)', marginTop: '0.25rem' }}>
            3-32 chars, lowercase letters, numbers, and hyphens only. Must start/end with letter or number.
          </p>
        )}
      </div>

      <div className="actions" style={{ marginTop: '1rem' }}>
        <button
          className="secondary"
          disabled={!isValid || checking}
          onClick={handleCheck}
        >
          {checking ? 'Checking...' : 'Check Availability'}
        </button>
        <button
          className="primary"
          disabled={!isValid || available !== true || registering}
          onClick={handleRegister}
        >
          {registering ? 'Submitting...' : 'Register'}
        </button>
      </div>

      {available === true && (
        <p style={{ fontSize: '0.85rem', color: 'var(--success)', marginTop: '0.75rem' }}>
          {name}.dmail is available!
        </p>
      )}
      {available === false && (
        <p style={{ fontSize: '0.85rem', color: 'var(--danger)', marginTop: '0.75rem' }}>
          {name}.dmail is already taken.
        </p>
      )}

      {registering && (
        <div className="stamping" style={{ marginTop: '1rem' }}>
          <div className="spinner" />
          Computing proof-of-work for name registration... This takes ~10 minutes.
        </div>
      )}

      <div style={{ marginTop: '2.5rem' }}>
        <h3 style={{ fontSize: '1rem', marginBottom: '1rem' }}>Your Names</h3>
        {myNames.length === 0 ? (
          <p className="empty-state">No registered names yet.</p>
        ) : (
          <ul className="contacts-list">
            {myNames.map(n => (
              <li key={n.name} className="contact-item">
                <div>
                  <div className="name">{n.name}.dmail</div>
                  <div className="key">{n.pubkey}</div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  )
}
