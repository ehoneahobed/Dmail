import { useState } from 'react'
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

  const handleAdd = async () => {
    setError('')
    setSaving(true)
    try {
      await api.saveContact(pubkey, petname)
      setPubkey('')
      setPetname('')
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
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="add-contact-form">
        <div className="field">
          <label>Dmail Address</label>
          <input
            type="text"
            placeholder="dmail:abc123..."
            value={pubkey}
            onChange={e => setPubkey(e.target.value)}
          />
        </div>
        <div className="field">
          <label>Name</label>
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
          style={{ marginBottom: '1px' }}
        >
          Add
        </button>
      </div>

      {contacts.length === 0 ? (
        <div className="empty-state">
          <p>No contacts yet. Add one above.</p>
        </div>
      ) : (
        <ul className="contacts-list">
          {contacts.map(c => (
            <li key={c.pubkey} className="contact-item">
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <Avatar address={c.pubkey} size={36} />
                <div>
                  <div className="name">{c.petname}</div>
                  <div className="key">{c.pubkey}</div>
                </div>
              </div>
              <button className="danger" onClick={() => handleDelete(c.pubkey)}>
                Remove
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
