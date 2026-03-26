import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, User } from '../api'
import Avatar from '../components/Avatar'

export default function UserDirectory() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    api
      .listUsers()
      .then(setUsers)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="page-header">
        <h2>User Directory</h2>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="empty-state">
          <div className="spinner" style={{ margin: '0 auto' }} />
        </div>
      ) : users.length === 0 ? (
        <div className="empty-state">
          <p>No users registered yet.</p>
        </div>
      ) : (
        <ul className="contacts-list">
          {users.map(u => (
            <li key={u.id} className="contact-item">
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                <Avatar address={u.pubkey} size={36} />
                <div>
                  <div className="name">{u.username}</div>
                  <div className="key">{u.pubkey}</div>
                </div>
              </div>
              <button
                className="primary"
                onClick={() => navigate(`/compose?to=${encodeURIComponent(u.pubkey)}`)}
              >
                Message
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
