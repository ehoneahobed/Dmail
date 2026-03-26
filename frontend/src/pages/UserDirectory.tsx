import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, User } from '../api'
import Avatar from '../components/Avatar'

export default function UserDirectory() {
  const [users, setUsers] = useState<User[]>([])
  const [loading, setLoading] = useState(true)
  const navigate = useNavigate()

  useEffect(() => {
    api
      .listUsers()
      .then(setUsers)
      .catch(() => setUsers([]))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div>
      <div className="page-header">
        <h2>User Directory</h2>
      </div>

      {loading ? (
        <div className="empty-state-card">
          <div className="spinner" style={{ margin: '0 auto' }} />
        </div>
      ) : users.length === 0 ? (
        <div className="empty-state-card">
          <div className="empty-icon">&#128101;</div>
          <h3>No users yet</h3>
          <p>Users who sign up on this server will appear here.</p>
        </div>
      ) : (
        <div className="card-list">
          {users.map(u => (
            <div key={u.id} className="card-list-item">
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', flex: 1, minWidth: 0 }}>
                <Avatar address={u.pubkey} size={40} />
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>{u.username}</div>
                  <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{u.pubkey}</div>
                </div>
              </div>
              <button
                className="primary"
                onClick={() => navigate(`/compose?to=${encodeURIComponent(u.pubkey)}`)}
              >
                Message
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
