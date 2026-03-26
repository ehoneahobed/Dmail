import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { api, Message } from '../api'
import Avatar from '../components/Avatar'
import MarkdownRenderer from '../components/MarkdownRenderer'
import DeliveryStatus from '../components/DeliveryStatus'

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
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.5rem' }}>
          <Avatar address={message.sender} size={40} />
          <div>
            <p>From: <strong>{resolveName(message.sender)}</strong></p>
            <p>To: <strong>{resolveName(message.recipient)}</strong></p>
          </div>
        </div>
        <p>
          {date.toLocaleString()}
          {message.status && (
            <span style={{ marginLeft: '0.5rem' }}>
              <DeliveryStatus status={message.status} />
            </span>
          )}
        </p>
        <h2>{message.subject}</h2>
      </div>

      <div className="body">
        <MarkdownRenderer content={message.body} />
      </div>

      <div style={{ marginTop: '2rem', display: 'flex', gap: '0.5rem' }}>
        <button
          className="primary"
          onClick={() => navigate(`/compose?reply=${message.id}&to=${encodeURIComponent(message.sender)}`)}
        >
          Reply
        </button>
        {message.thread_id && (
          <button
            className="secondary"
            onClick={() => navigate(`/thread/${message.thread_id}`)}
          >
            View Thread
          </button>
        )}
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
