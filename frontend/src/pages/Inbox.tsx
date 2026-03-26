import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, Message } from '../api'

interface Props {
  folder: string
  resolveName: (pubkey: string) => string
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleDateString([], { month: 'short', day: 'numeric' })
}

export default function Inbox({ folder, resolveName }: Props) {
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    setLoading(true)
    setError('')
    api
      .listMessages(folder)
      .then(setMessages)
      .catch(e => setError(e.message))
      .finally(() => setLoading(false))
  }, [folder])

  const title = folder.charAt(0).toUpperCase() + folder.slice(1)

  return (
    <div>
      <div className="page-header">
        <h2>{title}</h2>
        <button className="primary" onClick={() => navigate('/compose')}>
          Compose
        </button>
      </div>

      {error && <div className="error-banner">{error}</div>}

      {loading ? (
        <div className="empty-state">
          <div className="spinner" style={{ margin: '0 auto' }} />
        </div>
      ) : messages.length === 0 ? (
        <div className="empty-state">
          <p>No messages in {folder}.</p>
        </div>
      ) : (
        <ul className="message-list">
          {messages.map(msg => (
            <li
              key={msg.id}
              className={`message-item ${!msg.is_read && folder === 'inbox' ? 'unread' : ''}`}
              onClick={() => navigate(`/message/${msg.id}`)}
            >
              <span className="sender">
                {folder === 'sent' ? resolveName(msg.recipient) : resolveName(msg.sender)}
              </span>
              <span className="subject">{msg.subject}</span>
              <span className="time">{formatTime(msg.timestamp)}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
