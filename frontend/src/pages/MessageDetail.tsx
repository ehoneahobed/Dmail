import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, Message } from '../api'

interface Props {
  resolveName: (pubkey: string) => string
}

export default function MessageDetail({ resolveName }: Props) {
  const { id } = useParams<{ id: string }>()
  const [message, setMessage] = useState<Message | null>(null)
  const [error, setError] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    if (!id) return
    api
      .getMessage(id)
      .then(msg => {
        setMessage(msg)
        if (!msg.is_read) {
          api.markRead(id).catch(() => {})
        }
      })
      .catch(e => setError(e.message))
  }, [id])

  if (error) {
    return (
      <div>
        <div className="error-banner">{error}</div>
        <button className="secondary" onClick={() => navigate(-1)}>Back</button>
      </div>
    )
  }

  if (!message) {
    return (
      <div className="empty-state">
        <div className="spinner" style={{ margin: '0 auto' }} />
      </div>
    )
  }

  const date = new Date(message.timestamp * 1000)

  return (
    <div className="message-detail">
      <button className="secondary" onClick={() => navigate(-1)} style={{ marginBottom: '1rem' }}>
        Back
      </button>

      <div className="meta">
        <p>From: <strong>{resolveName(message.sender)}</strong></p>
        <p>To: <strong>{resolveName(message.recipient)}</strong></p>
        <p>{date.toLocaleString()}</p>
        <h2>{message.subject}</h2>
      </div>

      <div className="body">{message.body}</div>

      <div style={{ marginTop: '2rem', display: 'flex', gap: '0.5rem' }}>
        <button
          className="primary"
          onClick={() => navigate(`/compose?reply=${message.id}&to=${encodeURIComponent(message.sender)}`)}
        >
          Reply
        </button>
        <button
          className="danger"
          onClick={async () => {
            await api.deleteMessage(message.id)
            navigate(-1)
          }}
        >
          Delete
        </button>
      </div>
    </div>
  )
}
