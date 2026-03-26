import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, Contact } from '../api'
import Avatar from '../components/Avatar'

interface Props {
  contacts: Contact[]
  onUpdate: () => Promise<void>
}

export default function Contacts({ contacts, onUpdate }: Props) {
  const [pubkey, setPubkey] = useState('')
  const [petname, setPetname] = useState('')
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)
  const [showForm, setShowForm] = useState(false)
  const navigate = useNavigate()

  const handleAdd = async () => {
    setError('')
    setSaving(true)
    try {
      await api.saveContact(pubkey, petname)
      setPubkey('')
      setPetname('')
      setShowForm(false)
      await onUpdate()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (key: string) => {
    try {
      await api.deleteContact(key)
      await onUpdate()
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <div>
      <div className="page-header">
        <h2>Contacts</h2>
        <button className="primary" onClick={() => setShowForm(!showForm)}>
          {showForm ? 'Cancel' : '+ Add Contact'}
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {showForm && (
        <div className="card" style={{ marginBottom: '1.5rem' }}>
          <h3 className="card-title">New Contact</h3>
          <div className="field" style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
              Dmail Address
            </label>
            <input
              type="text"
              placeholder="dmail:abc123..."
              value={pubkey}
              onChange={e => setPubkey(e.target.value)}
            />
          </div>
          <div className="field" style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
              Display Name
            </label>
            <input
              type="text"
              placeholder="Alice"
              value={petname}
              onChange={e => setPetname(e.target.value)}
            />
          </div>
          <button
            className="primary"
            onClick={handleAdd}
            disabled={!pubkey || !petname || saving}
          >
            {saving ? 'Saving...' : 'Save Contact'}
          </button>
        </div>
      )}

      {contacts.length === 0 ? (
        <div className="empty-state-card">
          <div className="empty-icon">&#128101;</div>
          <h3>No contacts yet</h3>
          <p>Add contacts to easily send messages using nicknames instead of long addresses.</p>
          {!showForm && (
            <button className="primary" style={{ marginTop: '1rem' }} onClick={() => setShowForm(true)}>
              Add Your First Contact
            </button>
          )}
        </div>
      ) : (
        <ul className="contacts-list">
          {contacts.map(c => (
            <li key={c.pubkey} className="contact-item">
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flex: 1, minWidth: 0 }}>
                <Avatar address={c.pubkey} size={40} />
                <div style={{ minWidth: 0 }}>
                  <div className="name">{c.petname}</div>
                  <div className="key">{c.pubkey}</div>
                </div>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem', flexShrink: 0 }}>
                <button
                  className="secondary"
                  onClick={() => navigate(`/compose?to=${encodeURIComponent(c.pubkey)}`)}
                >
                  Message
                </button>
                <button className="danger" onClick={() => handleDelete(c.pubkey)}>
                  Remove
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
